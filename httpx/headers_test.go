package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithHeadersCopiesRequestsAndHonorsExplicitEmptyValues(t *testing.T) {
	seen := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	headers := http.Header{"X-Default": {"from-config"}, "X-Explicit": {"from-default"}}
	client := WithHeaders(server.Client(), headers)
	headers["X-Default"][0] = "mutated-after-construction"

	request := mustRequest(t, server.URL)
	request.Header.Set("X-Explicit", "")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	got := <-seen
	values, ok := got["X-Explicit"]
	if got.Get("X-Default") != "from-config" || !ok || len(values) != 1 || values[0] != "" {
		t.Fatalf("request headers = %#v", got)
	}
	values = request.Header.Values("X-Explicit")
	if len(request.Header) != 1 || len(values) != 1 || values[0] != "" {
		t.Fatalf("original request was mutated: %#v", request.Header)
	}
}

func TestWithHeadersPreservesRedirectPolicy(t *testing.T) {
	server := httptest.NewServer(http.RedirectHandler("/target", http.StatusFound))
	defer server.Close()
	calls := 0
	base := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			calls++
			return http.ErrUseLastResponse
		},
	}
	wrapped := WithHeaders(base, http.Header{"X-Default": {"value"}})
	response, err := wrapped.Do(mustRequest(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || calls != 1 {
		t.Fatalf("redirect policy response/calls = %d/%d", response.StatusCode, calls)
	}
}

func TestWithHeadersRetainsDefaultRedirectLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	defer server.Close()
	client := WithHeaders(server.Client(), http.Header{"X-Default": {"value"}})
	response, err := client.Do(mustRequest(t, server.URL))
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || requests != 10 {
		t.Fatalf("redirect error/requests = %v/%d", err, requests)
	}
}

func TestWithHeadersStopsCrossOriginRedirects(t *testing.T) {
	var destinationRequests int
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationRequests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.RedirectHandler(destination.URL, http.StatusFound))
	defer origin.Close()

	client := WithHeaders(origin.Client(), http.Header{"X-Secret": {"scoped"}})
	response, err := client.Do(mustRequest(t, origin.URL))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want original redirect response", response.StatusCode)
	}
	if destinationRequests != 0 {
		t.Fatalf("cross-origin destination received %d requests", destinationRequests)
	}
}

func TestWithHeadersStopsHTTPSDowngradeRedirects(t *testing.T) {
	var destinationRequests int
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationRequests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	origin := httptest.NewTLSServer(http.RedirectHandler(destination.URL, http.StatusFound))
	defer origin.Close()

	client := WithHeaders(origin.Client(), http.Header{"X-Secret": {"scoped"}})
	response, err := client.Do(mustRequest(t, origin.URL))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || destinationRequests != 0 {
		t.Fatalf("downgrade response/destination requests = %d/%d", response.StatusCode, destinationRequests)
	}
}

func TestWithHeadersAllowsSameOriginRedirects(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/finish", http.StatusFound)
			return
		}
		received = r.Header.Get("X-Default")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := WithHeaders(server.Client(), http.Header{"X-Default": {"present"}})
	response, err := client.Do(mustRequest(t, server.URL+"/start"))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent || received != "present" {
		t.Fatalf("status/header = %d/%q", response.StatusCode, received)
	}
}

func mustRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
