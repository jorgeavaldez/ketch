package httpx

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// Doer is the HTTP operation provider clients require.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type headerClient struct {
	client  *http.Client
	headers http.Header
}

// WithHeaders returns a client that applies immutable defaults to requests.
// The wrapped client's transport is shared, while its other settings are copied.
// Requests must use canonical header keys, as set by http.Header.Set and Add.
func WithHeaders(client *http.Client, headers http.Header) Doer {
	configured := false
	for _, values := range headers {
		configured = configured || len(values) > 0
	}
	if !configured {
		return client
	}
	copyClient := *client
	checkRedirect := client.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if checkRedirect != nil {
			if err := checkRedirect(req, via); err != nil {
				return err
			}
		} else if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if len(via) > 0 && !SameOrigin(via[0].URL, req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &headerClient{client: &copyClient, headers: headers.Clone()}
}

func (c *headerClient) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return c.client.Do(nil)
	}
	clone := req.Clone(req.Context())
	if clone.Header == nil {
		clone.Header = make(http.Header)
	}
	for name, values := range c.headers {
		canonical := http.CanonicalHeaderKey(name)
		if _, present := clone.Header[canonical]; !present {
			clone.Header[canonical] = append([]string(nil), values...)
		}
	}
	return c.client.Do(clone)
}

// SameOrigin compares HTTP(S) URLs by scheme, hostname, and effective port.
// Relative URLs and non-HTTP schemes never share an origin.
func SameOrigin(a, b *url.URL) bool {
	if a.Hostname() == "" || b.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(a.Scheme, "http") && !strings.EqualFold(a.Scheme, "https") {
		return false
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if strings.EqualFold(u.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}
