package search

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/1broseidon/ketch/httpx"
	config "github.com/1broseidon/ketch/internal/configbase"
)

// ErrUnknownBackend identifies an unknown provider; missing prerequisites do not wrap it.
var ErrUnknownBackend = errors.New("unknown search backend")

var errUnsafeSearxngOverride = errors.New("searxng_url override must use the configured instance origin when custom HTTP headers are active")

// NewFromConfig constructs a registered search provider, or the auto fallback
// chain. The existing SearXNG per-call override is applied to a copy,
// preserving the shared configuration.
func NewFromConfig(cfg *config.Config, backend, searxngURL string) (Searcher, error) {
	if backend == AutoBackend {
		return NewAutoFromConfig(cfg, searxngURL)
	}
	p, ok := Lookup(backend)
	if !ok {
		return nil, fmt.Errorf("%w %q (available: %s)", ErrUnknownBackend, backend, strings.Join(SelectableBackends(), ", "))
	}
	c := *cfg
	if p.ID == "searxng" {
		if err := applySearxngOverride(&c, searxngURL); err != nil {
			return nil, err
		}
	}
	return p.Build(&c)
}

// applySearxngOverride keeps operator-configured credentials on the approved
// origin. CLI and MCP callers may choose an instance, but not forward its secrets.
func applySearxngOverride(c *config.Config, override string) error {
	if override == "" {
		return nil
	}
	for _, values := range c.EffectiveHTTPHeaders("searxng") {
		if len(values) == 0 {
			continue
		}
		configured, configuredErr := url.Parse(c.String("searxng_url"))
		target, targetErr := url.Parse(override)
		if configuredErr != nil || targetErr != nil || !httpx.SameOrigin(configured, target) {
			return errUnsafeSearxngOverride
		}
		break
	}
	c.SetProvider("searxng_url", override)
	return nil
}

// SelectableBackends returns every value accepted by --backend and the backend
// config key: the auto chain first, then the providers in registry order.
func SelectableBackends() []string {
	return append([]string{AutoBackend}, AvailableBackends()...)
}

// IsBackend reports whether id names a selectable backend.
func IsBackend(id string) bool {
	if id == AutoBackend {
		return true
	}
	_, ok := Lookup(id)
	return ok
}
