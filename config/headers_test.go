package config

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/1broseidon/ketch/internal/testutil"
)

func TestHTTPHeaderEnvironmentReplacement(t *testing.T) {
	clearKetchEnv(t)
	t.Setenv("KETCH_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := Defaults()
	cfg.HTTPHeaders = http.Header{"X-Client": {"file"}, "X-File": {"global-only"}}
	cfg.ProviderHTTPHeaders = map[string]http.Header{
		"ddg":   {"X-Client": {"provider"}, "X-Provider": {"file-only"}},
		"brave": {"X-Other": {"other-provider"}},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KETCH_HTTP_HEADERS", `{"X-Environment":["override"]}`)

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := http.Header{"X-Environment": {"override"}, "X-Client": {"provider"}, "X-Provider": {"file-only"}}
	if headers := loaded.Config.EffectiveHTTPHeaders("ddg"); !reflect.DeepEqual(headers, want) {
		t.Fatalf("effective headers = %#v, want %#v", headers, want)
	}
	if len(loaded.Overrides) != 1 || loaded.Overrides[0].Key != "http_headers" || loaded.Overrides[0].Previous != "" {
		t.Fatalf("missing or unredacted header provenance: %+v", loaded.Overrides)
	}

	t.Setenv("KETCH_PROVIDER_HTTP_HEADERS", `{"ddg":{"X-Client":["environment-provider"]}}`)
	loaded, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	want = http.Header{"X-Client": {"environment-provider"}, "X-Environment": {"override"}}
	if headers := loaded.Config.EffectiveHTTPHeaders("ddg"); !reflect.DeepEqual(headers, want) || len(loaded.Config.ProviderHTTPHeaders) != 1 {
		t.Fatalf("provider environment setting did not replace file headers: %#v", loaded.Config.ProviderHTTPHeaders)
	}
	for _, override := range loaded.Overrides {
		if override.Previous != "" {
			t.Fatalf("header provenance exposed values: %+v", override)
		}
	}
}

func TestHeaderConfigSaveLoadPreservesEmptyDeletionLists(t *testing.T) {
	clearKetchEnv(t)
	testutil.SetIsolatedConfigHome(t)
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("KETCH_CONFIG", path)
	cfg := Defaults()
	cfg.HTTPHeaders = http.Header{"X-Client": {"global"}}
	cfg.ProviderHTTPHeaders = map[string]http.Header{"ddg": {"X-Client": {}, "X-Provider": {"provider"}}}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	headers := loaded.Config.EffectiveHTTPHeaders("ddg")
	if _, exists := headers["X-Client"]; exists || headers.Get("X-Provider") != "provider" {
		t.Fatalf("saved provider header overrides = %#v", headers)
	}
}

func TestHeaderValidationRejectsValuesWithoutExposingThem(t *testing.T) {
	for name, input := range map[string]string{
		"invalid name":     `{"http_headers":{"Bad Header":["private-value"]}}`,
		"control value":    `{"http_headers":{"X-Client":["private-value\nInjected: yes"]}}`,
		"case collision":   `{"http_headers":{"x-client":["one"],"X-Client":["private-value"]}}`,
		"unknown provider": `{"provider_http_headers":{"not-a-provider":{"X-Client":["private-value"]}}}`,
		"forbidden header": `{"http_headers":{"Host":["private-value"]}}`,
		"provider header":  `{"provider_http_headers":{"ddg":{"Host":["private-value"]}}}`,
		"null values":      `{"http_headers":{"X-Client":null}}`,
		"null provider":    `{"provider_http_headers":{"ddg":null}}`,
	} {
		t.Run(name, func(t *testing.T) {
			clearKetchEnv(t)
			testutil.SetIsolatedConfigHome(t)
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KETCH_CONFIG", path)
			_, err := Load()
			if err == nil {
				t.Fatal("expected invalid header configuration error")
			}
			if strings.Contains(err.Error(), "private-value") {
				t.Fatalf("error exposed header value: %v", err)
			}
		})
	}
}

func TestInvalidHeaderEnvironmentPreservesFileSettings(t *testing.T) {
	for _, tc := range []struct {
		name, global, provider string
	}{
		{name: "malformed global JSON", global: "not-json-private-value"},
		{name: "malformed provider JSON", provider: "not-json-private-value"},
		{name: "global prohibited header", global: `{"Host":["private-value"]}`},
		{name: "global invalid value", global: `{"X-Secret":["private-value\n"]}`},
		{name: "provider prohibited header", provider: `{"ddg":{"Host":["private-value"]}}`},
		{name: "unknown provider", provider: `{"unknown":{"X-Secret":["private-value"]}}`},
		{name: "both invalid", global: `{"Host":["private-value"]}`, provider: `{"unknown":{"X-Secret":["private-value"]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearKetchEnv(t)
			t.Setenv("KETCH_CONFIG", filepath.Join(t.TempDir(), "config.json"))
			cfg := Defaults()
			cfg.HTTPHeaders = http.Header{"X-Global": {"file"}}
			cfg.ProviderHTTPHeaders = map[string]http.Header{"ddg": {"X-Provider": {"file"}}}
			if err := Save(cfg); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KETCH_HTTP_HEADERS", tc.global)
			t.Setenv("KETCH_PROVIDER_HTTP_HEADERS", tc.provider)
			t.Setenv("KETCH_LIMIT", "7")

			loaded, err := Load()
			if err == nil {
				t.Fatal("expected invalid header environment error")
			}
			for name, value := range map[string]string{"KETCH_HTTP_HEADERS": tc.global, "KETCH_PROVIDER_HTTP_HEADERS": tc.provider} {
				if value != "" && !strings.Contains(err.Error(), name) {
					t.Errorf("error %q does not name %s", err, name)
				}
			}
			if strings.Contains(err.Error(), "private-value") {
				t.Fatal("error exposed a header value")
			}
			if !reflect.DeepEqual(loaded.Config.HTTPHeaders, cfg.HTTPHeaders) || !reflect.DeepEqual(loaded.Config.ProviderHTTPHeaders, cfg.ProviderHTTPHeaders) {
				t.Fatal("invalid override replaced valid file headers")
			}
			if loaded.Config.Limit != 7 || len(loaded.Overrides) != 1 || loaded.Overrides[0].Key != "limit" {
				t.Fatalf("expected only the valid limit override, got limit=%d overrides=%+v", loaded.Config.Limit, loaded.Overrides)
			}
		})
	}
}

func TestEffectiveHTTPHeadersReturnsIndependentValues(t *testing.T) {
	cfg := Defaults()
	cfg.HTTPHeaders = http.Header{"X-Client": {"global"}, "X-Global": {"global-only"}}
	cfg.ProviderHTTPHeaders = map[string]http.Header{"ddg": {"X-Client": {"provider"}}}
	got := cfg.EffectiveHTTPHeaders("ddg")
	got["X-Client"][0] = "mutated-provider"
	got["X-Global"][0] = "mutated-global"
	if cfg.HTTPHeaders.Get("X-Client") != "global" || cfg.HTTPHeaders.Get("X-Global") != "global-only" || cfg.ProviderHTTPHeaders["ddg"].Get("X-Client") != "provider" {
		t.Fatal("effective headers shared mutable state with configuration")
	}
}
