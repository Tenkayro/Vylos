package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Role string

const (
	RoleParent  Role = "parent"
	RoleChild   Role = "child"
	RoleService Role = "service"
)

type Session struct {
	Token     string    `json:"token"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ReplayGuard prevents replay attacks on authenticated LAN requests using timestamp windows and nonce tracking.
type ReplayGuard struct {
	mu     sync.Mutex
	nonces map[string]time.Time
	window time.Duration
}

func NewReplayGuard(window time.Duration) *ReplayGuard {
	if window <= 0 {
		window = 2 * time.Minute
	}
	return &ReplayGuard{
		nonces: make(map[string]time.Time),
		window: window,
	}
}

// ValidateAndRecord checks that timestampUnix is within ±window of current time and that nonce has not been seen.
func (g *ReplayGuard) ValidateAndRecord(deviceID, nonce string, timestampUnix int64) error {
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(nonce) == "" {
		return errors.New("missing device ID or nonce")
	}
	now := time.Now()
	reqTime := time.Unix(timestampUnix, 0)
	diff := now.Sub(reqTime)
	if diff < -g.window || diff > g.window {
		return fmt.Errorf("request timestamp outside allowed replay window (%v)", diff.Round(time.Second))
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Prune expired nonces
	cutoff := now.Add(-g.window * 2)
	for k, ts := range g.nonces {
		if ts.Before(cutoff) {
			delete(g.nonces, k)
		}
	}

	key := deviceID + ":" + nonce
	if _, exists := g.nonces[key]; exists {
		return errors.New("replay detected: nonce already used")
	}
	g.nonces[key] = now
	return nil
}

// AuthManager manages parent UI sessions, RBAC checks, and rate-limited authentication attempts.
type AuthManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
	limiters map[string]*rate.Limiter
	ttl      time.Duration
}

func NewAuthManager(sessionTTL time.Duration) *AuthManager {
	if sessionTTL <= 0 {
		sessionTTL = 4 * time.Hour
	}
	return &AuthManager{
		sessions: make(map[string]Session),
		limiters: make(map[string]*rate.Limiter),
		ttl:      sessionTTL,
	}
}

// AllowAttempt applies token-bucket rate limiting per actor/IP key (e.g., max burst of 5, 1 attempt per 3 seconds).
func (am *AuthManager) AllowAttempt(actorKey string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	lim, exists := am.limiters[actorKey]
	if !exists {
		lim = rate.NewLimiter(rate.Every(3*time.Second), 5)
		am.limiters[actorKey] = lim
	}
	return lim.Allow()
}

// CreateSession issues a new random session token for the given role.
func (am *AuthManager) CreateSession(role Role) (Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, err
	}
	token := hex.EncodeToString(raw)
	now := time.Now()
	sess := Session{
		Token:     token,
		Role:      role,
		CreatedAt: now,
		ExpiresAt: now.Add(am.ttl),
	}

	am.mu.Lock()
	am.sessions[token] = sess
	am.mu.Unlock()
	return sess, nil
}

// ValidateSession verifies a session token and checks whether it has the required role.
func (am *AuthManager) ValidateSession(token string, requiredRole Role) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	am.mu.RLock()
	sess, ok := am.sessions[token]
	am.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().After(sess.ExpiresAt) {
		am.RevokeSession(token)
		return Session{}, false
	}
	if requiredRole != "" && sess.Role != requiredRole {
		return Session{}, false
	}
	return sess, true
}

// RevokeSession invalidates a session token.
func (am *AuthManager) RevokeSession(token string) {
	am.mu.Lock()
	delete(am.sessions, token)
	am.mu.Unlock()
}

// RevokeAll clears all active sessions.
func (am *AuthManager) RevokeAll() {
	am.mu.Lock()
	am.sessions = make(map[string]Session)
	am.mu.Unlock()
}

// GenerateRandomToken generates a cryptographically secure hex token of byteLen bytes.
func GenerateRandomToken(byteLen int) (string, error) {
	if byteLen <= 0 {
		byteLen = 32
	}
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
