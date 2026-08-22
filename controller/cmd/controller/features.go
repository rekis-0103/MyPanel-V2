package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *app) serverFeature(w http.ResponseWriter, r *http.Request, serverID string, parts []string) {
	if len(parts) == 0 {
		notFound(w, r)
		return
	}
	if _, err := a.store.get(r.Context(), serverID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(w, r)
		} else {
			internal(w, r, err)
		}
		return
	}
	switch parts[0] {
	case "logs":
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		logs, err := a.agent.logs(r.Context(), serverID)
		if err != nil {
			write(w, http.StatusBadGateway, apiError{Error: "node console unavailable", Code: "node_unavailable", RequestID: requestID(r.Context())})
			return
		}
		write(w, http.StatusOK, map[string]string{"logs": logs})
	case "metrics":
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		state, err := a.agent.state(r.Context(), serverID)
		if err != nil {
			write(w, http.StatusBadGateway, apiError{Error: "node metrics unavailable", Code: "node_unavailable", RequestID: requestID(r.Context())})
			return
		}
		write(w, http.StatusOK, state)
	case "files":
		a.fileCollection(w, r, serverID)
	case "backups":
		a.backupRoutes(w, r, serverID, parts[1:])
	case "schedules":
		a.scheduleRoutes(w, r, serverID, parts[1:])
	case "console":
		a.console(w, r, serverID)
	default:
		notFound(w, r)
	}
}

func (a *app) fileCollection(w http.ResponseWriter, r *http.Request, serverID string) {
	filePath := r.URL.Query().Get("path")
	if len(filePath) > 512 || strings.ContainsRune(filePath, '\x00') {
		write(w, http.StatusBadRequest, apiError{Error: "invalid file path", Code: "invalid_file", RequestID: requestID(r.Context())})
		return
	}
	switch r.Method {
	case http.MethodGet:
		out, err := a.agent.files(r.Context(), serverID, filePath)
		if err != nil {
			write(w, http.StatusBadGateway, apiError{Error: "node file operation failed", Code: "node_operation_failed", RequestID: requestID(r.Context())})
			return
		}
		write(w, http.StatusOK, out)
	case http.MethodPut:
		var input struct {
			Path     string `json:"path"`
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if len(input.Path) > 512 {
			write(w, http.StatusBadRequest, apiError{Error: "invalid file path", Code: "invalid_file", RequestID: requestID(r.Context())})
			return
		}
		out, err := a.agent.writeFile(r.Context(), serverID, input)
		if err != nil {
			write(w, http.StatusBadGateway, apiError{Error: "node file operation failed", Code: "node_operation_failed", RequestID: requestID(r.Context())})
			return
		}
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "file.write", "server", serverID, clientIP(r), map[string]any{"path": input.Path})
		write(w, http.StatusOK, out)
	case http.MethodDelete:
		if filePath == "" {
			write(w, http.StatusBadRequest, apiError{Error: "file path is required", Code: "invalid_file", RequestID: requestID(r.Context())})
			return
		}
		if err := a.agent.deleteFile(r.Context(), serverID, filePath); err != nil {
			write(w, http.StatusBadGateway, apiError{Error: "node file operation failed", Code: "node_operation_failed", RequestID: requestID(r.Context())})
			return
		}
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "file.delete", "server", serverID, clientIP(r), map[string]any{"path": filePath})
		w.WriteHeader(http.StatusNoContent)
	default:
		method(w)
	}
}

func (a *app) backupRoutes(w http.ResponseWriter, r *http.Request, serverID string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := a.store.listBackups(r.Context(), serverID)
			if err != nil {
				internal(w, r, err)
				return
			}
			write(w, http.StatusOK, items)
		case http.MethodPost:
			var input struct {
				Name string `json:"name"`
			}
			if decode(w, r, &input) != nil {
				return
			}
			input.Name = strings.TrimSpace(input.Name)
			if input.Name == "" {
				input.Name = "Backup " + time.Now().Format("2006-01-02 15:04")
			}
			if len(input.Name) > 64 {
				write(w, http.StatusBadRequest, apiError{Error: "backup name is too long", Code: "invalid_backup", RequestID: requestID(r.Context())})
				return
			}
			item, createdJob, err := a.store.createBackupJob(r.Context(), serverID, input.Name)
			if err != nil {
				write(w, http.StatusConflict, apiError{Error: "another operation is active", Code: "job_conflict", RequestID: requestID(r.Context())})
				return
			}
			session, _ := currentSession(r.Context())
			_ = a.store.audit(r.Context(), &session.UserID, "backup.create", "server", serverID, clientIP(r), map[string]any{"backupId": item.ID})
			write(w, http.StatusAccepted, map[string]any{"backup": item, "job": createdJob})
		default:
			method(w)
		}
		return
	}
	backupID := parts[0]
	if uuid.Validate(backupID) != nil {
		notFound(w, r)
		return
	}
	item, err := a.store.getBackup(r.Context(), serverID, backupID)
	if err != nil {
		notFound(w, r)
		return
	}
	if len(parts) == 2 && parts[1] == "restore" && r.Method == http.MethodPost {
		if item.Status != "ready" {
			write(w, http.StatusConflict, apiError{Error: "backup is not ready", Code: "backup_not_ready", RequestID: requestID(r.Context())})
			return
		}
		createdJob, err := a.store.createJob(r.Context(), serverID, "restore", map[string]string{"backupId": backupID})
		if err != nil {
			write(w, http.StatusConflict, apiError{Error: "another operation is active", Code: "job_conflict", RequestID: requestID(r.Context())})
			return
		}
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "backup.restore", "backup", backupID, clientIP(r), map[string]any{"serverId": serverID})
		write(w, http.StatusAccepted, map[string]any{"backup": item, "job": createdJob})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if item.Status != "ready" && item.Status != "failed" {
			write(w, http.StatusConflict, apiError{Error: "backup operation is active", Code: "backup_active", RequestID: requestID(r.Context())})
			return
		}
		if err := a.agent.deleteBackup(r.Context(), serverID, backupID); err != nil {
			write(w, http.StatusBadGateway, apiError{Error: "node backup deletion failed", Code: "node_operation_failed", RequestID: requestID(r.Context())})
			return
		}
		if err := a.store.deleteBackup(r.Context(), serverID, backupID); err != nil {
			internal(w, r, err)
			return
		}
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "backup.delete", "backup", backupID, clientIP(r), map[string]any{"serverId": serverID})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	method(w)
}

func (a *app) scheduleRoutes(w http.ResponseWriter, r *http.Request, serverID string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := a.store.listSchedules(r.Context(), serverID)
			if err != nil {
				internal(w, r, err)
				return
			}
			write(w, http.StatusOK, items)
		case http.MethodPost:
			var input struct {
				Name            string         `json:"name"`
				Action          string         `json:"action"`
				Payload         map[string]any `json:"payload"`
				IntervalMinutes int            `json:"intervalMinutes"`
			}
			if decode(w, r, &input) != nil {
				return
			}
			input.Name = strings.TrimSpace(input.Name)
			if input.Name == "" || len(input.Name) > 64 || !map[string]bool{"start": true, "stop": true, "restart": true, "backup": true, "command": true}[input.Action] || input.IntervalMinutes < 1 || input.IntervalMinutes > 525600 {
				write(w, http.StatusBadRequest, apiError{Error: "invalid schedule", Code: "invalid_schedule", RequestID: requestID(r.Context())})
				return
			}
			if input.Action == "command" {
				command, _ := input.Payload["command"].(string)
				if !validCommand(command) {
					write(w, http.StatusBadRequest, apiError{Error: "invalid scheduled command", Code: "invalid_schedule", RequestID: requestID(r.Context())})
					return
				}
			}
			item, err := a.store.createSchedule(r.Context(), serverID, input.Name, input.Action, input.Payload, input.IntervalMinutes)
			if err != nil {
				internal(w, r, err)
				return
			}
			session, _ := currentSession(r.Context())
			_ = a.store.audit(r.Context(), &session.UserID, "schedule.create", "schedule", item.ID, clientIP(r), map[string]any{"serverId": serverID, "action": input.Action})
			write(w, http.StatusCreated, item)
		default:
			method(w)
		}
		return
	}
	if len(parts) == 1 && uuid.Validate(parts[0]) == nil && r.Method == http.MethodDelete {
		if err := a.store.deleteSchedule(r.Context(), serverID, parts[0]); err != nil {
			notFound(w, r)
			return
		}
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "schedule.delete", "schedule", parts[0], clientIP(r), map[string]any{"serverId": serverID})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	method(w)
}

func (a *app) console(w http.ResponseWriter, r *http.Request, serverID string) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	if !a.validOrigin(r) {
		write(w, http.StatusForbidden, apiError{Error: "origin validation failed", Code: "origin_failed", RequestID: requestID(r.Context())})
		return
	}
	// Origin is checked above against the same policy as HTTP mutations.
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true, CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return
	}
	defer connection.Close(websocket.StatusNormalClosure, "console closed")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	type incomingMessage struct {
		Type      string `json:"type"`
		Command   string `json:"command"`
		CSRFToken string `json:"csrfToken"`
	}
	incoming := make(chan incomingMessage)
	readErrors := make(chan error, 1)
	go func() {
		for {
			_, data, err := connection.Read(ctx)
			if err != nil {
				readErrors <- err
				return
			}
			var message incomingMessage
			if json.Unmarshal(data, &message) == nil {
				incoming <- message
			}
		}
	}()
	logUpdates := make(chan consoleLogUpdate, 8)
	statusUpdates := make(chan agentState, 1)
	go a.pollConsoleLogs(ctx, serverID, logUpdates)
	go a.pollConsoleState(ctx, serverID, statusUpdates)
	session, _ := currentSession(r.Context())
	for {
		select {
		case <-ctx.Done():
			return
		case <-readErrors:
			return
		case message := <-incoming:
			if message.Type != "command" || subtle.ConstantTimeCompare([]byte(message.CSRFToken), []byte(session.CSRFToken)) != 1 || !validCommand(message.Command) {
				_ = connection.Write(ctx, websocket.MessageText, mustJSON(map[string]any{"type": "command-result", "error": "command rejected"}))
				continue
			}
			output, err := a.agent.command(ctx, serverID, strings.TrimSpace(message.Command))
			result := map[string]any{"type": "command-result", "output": output}
			if err != nil {
				result["error"] = "node command failed"
			}
			commandName := strings.Fields(strings.TrimSpace(message.Command))[0]
			_ = a.store.audit(ctx, &session.UserID, "console.command", "server", serverID, clientIP(r), map[string]any{"commandName": commandName, "length": len(message.Command)})
			if connection.Write(ctx, websocket.MessageText, mustJSON(result)) != nil {
				return
			}
		case update := <-logUpdates:
			if connection.Write(ctx, websocket.MessageText, mustJSON(map[string]any{"type": "log", "logs": update.Logs, "reset": update.Reset})) != nil {
				return
			}
		case state := <-statusUpdates:
			if connection.Write(ctx, websocket.MessageText, mustJSON(map[string]any{"type": "status", "state": state.State, "metrics": state})) != nil {
				return
			}
		}
	}
}

type consoleLogUpdate struct {
	Logs  string
	Reset bool
}

func (a *app) pollConsoleLogs(ctx context.Context, serverID string, updates chan<- consoleLogUpdate) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	previous := ""
	for {
		logs, err := a.agent.logs(ctx, serverID)
		if err == nil && logs != previous {
			delta, reset := consoleDelta(previous, logs)
			previous = logs
			if delta != "" {
				select {
				case updates <- consoleLogUpdate{Logs: delta, Reset: reset}:
				case <-ctx.Done():
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *app) pollConsoleState(ctx context.Context, serverID string, updates chan<- agentState) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		state, err := a.agent.state(ctx, serverID)
		if err == nil {
			select {
			case updates <- state:
			case <-ctx.Done():
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func consoleDelta(previous, current string) (string, bool) {
	if previous == "" {
		return current, true
	}
	if strings.HasPrefix(current, previous) {
		return current[len(previous):], false
	}
	previousLines := strings.SplitAfter(previous, "\n")
	currentLines := strings.SplitAfter(current, "\n")
	if previousLines[len(previousLines)-1] == "" {
		previousLines = previousLines[:len(previousLines)-1]
	}
	if currentLines[len(currentLines)-1] == "" {
		currentLines = currentLines[:len(currentLines)-1]
	}
	for overlap := min(len(previousLines), len(currentLines)); overlap > 0; overlap-- {
		matched := true
		for index := 0; index < overlap; index++ {
			if previousLines[len(previousLines)-overlap+index] != currentLines[index] {
				matched = false
				break
			}
		}
		if matched {
			return strings.Join(currentLines[overlap:], ""), false
		}
	}
	return current, true
}

func (a *app) executeFeatureJob(ctx context.Context, item job, serverItem server, result *any) error {
	switch item.Action {
	case "backup":
		var payload struct {
			BackupID string `json:"backupId"`
		}
		if err := json.Unmarshal(item.Payload, &payload); err != nil || uuid.Validate(payload.BackupID) != nil {
			return errors.New("invalid backup job payload")
		}
		_ = a.store.updateBackup(ctx, payload.BackupID, "creating", 0, "", nil)
		var output struct {
			BackupID       string `json:"backupId"`
			SizeBytes      int64  `json:"sizeBytes"`
			ChecksumSHA256 string `json:"checksumSha256"`
		}
		err := a.agent.serverAction(ctx, item.ServerID, "backup", payload, &output)
		status := "ready"
		if err != nil {
			status = "failed"
		}
		_ = a.store.updateBackup(ctx, payload.BackupID, status, output.SizeBytes, output.ChecksumSHA256, err)
		*result = output
		return err
	case "restore":
		var payload struct {
			BackupID string `json:"backupId"`
		}
		if err := json.Unmarshal(item.Payload, &payload); err != nil || uuid.Validate(payload.BackupID) != nil {
			return errors.New("invalid restore job payload")
		}
		backupItem, err := a.store.getBackup(ctx, item.ServerID, payload.BackupID)
		if err != nil {
			return err
		}
		_ = a.store.updateBackup(ctx, payload.BackupID, "restoring", backupItem.SizeBytes, valueOrEmpty(backupItem.ChecksumSHA256), nil)
		err = a.agent.serverAction(ctx, item.ServerID, "restore", payload, result)
		_ = a.store.updateBackup(ctx, payload.BackupID, "ready", backupItem.SizeBytes, valueOrEmpty(backupItem.ChecksumSHA256), err)
		if err == nil {
			_ = a.store.setDesiredState(ctx, item.ServerID, "offline")
			_ = a.store.setObservedState(ctx, item.ServerID, "offline", nil)
		}
		return err
	case "command":
		var payload struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(item.Payload, &payload) != nil || !validCommand(payload.Command) {
			return errors.New("invalid command job payload")
		}
		output, err := a.agent.command(ctx, item.ServerID, payload.Command)
		*result = map[string]string{"output": output}
		return err
	default:
		return fmt.Errorf("unsupported job action %q", item.Action)
	}
}

func (a *app) enqueueDueSchedules(ctx context.Context) error {
	items, err := a.store.claimDueSchedules(ctx, 20)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Action == "backup" {
			_, _, _ = a.store.createBackupJob(ctx, item.ServerID, "Scheduled: "+item.Name)
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal(item.Payload, &payload)
		if item.Action == "start" || item.Action == "restart" {
			_, _ = a.store.createLifecycleJob(ctx, item.ServerID, item.Action, "running", payload)
		} else if item.Action == "stop" {
			_, _ = a.store.createLifecycleJob(ctx, item.ServerID, item.Action, "offline", payload)
		} else {
			_, _ = a.store.createJob(ctx, item.ServerID, item.Action, payload)
		}
	}
	return nil
}

func validCommand(command string) bool {
	command = strings.TrimSpace(command)
	return command != "" && len(command) <= 512 && !strings.ContainsAny(command, "\r\n\x00")
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
