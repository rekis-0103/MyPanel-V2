package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *app) notificationCollection(w http.ResponseWriter, r *http.Request) {
	session, _ := currentSession(r.Context())
	switch r.Method {
	case http.MethodGet:
		items, err := a.store.listNotifications(r.Context(), session.UserID, 100)
		if err != nil {
			internal(w, r, err)
			return
		}
		write(w, http.StatusOK, items)
	case http.MethodPost:
		var input struct {
			Action string `json:"action"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if input.Action != "read_all" {
			write(w, http.StatusBadRequest, apiError{Error: "invalid notification action", Code: "invalid_action", RequestID: requestID(r.Context())})
			return
		}
		if err := a.store.readAllNotifications(r.Context(), session.UserID); err != nil {
			internal(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		method(w)
	}
}

func (a *app) notificationItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/notifications/"), "/")
	if uuid.Validate(id) != nil {
		notFound(w, r)
		return
	}
	session, _ := currentSession(r.Context())
	if err := a.store.readNotification(r.Context(), session.UserID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(w, r)
		} else {
			internal(w, r, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
