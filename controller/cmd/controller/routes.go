package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+\-]{0,31}$`)

func (a *app) catalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	write(w, http.StatusOK, []map[string]any{
		{"id": "vanilla", "name": "Vanilla", "java": 21},
		{"id": "paper", "name": "Paper", "java": 21},
		{"id": "purpur", "name": "Purpur", "java": 21},
		{"id": "fabric", "name": "Fabric", "java": 21},
		{"id": "forge", "name": "Forge", "java": 21},
		{"id": "neoforge", "name": "NeoForge", "java": 21},
	})
}

func (a *app) serverCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.store.list(r.Context())
		if err != nil {
			internal(w, r, err)
			return
		}
		write(w, http.StatusOK, items)
	case http.MethodPost:
		var input createServerInput
		if decode(w, r, &input) != nil {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		input.Runtime = strings.ToLower(strings.TrimSpace(input.Runtime))
		input.Version = strings.TrimSpace(input.Version)
		if input.DiskMB == 0 {
			input.DiskMB = 10240
		}
		if input.Name == "" || len(input.Name) > 48 || !runtimeOK(input.Runtime) ||
			!versionPattern.MatchString(input.Version) || input.MemoryMB < 1024 || input.MemoryMB > 8192 ||
			input.CPU < 1 || input.CPU > a.cfg.NodeCPUs || input.DiskMB < 1024 || input.DiskMB > 102400 {
			write(w, http.StatusBadRequest, apiError{Error: "invalid server configuration", Code: "invalid_server", RequestID: requestID(r.Context())})
			return
		}
		item, err := a.store.create(r.Context(), input, a.cfg)
		if errors.Is(err, errCapacity) || errors.Is(err, errNoAllocation) {
			write(w, http.StatusConflict, apiError{Error: err.Error(), Code: "capacity_conflict", RequestID: requestID(r.Context())})
			return
		}
		if err != nil {
			internal(w, r, err)
			return
		}
		createdJob, err := a.store.createJob(r.Context(), item.ID, "provision", map[string]any{})
		if err != nil {
			message := "could not enqueue provisioning"
			_ = a.store.setObservedState(r.Context(), item.ID, "error", &message)
			internal(w, r, err)
			return
		}
		item.CurrentJob = &createdJob
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "server.create", "server", item.ID, clientIP(r), map[string]any{"name": item.Name})
		write(w, http.StatusAccepted, map[string]any{"server": item, "job": createdJob})
	default:
		method(w)
	}
}

func (a *app) serverItem(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/servers/"), "/"), "/")
	if len(parts) == 0 || uuid.Validate(parts[0]) != nil {
		notFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			item, err := a.store.get(r.Context(), id)
			if errors.Is(err, pgx.ErrNoRows) {
				notFound(w, r)
				return
			}
			if err != nil {
				internal(w, r, err)
				return
			}
			write(w, http.StatusOK, item)
		case http.MethodDelete:
			var input struct {
				PurgeData bool `json:"purgeData"`
			}
			if decode(w, r, &input) != nil {
				return
			}
			a.enqueueAction(w, r, id, "delete", input)
		default:
			method(w)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "actions" && r.Method == http.MethodPost {
		var input struct {
			Action string `json:"action"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if input.Action != "start" && input.Action != "stop" && input.Action != "restart" {
			write(w, http.StatusBadRequest, apiError{Error: "unknown action", Code: "unknown_action", RequestID: requestID(r.Context())})
			return
		}
		a.enqueueAction(w, r, id, input.Action, map[string]any{})
		return
	}
	if len(parts) == 2 && parts[1] == "config" {
		switch r.Method {
		case http.MethodGet:
			item, err := a.store.get(r.Context(), id)
			if err != nil {
				notFound(w, r)
				return
			}
			write(w, http.StatusOK, item.Config)
		case http.MethodPut:
			var input map[string]any
			if decode(w, r, &input) != nil {
				return
			}
			if err := validateServerConfig(input); err != nil {
				write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "invalid_config", RequestID: requestID(r.Context())})
				return
			}
			data, _ := json.Marshal(input)
			createdJob, err := a.store.updateConfigJob(r.Context(), id, data)
			if err != nil {
				write(w, http.StatusConflict, apiError{Error: "another update is active", Code: "job_conflict", RequestID: requestID(r.Context())})
				return
			}
			item, err := a.store.get(r.Context(), id)
			if err != nil {
				internal(w, r, err)
				return
			}
			session, _ := currentSession(r.Context())
			keys := make([]string, 0, len(input))
			for key := range input {
				keys = append(keys, key)
			}
			_ = a.store.audit(r.Context(), &session.UserID, "server.config.update", "server", id, clientIP(r), map[string]any{"keys": keys})
			write(w, http.StatusAccepted, map[string]any{"server": item, "job": createdJob})
		default:
			method(w)
		}
		return
	}
	a.serverFeature(w, r, id, parts[1:])
}

func (a *app) enqueueAction(w http.ResponseWriter, r *http.Request, id, action string, payload any) {
	item, err := a.store.get(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w, r)
		return
	}
	if err != nil {
		internal(w, r, err)
		return
	}
	desired := item.DesiredState
	if action == "start" || action == "restart" {
		desired = "running"
	} else if action == "stop" {
		desired = "offline"
	} else if action == "delete" {
		desired = "deleted"
	}
	createdJob, err := a.store.createLifecycleJob(r.Context(), id, action, desired, payload)
	if err != nil {
		write(w, http.StatusConflict, apiError{Error: "another operation is already active", Code: "job_conflict", RequestID: requestID(r.Context())})
		return
	}
	item.DesiredState = desired
	item.CurrentJob = &createdJob
	session, _ := currentSession(r.Context())
	_ = a.store.audit(r.Context(), &session.UserID, "server."+action, "server", id, clientIP(r), map[string]any{})
	write(w, http.StatusAccepted, map[string]any{"server": item, "job": createdJob})
}

func (a *app) jobItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/"), "/")
	if uuid.Validate(id) != nil {
		notFound(w, r)
		return
	}
	item, err := a.store.getJob(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w, r)
		return
	}
	if err != nil {
		internal(w, r, err)
		return
	}
	write(w, http.StatusOK, item)
}

func (a *app) auditCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	items, err := a.store.listAudit(r.Context(), 100)
	if err != nil {
		internal(w, r, err)
		return
	}
	write(w, http.StatusOK, items)
}

func runtimeOK(value string) bool {
	return map[string]bool{"vanilla": true, "paper": true, "purpur": true, "fabric": true, "forge": true, "neoforge": true}[value]
}

func validateServerConfig(input map[string]any) error {
	for key, value := range input {
		switch key {
		case "motd":
			text, ok := value.(string)
			if !ok || len(text) > 160 || strings.ContainsAny(text, "\r\n\x00") {
				return errors.New("motd must be a single line of at most 160 characters")
			}
		case "difficulty":
			text, ok := value.(string)
			if !ok || !map[string]bool{"peaceful": true, "easy": true, "normal": true, "hard": true}[text] {
				return errors.New("difficulty is invalid")
			}
		case "gamemode":
			text, ok := value.(string)
			if !ok || !map[string]bool{"survival": true, "creative": true, "adventure": true, "spectator": true}[text] {
				return errors.New("gamemode is invalid")
			}
		case "maxPlayers":
			if !jsonIntegerInRange(value, 1, 1000) {
				return errors.New("maxPlayers must be between 1 and 1000")
			}
		case "viewDistance", "simulationDistance":
			if !jsonIntegerInRange(value, 2, 32) {
				return errors.New(key + " must be between 2 and 32")
			}
		case "onlineMode", "whiteList":
			if _, ok := value.(bool); !ok {
				return errors.New(key + " must be a boolean")
			}
		case "whiteListPlayers":
			text, ok := value.(string)
			if !ok || len(text) > 1024 || strings.ContainsAny(text, "\r\n\x00") {
				return errors.New("whiteListPlayers is invalid")
			}
		default:
			return errors.New("unsupported configuration entry: " + key)
		}
	}
	return nil
}

func jsonIntegerInRange(value any, minimum, maximum int) bool {
	number, ok := value.(float64)
	return ok && number == float64(int(number)) && number >= float64(minimum) && number <= float64(maximum)
}
