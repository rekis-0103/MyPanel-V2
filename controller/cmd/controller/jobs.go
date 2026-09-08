package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var processExitCodePattern = regexp.MustCompile(`(?i)process exited with code (-?[0-9]+)`)

func (a *app) runJobWorker(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				item, err := a.store.claimJob(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					break
				}
				if err != nil {
					log.Printf("claim job failed error=%v", err)
					break
				}
				a.executeJob(ctx, item)
			}
		}
	}
}

func (a *app) executeJob(parent context.Context, item job) {
	timeout := 15 * time.Minute
	if item.Action == "modpack_install" {
		timeout = 45 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	serverItem, err := a.store.get(ctx, item.ServerID)
	if err != nil {
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer finishCancel()
		_ = a.store.finishJob(finishCtx, item.ID, map[string]any{}, err)
		return
	}
	transition := map[string]string{"provision": "installing", "start": "starting", "restart": "stopping", "stop": "stopping", "delete": "deleting", "release": "deleting", "update": "installing", "modpack_install": "installing"}[item.Action]
	if transition != "" {
		_ = a.store.setObservedState(ctx, item.ServerID, transition, nil)
	}
	spec := agentServerSpec{ID: serverItem.ID, Runtime: serverItem.Runtime, Version: serverItem.Version, JavaVersion: serverItem.JavaVersion,
		MemoryMB: serverItem.MemoryMB, CPU: serverItem.CPU, DiskMB: serverItem.DiskMB,
		BindIP: serverItem.BindIP, Port: serverItem.Port, Config: serverItem.Config}
	if installed, modpackErr := a.store.getModpack(ctx, item.ServerID); modpackErr == nil {
		spec.Modpack = &agentModpackSpec{Provider: installed.Provider, Slug: installed.Slug, FileID: installed.FileID}
	} else if !errors.Is(modpackErr, pgx.ErrNoRows) {
		err = modpackErr
	}
	var result any = map[string]any{}
	if message := lifecycleActionMessage(item.Action); message != "" {
		a.recordConsoleEvent(ctx, item.ServerID, message)
	}
	switch {
	case err != nil:
		// Loading the managed runtime definition failed; finish the job below.
	case item.Action == "modpack_install":
		err = a.executeModpackInstall(ctx, item, serverItem, spec, &result)
	case item.Action == "provision" || item.Action == "update":
		err = a.agent.serverAction(ctx, item.ServerID, "provision", spec, &result)
		if err == nil {
			_ = a.store.setObservedState(ctx, item.ServerID, "offline", nil)
		}
	case item.Action == "start" || item.Action == "stop" || item.Action == "restart":
		err = a.agent.serverAction(ctx, item.ServerID, item.Action, spec, &result)
		if err == nil && (item.Action == "start" || item.Action == "restart") {
			_, err = a.waitForServerRunning(ctx, item.ServerID)
		}
		if err == nil {
			_ = a.store.setObservedState(ctx, item.ServerID, lifecycleCompletionState(item.Action), nil)
			if item.Action == "start" || item.Action == "restart" {
				_ = a.store.clearRestartRequired(ctx, item.ServerID)
			}
			if item.Action == "stop" {
				a.recordConsoleEvent(ctx, item.ServerID, "Server marked as stopped.")
			} else {
				a.recordConsoleEvent(ctx, item.ServerID, "Server marked as running.")
				if item.Action == "restart" {
					a.recordConsoleEvent(ctx, item.ServerID, "Restart successful.")
				}
			}
		}
	case item.Action == "delete":
		var payload struct {
			PurgeData bool `json:"purgeData"`
		}
		_ = json.Unmarshal(item.Payload, &payload)
		err = a.agent.serverAction(ctx, item.ServerID, "delete", payload, &result)
		if err == nil {
			err = a.store.finalizeDelete(ctx, item.ServerID)
		}
	case item.Action == "release":
		err = a.agent.serverAction(ctx, item.ServerID, "delete", map[string]any{"purgeData": false}, &result)
		if err == nil {
			err = a.store.finalizeRelease(ctx, item.ServerID)
		}
	default:
		err = a.executeFeatureJob(ctx, item, serverItem, &result)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err != nil {
		message := err.Error()
		_ = a.store.setObservedState(finishCtx, item.ServerID, "error", &message)
		if isLifecycleAction(item.Action) {
			a.recordConsoleEvent(finishCtx, item.ServerID, lifecycleFailureMessage(item.Action, err))
		}
		log.Printf("job failed id=%s action=%s server=%s error=%v", item.ID, item.Action, item.ServerID, err)
	}
	if item.Action == "provision" {
		_ = a.store.setSubscriptionProvisionResult(finishCtx, item.ServerID, err)
	}
	if finishErr := a.store.finishJob(finishCtx, item.ID, result, err); finishErr != nil {
		log.Printf("finish job failed id=%s error=%v", item.ID, finishErr)
	}
}

func lifecycleActionMessage(action string) string {
	switch action {
	case "start":
		return "Starting server..."
	case "restart":
		return "Restarting server..."
	default:
		return ""
	}
}

func isLifecycleAction(action string) bool {
	return action == "start" || action == "stop" || action == "restart"
}

func lifecycleFailureMessage(action string, err error) string {
	reason := "node could not complete the operation"
	value := strings.ToLower(err.Error())
	switch {
	case strings.Contains(value, "memory limit"):
		reason = "process exceeded the server memory limit"
	case strings.Contains(value, "disk limit"):
		reason = "server disk limit exceeded"
	case processExitCodePattern.MatchString(value):
		reason = "process exited with code " + processExitCodePattern.FindStringSubmatch(value)[1]
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(value, "health check did not report ready"):
		reason = "Minecraft health check did not report ready before the timeout"
	case strings.Contains(value, "connection refused") || strings.Contains(value, "node unavailable"):
		reason = "node is unavailable"
	}
	return fmt.Sprintf("Server error during %s: %s.", action, strings.TrimSuffix(reason, "."))
}

func (a *app) recordConsoleEvent(ctx context.Context, serverID, message string) {
	if err := a.store.addConsoleEvent(ctx, serverID, message); err != nil {
		log.Printf("record console event failed server=%s error=%v", serverID, err)
	}
}

func (a *app) waitForServerRunning(parent context.Context, serverID string) (agentState, error) {
	return a.waitForServerRunningWithin(parent, serverID, 10*time.Minute)
}

func (a *app) waitForServerRunningWithin(parent context.Context, serverID string, timeout time.Duration) (agentState, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := a.agent.readiness(ctx, serverID)
		if err != nil {
			return agentState{}, fmt.Errorf("node unavailable while waiting for Minecraft readiness: %w", err)
		}
		switch state.State {
		case "running":
			return state, nil
		case "offline", "error":
			reason := strings.TrimSpace(state.Reason)
			if reason == "" {
				reason = "process stopped before Minecraft became ready"
			}
			return state, errors.New(reason)
		}
		select {
		case <-ctx.Done():
			return agentState{}, errors.New("Minecraft health check did not report ready before the timeout")
		case <-ticker.C:
		}
	}
}

func lifecycleCompletionState(action string) string {
	if action == "stop" {
		return "offline"
	}
	return "running"
}

func (a *app) runReconciler(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			items, err := a.store.list(ctx)
			if err != nil {
				log.Printf("reconcile list failed error=%v", err)
				continue
			}
			for _, item := range items {
				if item.CurrentJob != nil {
					continue
				}
				state, err := a.agent.state(ctx, item.ID)
				if err != nil {
					continue
				}
				if state.State != "" && state.State != item.State {
					nextState := state.State
					var lastError *string
					if item.DesiredState == "running" && (state.State == "offline" || state.State == "error") {
						reason := strings.TrimSpace(state.Reason)
						if reason == "" {
							reason = "process stopped unexpectedly"
						}
						message := lifecycleFailureMessage("runtime", errors.New(reason))
						a.recordConsoleEvent(ctx, item.ID, message)
						nextState = "error"
						lastError = &reason
					} else if state.State == "running" {
						a.recordConsoleEvent(ctx, item.ID, "Server marked as running.")
					} else if state.State == "offline" {
						a.recordConsoleEvent(ctx, item.ID, "Server marked as stopped.")
					}
					_ = a.store.setObservedState(ctx, item.ID, nextState, lastError)
				}
				correctiveAction := ""
				if item.DesiredState == "running" && (state.State == "offline" || state.State == "error") {
					correctiveAction = "start"
				} else if item.DesiredState == "offline" && state.State == "running" {
					correctiveAction = "stop"
				}
				if correctiveAction != "" {
					if _, err := a.store.createJob(ctx, item.ID, correctiveAction, map[string]any{"source": "reconciler"}); err != nil {
						log.Printf("enqueue reconciliation failed server=%s action=%s error=%v", item.ID, correctiveAction, err)
					}
				}
			}
		}
	}
}

func (a *app) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.SchedulerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.enqueueDueSchedules(ctx); err != nil {
				log.Printf("scheduler failed error=%v", err)
			}
		}
	}
}
