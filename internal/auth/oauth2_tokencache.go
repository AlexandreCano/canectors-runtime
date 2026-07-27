package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	// singleflight collapses concurrent misses on the same credentials into one
	// token request, which a plain mutex would only do by serializing callers.
	"golang.org/x/sync/singleflight"
)

// Scheduled pipelines rebuild their modules on every tick, so each run used to
// start with an empty per-handler token cache and fetch a brand new token. A
// token valid for an hour was therefore requested once per execution: a pipeline
// polling every 15 seconds issued around 5 760 token requests a day for nothing,
// against endpoints that providers rate-limit aggressively.
//
// Tokens are a property of the credentials, not of a handler instance, so the
// cache lives for the life of the process and is shared by every handler using
// the same client. It stays in memory only, and the key is hashed so no secret
// is retained in a map key.
var sharedTokenCache = &tokenCache{entries: map[string]tokenCacheEntry{}}

// tokenFetchGroup dedupes in-flight token requests per credentials key.
var tokenFetchGroup singleflight.Group

type tokenCacheEntry struct {
	token  string
	expiry time.Time
}

type tokenCache struct {
	mu      sync.RWMutex
	entries map[string]tokenCacheEntry
}

// tokenCacheKey identifies a set of client credentials without keeping them
// readable in memory: a rotated secret yields a different key, so a stale token
// is never handed to a client that no longer owns it.
func tokenCacheKey(tokenURL, clientID, clientSecret string, scopes []string) string {
	sum := sha256.New()
	for _, part := range append([]string{tokenURL, clientID, clientSecret}, scopes...) {
		sum.Write([]byte(part))
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// get returns a cached token when one is still valid.
func (c *tokenCache) get(key string) (string, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || entry.token == "" || !time.Now().Before(entry.expiry) {
		return "", time.Time{}, false
	}
	return entry.token, entry.expiry, true
}

func (c *tokenCache) put(key, token string, expiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = tokenCacheEntry{token: token, expiry: expiry}
}

// invalidate drops a cached token, used when a provider rejects it before the
// expiry we computed.
func (c *tokenCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}
