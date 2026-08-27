package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
)

const sessionCookie = "mypanel_session"

type sessionStore struct {
	redis *redis.Client
	ttl   time.Duration
}

type sessionContextKey struct{}

func openSessionStore(ctx context.Context, cfg config) (*sessionStore, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	if cfg.RedisPassword != "" {
		opts.Password = cfg.RedisPassword
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &sessionStore{redis: client, ttl: cfg.SessionTTL}, nil
}

func (s *sessionStore) create(ctx context.Context, user userRecord) (string, sessionRecord, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", sessionRecord{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return "", sessionRecord{}, err
	}
	record := sessionRecord{UserID: user.ID, Username: user.Username, Role: user.Role, SessionVersion: user.SessionVersion, MustChangePassword: user.MustChangePassword, CSRFToken: csrf}
	data, err := json.Marshal(record)
	if err != nil {
		return "", sessionRecord{}, err
	}
	if err := s.redis.Set(ctx, sessionKey(token), data, s.ttl).Err(); err != nil {
		return "", sessionRecord{}, err
	}
	return token, record, nil
}

func (s *sessionStore) get(ctx context.Context, token string) (sessionRecord, error) {
	if len(token) < 32 || len(token) > 128 {
		return sessionRecord{}, redis.Nil
	}
	data, err := s.redis.Get(ctx, sessionKey(token)).Bytes()
	if err != nil {
		return sessionRecord{}, err
	}
	var record sessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return sessionRecord{}, err
	}
	return record, nil
}

func (s *sessionStore) delete(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.redis.Del(ctx, sessionKey(token)).Err()
}

func (s *sessionStore) allowLogin(ctx context.Context, ip, username string) (bool, time.Duration, error) {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username))))
	keys := []string{"login:user:" + ip + ":" + hex.EncodeToString(digest[:8]), "login:ip:" + ip}
	result, err := s.redis.Eval(ctx, `
local first = redis.call('INCR', KEYS[1])
if first == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
local second = redis.call('INCR', KEYS[2])
if second == 1 then redis.call('EXPIRE', KEYS[2], ARGV[1]) end
return {first, redis.call('TTL', KEYS[1]), second, redis.call('TTL', KEYS[2])}
`, keys, 900).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 4 {
		return false, 0, errors.New("invalid login rate-limit response")
	}
	ttlSeconds := max(result[1], result[3])
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}
	return result[0] <= 5 && result[2] <= 20, time.Duration(ttlSeconds) * time.Second, nil
}

func (s *sessionStore) clearLoginLimit(ctx context.Context, ip, username string) {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username))))
	_ = s.redis.Del(ctx, "login:user:"+ip+":"+hex.EncodeToString(digest[:8])).Err()
}

func (s *sessionStore) allowRegistration(ctx context.Context, ip string) (bool, time.Duration, error) {
	key := "register:ip:" + ip
	result, err := s.redis.Eval(ctx, `
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
return {count, redis.call('TTL', KEYS[1])}
`, []string{key}, 3600).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 2 {
		return false, 0, errors.New("invalid registration rate-limit response")
	}
	return result[0] <= 5, time.Duration(max(int64(0), result[1])) * time.Second, nil
}

func sessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "session:" + hex.EncodeToString(digest[:])
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func hashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	const memory = 19 * 1024
	const iterations = 2
	const parallelism = 1
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations,
		parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	if memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 10 || parallelism < 1 || parallelism > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" || len(input.Username) > 64 || input.Password == "" || len(input.Password) > 1024 {
		write(w, http.StatusUnauthorized, apiError{Error: "invalid credentials", Code: "invalid_credentials", RequestID: requestID(r.Context())})
		return
	}
	ip := clientIP(r)
	allowed, retryAfter, err := a.sessions.allowLogin(r.Context(), ip, input.Username)
	if err != nil {
		internal(w, r, err)
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		write(w, http.StatusTooManyRequests, apiError{Error: "terlalu banyak percobaan login", Code: "rate_limited", RequestID: requestID(r.Context())})
		return
	}
	user, err := a.store.findUser(r.Context(), input.Username)
	if err != nil || user.Status != "active" || !verifyPassword(user.PasswordHash, input.Password) {
		_ = a.store.audit(r.Context(), nil, "auth.login_failed", "user", "", ip, map[string]any{"username": input.Username})
		write(w, http.StatusUnauthorized, apiError{Error: "invalid credentials", Code: "invalid_credentials", RequestID: requestID(r.Context())})
		return
	}
	token, session, err := a.sessions.create(r.Context(), user)
	if err != nil {
		internal(w, r, err)
		return
	}
	a.sessions.clearLoginLimit(r.Context(), ip, input.Username)
	a.setSessionCookie(w, token)
	_ = a.store.audit(r.Context(), &user.ID, "auth.login", "user", user.ID, ip, map[string]any{})
	write(w, http.StatusOK, sessionResponse(session))
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil {
		_ = a.sessions.delete(r.Context(), cookie.Value)
	}
	if session, ok := currentSession(r.Context()); ok {
		_ = a.store.audit(r.Context(), &session.UserID, "auth.logout", "user", session.UserID, clientIP(r), map[string]any{})
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: a.cfg.CookieSecure})
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) me(w http.ResponseWriter, r *http.Request) {
	session, _ := currentSession(r.Context())
	write(w, http.StatusOK, sessionResponse(session))
}

func (a *app) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			write(w, http.StatusUnauthorized, apiError{Error: "authentication required", Code: "authentication_required", RequestID: requestID(r.Context())})
			return
		}
		session, err := a.sessions.get(r.Context(), cookie.Value)
		if err != nil {
			write(w, http.StatusUnauthorized, apiError{Error: "authentication required", Code: "authentication_required", RequestID: requestID(r.Context())})
			return
		}
		user, err := a.store.findUserByID(r.Context(), session.UserID)
		if err != nil || user.Status != "active" || user.SessionVersion != session.SessionVersion {
			_ = a.sessions.delete(r.Context(), cookie.Value)
			write(w, http.StatusUnauthorized, apiError{Error: "authentication required", Code: "authentication_required", RequestID: requestID(r.Context())})
			return
		}
		session.Username = user.Username
		session.Role = user.Role
		session.MustChangePassword = user.MustChangePassword
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(session.CSRFToken)) != 1 || !a.validOrigin(r) {
				write(w, http.StatusForbidden, apiError{Error: "CSRF validation failed", Code: "csrf_failed", RequestID: requestID(r.Context())})
				return
			}
		}
		if session.MustChangePassword && r.URL.Path != "/api/v1/auth/me" && r.URL.Path != "/api/v1/auth/logout" && r.URL.Path != "/api/v1/auth/change-password" {
			write(w, http.StatusForbidden, apiError{Error: "password change required", Code: "password_change_required", RequestID: requestID(r.Context())})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, session)))
	}
}

func (a *app) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int(a.cfg.SessionTTL.Seconds()), Secure: a.cfg.CookieSecure})
}

func currentSession(ctx context.Context) (sessionRecord, bool) {
	v, ok := ctx.Value(sessionContextKey{}).(sessionRecord)
	return v, ok
}

func (a *app) validOrigin(r *http.Request) bool {
	origin := strings.TrimSuffix(r.Header.Get("Origin"), "/")
	if origin == "" {
		return true
	}
	if a.cfg.TrustedOrigin != "" {
		return subtle.ConstantTimeCompare([]byte(origin), []byte(a.cfg.TrustedOrigin)) == 1
	}
	scheme := "http"
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		scheme = "https"
	}
	return origin == scheme+"://"+r.Host
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host := r.RemoteAddr
	if index := strings.LastIndex(host, ":"); index > -1 {
		host = strings.Trim(host[:index], "[]")
	}
	return host
}
