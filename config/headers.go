package config

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/1broseidon/ketch/code"
	"github.com/1broseidon/ketch/docs"
	"github.com/1broseidon/ketch/search"
	"golang.org/x/net/http/httpguts"
)

func validateHeaders(headers http.Header) (http.Header, error) {
	normalized := make(http.Header, len(headers))
	for name, values := range headers {
		canonical := http.CanonicalHeaderKey(name)
		switch strings.ToLower(name) {
		case "host", "content-length", "transfer-encoding", "connection", "proxy-connection", "keep-alive", "te", "trailer", "upgrade", "expect", "proxy-authorization", "proxy-authenticate":
			return nil, fmt.Errorf("invalid or prohibited HTTP header name")
		}
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, fmt.Errorf("invalid or prohibited HTTP header name")
		}
		if _, exists := normalized[canonical]; exists {
			return nil, fmt.Errorf("case-colliding HTTP header names")
		}
		if values == nil {
			return nil, fmt.Errorf("HTTP header values must be arrays")
		}
		for _, value := range values {
			if !httpguts.ValidHeaderFieldValue(value) {
				return nil, fmt.Errorf("invalid HTTP header value")
			}
		}
		copyValues := make([]string, len(values))
		copy(copyValues, values)
		normalized[canonical] = copyValues
	}
	return normalized, nil
}

// ValidateHTTPHeaders validates and canonicalizes configured request headers.
func ValidateHTTPHeaders(c *Config) error {
	global, err := validateHeaders(c.HTTPHeaders)
	if err != nil {
		return err
	}
	providers, err := validateProviderHeaders(c.ProviderHTTPHeaders)
	if err != nil {
		return err
	}
	c.HTTPHeaders = global
	c.ProviderHTTPHeaders = providers
	return nil
}

func validateProviderHeaders(headersByProvider map[string]http.Header) (map[string]http.Header, error) {
	normalizedProviders := make(map[string]http.Header, len(headersByProvider))
	for provider, headers := range headersByProvider {
		if headers == nil {
			return nil, fmt.Errorf("provider HTTP headers must be objects")
		}
		_, searchOK := search.Lookup(provider)
		_, codeOK := code.Lookup(provider)
		_, docsOK := docs.Lookup(provider)
		if !searchOK && !codeOK && !docsOK {
			return nil, fmt.Errorf("unknown provider in provider_http_headers")
		}
		normalized, err := validateHeaders(headers)
		if err != nil {
			return nil, fmt.Errorf("invalid provider_http_headers entry")
		}
		normalizedProviders[provider] = normalized
	}
	return normalizedProviders, nil
}
