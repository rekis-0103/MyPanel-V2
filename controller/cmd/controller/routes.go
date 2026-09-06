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

type catalogVersion struct {
	ID   string `json:"id"`
	Java int    `json:"java"`
}

var supportedMinecraftVersions = []catalogVersion{
	{ID: "26.2", Java: 25},
	{ID: "26.1.1", Java: 25},
	{ID: "26.1", Java: 25},
	{ID: "1.21.11", Java: 21},
	{ID: "1.21.10", Java: 21},
	{ID: "1.21.8", Java: 21},
	{ID: "1.21.5", Java: 21},
	{ID: "1.21.4", Java: 21},
	{ID: "1.21.1", Java: 21},
	{ID: "1.20.6", Java: 21},
}

func requiredJavaVersion(version string) int {
	for _, item := range supportedMinecraftVersions {
		if item.ID == version {
			return item.Java
		}
	}
	if strings.HasPrefix(version, "26.") {
		return 25
	}
	return 21
}

func (a *app) catalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	write(w, http.StatusOK, []map[string]any{
		{"id": "vanilla", "name": "Vanilla", "java": 21, "javaVersions": []int{21, 25}, "icon": "grass", "versions": supportedMinecraftVersions},
		{"id": "paper", "name": "Paper", "java": 21, "javaVersions": []int{21, 25}, "icon": "feather", "versions": supportedMinecraftVersions},
		{"id": "purpur", "name": "Purpur", "java": 21, "javaVersions": []int{21, 25}, "icon": "crystal", "versions": supportedMinecraftVersions},
		{"id": "fabric", "name": "Fabric", "java": 21, "javaVersions": []int{21, 25}, "icon": "grass", "versions": supportedMinecraftVersions},
		{"id": "forge", "name": "Forge", "java": 21, "javaVersions": []int{21, 25}, "icon": "anvil", "versions": supportedMinecraftVersions},
		{"id": "neoforge", "name": "NeoForge", "java": 21, "javaVersions": []int{21, 25}, "icon": "crystal", "versions": supportedMinecraftVersions},
	})
}

func (a *app) serverCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		session, _ := currentSession(r.Context())
		items, err := a.store.listForSession(r.Context(), session)
		if err != nil {
			internal(w, r, err)
			return
		}
		write(w, http.StatusOK, items)
	case http.MethodPost:
		session, _ := currentSession(r.Context())
		if session.Role != "owner" {
			forbidden(w, r)
			return
		}
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
		input.JavaVersion = normalizeJavaVersion(input.JavaVersion)
		if input.Name == "" || len(input.Name) > 48 || !runtimeOK(input.Runtime) ||
			!versionPattern.MatchString(input.Version) || !javaVersionOK(input.JavaVersion) || input.MemoryMB < 1024 || input.MemoryMB > 8192 ||
			input.CPU < 1 || input.CPU > a.cfg.NodeCPUs || input.DiskMB < 1024 || input.DiskMB > a.cfg.NodeDiskMB {
			write(w, http.StatusBadRequest, apiError{Error: "invalid server configuration", Code: "invalid_server", RequestID: requestID(r.Context())})
			return
		}
		item, err := a.store.create(r.Context(), input, a.cfg, session.UserID)
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
	session, _ := currentSession(r.Context())
	if _, err := a.store.getForSession(r.Context(), id, session); err != nil {
		notFound(w, r)
		return
	}
	if session.Role != "owner" && !(len(parts) == 1 && r.Method == http.MethodGet) {
		if status, err := a.store.subscriptionStatus(r.Context(), id); err == nil && (status == "grace" || status == "released" || status == "canceled") {
			forbidden(w, r)
			return
		}
	}
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
			if session.Role != "owner" {
				forbidden(w, r)
				return
			}
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
	session, _ := currentSession(r.Context())
	if _, err := a.store.getForSession(r.Context(), item.ServerID, session); err != nil {
		notFound(w, r)
		return
	}
	write(w, http.StatusOK, item)
}

func (a *app) auditCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	session, _ := currentSession(r.Context())
	items, err := a.store.listAuditForSession(r.Context(), session, 100)
	if err != nil {
		internal(w, r, err)
		return
	}
	write(w, http.StatusOK, items)
}

func runtimeOK(value string) bool {
	return map[string]bool{"vanilla": true, "paper": true, "purpur": true, "fabric": true, "forge": true, "neoforge": true}[value]
}

func javaVersionOK(value int) bool { return value == 21 || value == 25 }

func normalizeJavaVersion(value int) int {
	if value == 0 {
		return 21
	}
	return value
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
		case "jvmOpts", "extraArgs":
			text, ok := value.(string)
			if !ok || !validStartupOption(text) {
				return errors.New(key + " contains unsupported startup characters")
			}
		default:
			return errors.New("unsupported configuration entry: " + key)
		}
	}
	return nil
}

var startupOptionPattern = regexp.MustCompile(`^[0-9A-Za-z._:/=,+%\- ]*$`)

func validStartupOption(value string) bool {
	return len(value) <= 512 && startupOptionPattern.MatchString(value)
}

func jsonIntegerInRange(value any, minimum, maximum int) bool {
	number, ok := value.(float64)
	return ok && number == float64(int(number)) && number >= float64(minimum) && number <= float64(maximum)
}
