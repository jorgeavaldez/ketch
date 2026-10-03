package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	config "github.com/1broseidon/ketch/internal/configbase"
)

func TestNewFromConfigBraveKeyCompatibility(t *testing.T) {
	t.Run("singular only", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.SetProvider("brave_api_key", "legacy")
		searcher, err := NewFromConfig(&cfg, "brave", "")
		if err != nil {
			t.Fatal(err)
		}
		backend, ok := searcher.(*Brave)
		if !ok || backend.keys.size() != 1 || backend.keys.keys[0] != "legacy" {
			t.Fatalf("backend = %#v", searcher)
		}
	})

	t.Run("plural only", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.SetProvider("brave_api_keys", []string{"one", "two"})
		searcher, err := NewFromConfig(&cfg, "brave", "")
		if err != nil {
			t.Fatal(err)
		}
		backend, ok := searcher.(*Brave)
		if !ok || backend.keys.size() != 2 {
			t.Fatalf("backend = %#v", searcher)
		}
	})

	t.Run("neither", func(t *testing.T) {
		cfg := config.Defaults()
		if _, err := NewFromConfig(&cfg, "brave", ""); err == nil {
			t.Fatal("expected a missing-key precondition error")
		}
	})
}

func TestNewFromConfigBuildsEveryEffectiveKeyPool(t *testing.T) {
	cfg := config.Defaults()
	{
		providerValue0, providerValue1 := "brave-legacy", []string{"brave-new"}
		cfg.SetProvider("brave_api_key", providerValue0)
		cfg.SetProvider("brave_api_keys", providerValue1)
	}
	{
		providerValue0, providerValue1 := "exa-legacy", []string{"exa-new"}
		cfg.SetProvider("exa_api_key", providerValue0)
		cfg.SetProvider("exa_api_keys", providerValue1)
	}
	{
		providerValue0, providerValue1 := "firecrawl-legacy", []string{"firecrawl-new"}
		cfg.SetProvider("firecrawl_api_key", providerValue0)
		cfg.SetProvider("firecrawl_api_keys", providerValue1)
	}
	{
		providerValue0, providerValue1 := "keenable-legacy", []string{"keenable-new"}
		cfg.SetProvider("keenable_api_key", providerValue0)
		cfg.SetProvider("keenable_api_keys", providerValue1)
	}
	{
		providerValue0, providerValue1 := "tavily-legacy", []string{"tavily-new"}
		cfg.SetProvider("tavily_api_key", providerValue0)
		cfg.SetProvider("tavily_api_keys", providerValue1)
	}
	{
		providerValue0, providerValue1 := "serpbase-legacy", []string{"serpbase-new"}
		cfg.SetProvider("serpbase_api_key", providerValue0)
		cfg.SetProvider("serpbase_api_keys", providerValue1)
	}
	{
		providerValue0, providerValue1 := "youcom-legacy", []string{"youcom-new"}
		cfg.SetProvider("youcom_api_key", providerValue0)
		cfg.SetProvider("youcom_api_keys", providerValue1)
	}

	for _, backend := range []string{"brave", "exa", "firecrawl", "keenable", "tavily", "serpbase", "youcom"} {
		searcher, err := NewFromConfig(&cfg, backend, "")
		if err != nil {
			t.Fatalf("%s: %v", backend, err)
		}
		var size int
		switch candidate := searcher.(type) {
		case *Brave:
			size = candidate.keys.size()
		case *EXA:
			size = candidate.keys.size()
		case *Firecrawl:
			size = candidate.keys.size()
		case *Keenable:
			size = candidate.keys.size()
		case *Tavily:
			size = candidate.keys.size()
		case *SerpBase:
			size = candidate.keys.size()
		case *Youcom:
			size = candidate.keys.size()
		default:
			t.Fatalf("%s: unexpected searcher %T", backend, searcher)
		}
		if size != 2 {
			t.Errorf("%s pool size = %d, want 2", backend, size)
		}
	}
}

func TestExportedBackendConstructorsKeepSingleKeyCompatibility(t *testing.T) {
	exaKey := "exa"
	keenableKey := "keenable"
	tests := []struct {
		name string
		size int
	}{
		{name: "brave", size: NewBrave("brave").keys.size()},
		{name: "exa", size: NewEXA(&exaKey).keys.size()},
		{name: "firecrawl", size: NewFirecrawl("firecrawl").keys.size()},
		{name: "keenable", size: NewKeenable(&keenableKey).keys.size()},
		{name: "tavily", size: NewTavily("tavily").keys.size()},
		{name: "serpbase", size: NewSerpBase("serpbase").keys.size()},
		{name: "youcom", size: NewYoucom("youcom").keys.size()},
	}
	for _, tc := range tests {
		if tc.size != 1 {
			t.Errorf("%s constructor pool size = %d, want 1", tc.name, tc.size)
		}
	}
}

func TestNewFromConfigTavilyRequiresKey(t *testing.T) {
	cfg := config.Defaults()
	if _, err := NewFromConfig(&cfg, "tavily", ""); err == nil {
		t.Fatal("expected missing-key error for tavily")
	}
}

func TestNewFromConfigParallelIsKeyless(t *testing.T) {
	cfg := config.Defaults()
	searcher, err := NewFromConfig(&cfg, "parallel", "")
	if err != nil {
		t.Fatal(err)
	}
	backend, ok := searcher.(*Parallel)
	if !ok {
		t.Fatalf("unexpected type %T", searcher)
	}
	if backend.endpoint != parallelEndpoint {
		t.Fatalf("endpoint = %q, want %q", backend.endpoint, parallelEndpoint)
	}
}

func TestNewFromConfigSerpBaseRequiresKey(t *testing.T) {
	cfg := config.Defaults()
	if _, err := NewFromConfig(&cfg, "serpbase", ""); err == nil {
		t.Fatal("expected missing-key error for serpbase")
	}
}

func TestNewFromConfigYoucomIsKeylessByDefault(t *testing.T) {
	t.Run("empty key pool builds", func(t *testing.T) {
		cfg := config.Defaults()
		searcher, err := NewFromConfig(&cfg, "youcom", "")
		if err != nil {
			t.Fatal(err)
		}
		backend, ok := searcher.(*Youcom)
		if !ok {
			t.Fatalf("unexpected type %T", searcher)
		}
		if backend.keys.size() != 0 {
			t.Fatalf("keys = %d, want 0 (keyless free profile)", backend.keys.size())
		}
	})
	t.Run("configured keys reach the pool", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.SetProvider("youcom_api_keys", []string{"one", "two"})
		backend, err := NewFromConfig(&cfg, "youcom", "")
		if err != nil {
			t.Fatal(err)
		}
		if got := backend.(*Youcom).keys.size(); got != 2 {
			t.Fatalf("pool size = %d, want 2", got)
		}
	})
}

func TestNewFromConfigAppliesProviderHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Client"); got != "provider" {
			t.Errorf("X-Client = %q, want provider override", got)
		}
		if got := r.Header.Get("X-Global"); got != "global" {
			t.Errorf("X-Global = %q, want global default", got)
		}
		if _, present := r.Header["X-Remove"]; present {
			t.Errorf("inherited X-Remove was not deleted: %q", r.Header.Get("X-Remove"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/headers","title":"Headers"}]}`))
	}))
	defer server.Close()

	cfg := config.Defaults()
	cfg.SetProvider("searxng_url", server.URL)
	cfg.HTTPHeaders = http.Header{"X-Client": {"global"}, "X-Global": {"global"}, "X-Remove": {"remove"}}
	cfg.ProviderHTTPHeaders = map[string]http.Header{"searxng": {"x-client": {"provider"}, "X-Remove": {}}}
	searcher, err := NewFromConfig(&cfg, "searxng", "")
	if err != nil {
		t.Fatal(err)
	}
	results, err := searcher.Search(context.Background(), "headers", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].URL != "https://example.com/headers" {
		t.Fatalf("results = %#v, want the configured instance's response", results)
	}
}

func TestSearxngOverrideRejectsHeaderForwarding(t *testing.T) {
	for _, mode := range []string{"searxng", "auto", "auto constructor", "multi", "multi all", "random", "random all"} {
		for _, scope := range []string{"global", "provider"} {
			t.Run(mode+"/"+scope, func(t *testing.T) {
				cfg := config.Defaults()
				cfg.SetProvider("searxng_url", "https://operator.example")
				headers := http.Header{"Authorization": {"Bearer private-value"}}
				if scope == "global" {
					cfg.HTTPHeaders = headers
				} else {
					cfg.ProviderHTTPHeaders = map[string]http.Header{"searxng": headers}
				}
				const override = "https://untrusted.example"
				var err error
				switch mode {
				case "searxng", "auto":
					_, err = NewFromConfig(&cfg, mode, override)
				case "auto constructor":
					_, err = NewAutoFromConfig(&cfg, override)
				case "multi":
					_, err = NewMultiFromConfig(&cfg, []string{"searxng"}, override)
				case "multi all":
					_, err = NewMultiFromConfig(&cfg, []string{"all"}, override)
				case "random":
					_, err = NewRandomFromConfig(&cfg, []string{"searxng"}, override)
				case "random all":
					_, err = NewRandomFromConfig(&cfg, []string{"all"}, override)
				}
				if err == nil {
					t.Fatal("accepted a cross-origin override with configured headers")
				}
				if cfg.String("searxng_url") != "https://operator.example" {
					t.Fatal("override mutated shared configuration")
				}
			})
		}
	}
}

func TestSearxngHeaderOverrideOrigins(t *testing.T) {
	for _, tc := range []struct {
		name, configured, override string
		wantError                  bool
	}{
		{"unchanged", "https://operator.example", "", false},
		{"same origin", "https://operator.example", "https://operator.example/other", false},
		{"default HTTPS port", "https://operator.example:443", "https://OPERATOR.example/other", false},
		{"default HTTP port", "http://operator.example:80", "http://operator.example/other", false},
		{"different host", "https://operator.example", "https://untrusted.example", true},
		{"different port", "https://operator.example", "https://operator.example:8443", true},
		{"downgrade", "https://operator.example", "http://operator.example", true},
		{"relative override", "https://operator.example", "/other", true},
		{"malformed override", "https://operator.example", "https://operator.example:bad", true},
		{"no configured origin", "", "https://untrusted.example", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Defaults()
			cfg.SetProvider("searxng_url", tc.configured)
			cfg.HTTPHeaders = http.Header{"Authorization": {"Bearer private-value"}}
			_, err := NewFromConfig(&cfg, "searxng", tc.override)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %t", err, tc.wantError)
			}
		})
	}
}

func TestSearxngOverrideWithoutEffectiveHeaders(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		cfg := config.Defaults()
		cfg.SetProvider("searxng_url", "https://operator.example")
		if deleted {
			cfg.HTTPHeaders = http.Header{"Authorization": {"Bearer private-value"}}
			cfg.ProviderHTTPHeaders = map[string]http.Header{"searxng": {"Authorization": {}}}
		}
		if _, err := NewFromConfig(&cfg, "searxng", "https://other.example"); err != nil {
			t.Fatalf("override without effective headers: %v", err)
		}
	}
}

func TestSearxngSameOriginOverrideSendsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/other/search" || r.Header.Get("X-Client") != "configured" {
			t.Errorf("path/header = %q/%q", r.URL.Path, r.Header.Get("X-Client"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/headers","title":"Headers"}]}`))
	}))
	defer server.Close()
	for _, backend := range []string{"searxng", "auto"} {
		cfg := config.Defaults()
		cfg.SetProvider("searxng_url", server.URL)
		cfg.ProviderHTTPHeaders = map[string]http.Header{"searxng": {"X-Client": {"configured"}}}
		searcher, err := NewFromConfig(&cfg, backend, server.URL+"/other")
		if err != nil {
			t.Fatal(err)
		}
		results, err := searcher.Search(context.Background(), "q", 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || results[0].URL != "https://example.com/headers" {
			t.Fatalf("%s results = %#v, want the overridden instance's response", backend, results)
		}
		if cfg.String("searxng_url") != server.URL {
			t.Fatal("override mutated shared configuration")
		}
	}
}

func TestNewFromConfigFirecrawlURL(t *testing.T) {
	t.Run("hosted allows empty key", func(t *testing.T) {
		cfg := config.Defaults()
		searcher, err := NewFromConfig(&cfg, "firecrawl", "")
		if err != nil {
			t.Fatal(err)
		}
		backend, ok := searcher.(*Firecrawl)
		if !ok {
			t.Fatalf("unexpected type %T", searcher)
		}
		if backend.keys.size() != 0 {
			t.Fatalf("keys = %d, want 0", backend.keys.size())
		}
		if got := backend.endpoint; got != config.FirecrawlSearchURL(config.DefaultFirecrawlURL) {
			t.Fatalf("endpoint = %q, want hosted /v2/search", got)
		}
	})

	t.Run("self-hosted allows empty key", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.SetProvider("firecrawl_url", "http://localhost:3002")
		searcher, err := NewFromConfig(&cfg, "firecrawl", "")
		if err != nil {
			t.Fatal(err)
		}
		backend, ok := searcher.(*Firecrawl)
		if !ok {
			t.Fatalf("unexpected type %T", searcher)
		}
		if backend.keys.size() != 0 {
			t.Fatalf("keys = %d, want 0", backend.keys.size())
		}
		if got := backend.endpoint; got != "http://localhost:3002/v2/search" {
			t.Fatalf("endpoint = %q, want local /v2/search", got)
		}
	})

	t.Run("custom base with key", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.SetProvider("firecrawl_url", "https://fc.example.com/")
		cfg.SetProvider("firecrawl_api_key", "k")
		searcher, err := NewFromConfig(&cfg, "firecrawl", "")
		if err != nil {
			t.Fatal(err)
		}
		backend := searcher.(*Firecrawl)
		if got := backend.endpoint; got != "https://fc.example.com/v2/search" {
			t.Fatalf("endpoint = %q", got)
		}
	})
}
