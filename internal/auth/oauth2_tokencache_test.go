package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/cannectors/runtime/pkg/connector"
)

// resetSharedTokenCache empties the process-wide cache so one test cannot serve
// a token to the next, then restores that state on cleanup.
//
// Handler tests reach the global cache through whatever credentials they
// configure. Their httptest servers happen to get distinct ports and therefore
// distinct keys, so they do not collide today — but that isolation is accidental,
// and a test that pins a token URL would silently inherit another test's token.
func resetSharedTokenCache(t *testing.T) {
	t.Helper()
	clear := func() {
		sharedTokenCache.mu.Lock()
		sharedTokenCache.entries = map[string]tokenCacheEntry{}
		sharedTokenCache.mu.Unlock()
		tokenFetchGroup = singleflight.Group{}
	}
	clear()
	t.Cleanup(clear)
}

func TestTokenCache_SharesTokensBetweenHandlers(t *testing.T) {
	cache := &tokenCache{entries: map[string]tokenCacheEntry{}}
	key := tokenCacheKey("https://idp/token", "client", "secret", []string{"read"})

	if _, _, ok := cache.get(key); ok {
		t.Fatal("empty cache returned a token")
	}

	cache.put(key, "token-1", time.Now().Add(time.Hour))
	got, _, ok := cache.get(key)
	if !ok || got != "token-1" {
		t.Fatalf("get() = %q, %v; want token-1, true", got, ok)
	}
}

func TestTokenCache_IgnoresExpiredEntries(t *testing.T) {
	cache := &tokenCache{entries: map[string]tokenCacheEntry{}}
	key := tokenCacheKey("https://idp/token", "client", "secret", nil)

	cache.put(key, "stale", time.Now().Add(-time.Second))
	if _, _, ok := cache.get(key); ok {
		t.Error("an expired token was served from the cache")
	}
}

func TestTokenCache_InvalidateDropsTheEntry(t *testing.T) {
	cache := &tokenCache{entries: map[string]tokenCacheEntry{}}
	key := tokenCacheKey("https://idp/token", "client", "secret", nil)

	cache.put(key, "token", time.Now().Add(time.Hour))
	cache.invalidate(key)
	if _, _, ok := cache.get(key); ok {
		t.Error("invalidate() left the token in place")
	}
}

// A rotated secret must not keep using the token issued to the previous one,
// which is why the key covers the credentials rather than the endpoint alone.
func TestTokenCacheKey_DistinguishesCredentials(t *testing.T) {
	base := tokenCacheKey("https://idp/token", "client", "secret", []string{"read"})
	cases := map[string]string{
		"different url":    tokenCacheKey("https://other/token", "client", "secret", []string{"read"}),
		"different client": tokenCacheKey("https://idp/token", "other", "secret", []string{"read"}),
		"rotated secret":   tokenCacheKey("https://idp/token", "client", "rotated", []string{"read"}),
		"different scope":  tokenCacheKey("https://idp/token", "client", "secret", []string{"write"}),
	}
	for name, key := range cases {
		if key == base {
			t.Errorf("%s produced the same cache key as the base credentials", name)
		}
	}
	if tokenCacheKey("https://idp/token", "client", "secret", []string{"read"}) != base {
		t.Error("the same credentials produced two different keys")
	}
}

// The key must not carry the secret in readable form.
func TestTokenCacheKey_DoesNotEmbedTheSecret(t *testing.T) {
	key := tokenCacheKey("https://idp/token", "client", "super-secret-value", nil)
	if strings.Contains(key, "super-secret-value") {
		t.Error("the cache key contains the client secret verbatim")
	}
}

// newTokenServerHandler builds an oauth2 handler backed by a token endpoint that
// counts the requests it serves.
func newTokenServerHandler(t *testing.T, requests *int32) *oauth2Handler {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(requests, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "issued-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	t.Cleanup(server.Close)

	handler, err := NewHandler(&connector.AuthConfig{
		Type: "oauth2",
		Credentials: toJSON(t, map[string]string{
			"tokenUrl":     server.URL,
			"clientId":     "client",
			"clientSecret": "secret",
		}),
	}, nil)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	oauth, ok := handler.(*oauth2Handler)
	if !ok {
		t.Fatalf("expected *oauth2Handler, got %T", handler)
	}
	return oauth
}

// TestOAuth2Handler_InvalidateIsVisibleToEveryHandler locks the fix for a
// divergence the handler-local cache allowed: two handlers built on the same
// credentials each kept their own copy of the token, so invalidating through one
// left the other presenting a token the provider had already rejected, until its
// own expiry ran out. Token state now lives in exactly one place.
func TestOAuth2Handler_InvalidateIsVisibleToEveryHandler(t *testing.T) {
	resetSharedTokenCache(t)

	var requests int32
	first := newTokenServerHandler(t, &requests)

	if _, err := first.getValidToken(context.Background()); err != nil {
		t.Fatalf("getValidToken() error = %v", err)
	}

	// A second handler on the same credentials — what a scheduled tick rebuilds.
	second := &oauth2Handler{cacheKey: first.cacheKey}
	if _, _, ok := sharedTokenCache.get(second.cacheKey); !ok {
		t.Fatal("a rebuilt handler did not see the token already fetched")
	}

	first.InvalidateToken()

	if _, _, ok := sharedTokenCache.get(second.cacheKey); ok {
		t.Error("the second handler still holds a token invalidated through the first")
	}
	if expiry := second.TokenExpiry(); !expiry.IsZero() {
		t.Errorf("TokenExpiry() = %v after invalidation, want zero", expiry)
	}
}

// TestOAuth2Handler_ConcurrentMissesFetchOnce locks the single-flight behavior.
// Webhook deliveries run in parallel on shared modules, so a cold cache used to
// mean one token request per in-flight delivery against an endpoint that
// providers rate-limit.
func TestOAuth2Handler_ConcurrentMissesFetchOnce(t *testing.T) {
	resetSharedTokenCache(t)

	var requests int32
	handler := newTokenServerHandler(t, &requests)

	const callers = 20
	var wg sync.WaitGroup
	tokens := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := handler.getValidToken(context.Background())
			if err != nil {
				t.Errorf("getValidToken() error = %v", err)
				return
			}
			tokens[i] = token
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("%d concurrent callers issued %d token requests, want 1", callers, got)
	}
	for i, token := range tokens {
		if token != "issued-token" {
			t.Errorf("caller %d got %q, want issued-token", i, token)
		}
	}
}
