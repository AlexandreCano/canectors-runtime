package auth

import (
	"strings"
	"testing"
	"time"
)

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
