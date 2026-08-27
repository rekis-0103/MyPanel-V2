package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

func (s *store) createUser(ctx context.Context, username, passwordHash string) (publicUser, error) {
	var out publicUser
	err := s.db.QueryRow(ctx, `INSERT INTO users (id,username,password_hash,role,status)
VALUES ($1,$2,$3,'user','active') RETURNING id,username,role,status,must_change_password,created_at,updated_at`,
		uuid.NewString(), username, passwordHash).Scan(&out.ID, &out.Username, &out.Role, &out.Status,
		&out.MustChangePassword, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *store) listUsers(ctx context.Context, query string, limit int) ([]publicUser, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `SELECT id,username,role,status,must_change_password,created_at,updated_at
FROM users WHERE $1='' OR username ILIKE '%'||$1||'%' OR id::text=$1 ORDER BY created_at DESC LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]publicUser, 0)
	for rows.Next() {
		var item publicUser
		if err := rows.Scan(&item.ID, &item.Username, &item.Role, &item.Status, &item.MustChangePassword, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *store) setUserStatus(ctx context.Context, id, status string) error {
	ct, err := s.db.Exec(ctx, `UPDATE users SET status=$2,session_version=session_version+1,updated_at=now() WHERE id=$1 AND role='user'`, id, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *store) setPassword(ctx context.Context, id, hash string, mustChange bool) error {
	ct, err := s.db.Exec(ctx, `UPDATE users SET password_hash=$2,must_change_password=$3,session_version=session_version+1,updated_at=now() WHERE id=$1`, id, hash, mustChange)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *store) listAuditForSession(ctx context.Context, session sessionRecord, limit int) ([]map[string]any, error) {
	if session.Role == "owner" {
		return s.listAudit(ctx, limit)
	}
	rows, err := s.db.Query(ctx, `SELECT a.id,u.username,a.action,a.target_type,a.target_id,a.ip,a.detail,a.created_at
FROM audit_events a LEFT JOIN users u ON u.id=a.user_id WHERE a.user_id=$1 ORDER BY a.created_at DESC LIMIT $2`, session.UserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var username, targetType, targetID, ip *string
		var action string
		var detail json.RawMessage
		var created time.Time
		if err := rows.Scan(&id, &username, &action, &targetType, &targetID, &ip, &detail, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "username": username, "action": action, "targetType": targetType, "targetId": targetID, "ip": ip, "detail": detail, "createdAt": created})
	}
	return out, rows.Err()
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	if !a.cfg.RegistrationEnabled {
		write(w, http.StatusForbidden, apiError{Error: "registration is disabled", Code: "registration_disabled", RequestID: requestID(r.Context())})
		return
	}
	if !a.validOrigin(r) {
		write(w, http.StatusForbidden, apiError{Error: "origin validation failed", Code: "origin_failed", RequestID: requestID(r.Context())})
		return
	}
	allowed, retryAfter, err := a.sessions.allowRegistration(r.Context(), clientIP(r))
	if err != nil {
		internal(w, r, err)
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", fmt.Sprint(max(1, int(retryAfter.Seconds()))))
		write(w, http.StatusTooManyRequests, apiError{Error: "too many registration attempts", Code: "rate_limited", RequestID: requestID(r.Context())})
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
	if !usernamePattern.MatchString(input.Username) {
		write(w, http.StatusBadRequest, apiError{Error: "username must be 3-32 letters, numbers, dots, underscores, or hyphens", Code: "invalid_username", RequestID: requestID(r.Context())})
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "weak_password", RequestID: requestID(r.Context())})
		return
	}
	item, err := a.store.createUser(r.Context(), input.Username, hash)
	if err != nil {
		write(w, http.StatusConflict, apiError{Error: "username is unavailable", Code: "username_conflict", RequestID: requestID(r.Context())})
		return
	}
	_ = a.store.audit(r.Context(), &item.ID, "auth.register", "user", item.ID, clientIP(r), map[string]any{})
	user, err := a.store.findUserByID(r.Context(), item.ID)
	if err != nil {
		internal(w, r, err)
		return
	}
	token, session, err := a.sessions.create(r.Context(), user)
	if err != nil {
		internal(w, r, err)
		return
	}
	a.setSessionCookie(w, token)
	write(w, http.StatusCreated, sessionResponse(session))
}

func (a *app) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	session, _ := currentSession(r.Context())
	user, err := a.store.findUserByID(r.Context(), session.UserID)
	if err != nil || !verifyPassword(user.PasswordHash, input.CurrentPassword) {
		write(w, http.StatusUnauthorized, apiError{Error: "invalid credentials", Code: "invalid_credentials", RequestID: requestID(r.Context())})
		return
	}
	hash, err := hashPassword(input.NewPassword)
	if err != nil {
		write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "weak_password", RequestID: requestID(r.Context())})
		return
	}
	if err := a.store.setPassword(r.Context(), user.ID, hash, false); err != nil {
		internal(w, r, err)
		return
	}
	_ = a.store.audit(r.Context(), &user.ID, "auth.password_change", "user", user.ID, clientIP(r), map[string]any{})
	if cookie, _ := r.Cookie(sessionCookie); cookie != nil {
		_ = a.sessions.delete(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: a.cfg.CookieSecure})
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) adminUsers(w http.ResponseWriter, r *http.Request) {
	session, _ := currentSession(r.Context())
	if session.Role != "owner" {
		forbidden(w, r)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users"), "/")
	if path == "" {
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		items, err := a.store.listUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), 100)
		if err != nil {
			internal(w, r, err)
			return
		}
		write(w, http.StatusOK, items)
		return
	}
	parts := strings.Split(path, "/")
	if uuid.Validate(parts[0]) != nil {
		notFound(w, r)
		return
	}
	target := parts[0]
	if len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPut {
		var input struct {
			Status string `json:"status"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if input.Status != "active" && input.Status != "suspended" {
			write(w, http.StatusBadRequest, apiError{Error: "invalid user status", Code: "invalid_status", RequestID: requestID(r.Context())})
			return
		}
		if err := a.store.setUserStatus(r.Context(), target, input.Status); err != nil {
			notFound(w, r)
			return
		}
		_ = a.store.audit(r.Context(), &session.UserID, "admin.user.status", "user", target, clientIP(r), map[string]any{"status": input.Status})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 2 && parts[1] == "reset-password" && r.Method == http.MethodPost {
		var input struct {
			Password string `json:"password"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		user, err := a.store.findUserByID(r.Context(), target)
		if err != nil || user.Role != "user" {
			notFound(w, r)
			return
		}
		hash, err := hashPassword(input.Password)
		if err != nil {
			write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "weak_password", RequestID: requestID(r.Context())})
			return
		}
		if err := a.store.setPassword(r.Context(), target, hash, true); err != nil {
			notFound(w, r)
			return
		}
		_ = a.store.audit(r.Context(), &session.UserID, "admin.user.password_reset", "user", target, clientIP(r), map[string]any{})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	notFound(w, r)
}

func sessionResponse(s sessionRecord) map[string]any {
	return map[string]any{"userId": s.UserID, "username": s.Username, "role": s.Role, "mustChangePassword": s.MustChangePassword, "csrfToken": s.CSRFToken}
}

var _ = errors.Is
