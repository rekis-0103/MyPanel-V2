package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

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
	ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
	defer cancel()
	serverItem, err := a.store.get(ctx, item.ServerID)
	if err != nil {
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer finishCancel()
		_ = a.store.finishJob(finishCtx, item.ID, map[string]any{}, err)
		return
	}
	transition := map[string]string{"provision": "installing", "start": "starting", "restart": "stopping", "stop": "stopping", "delete": "deleting", "update": "installing"}[item.Action]
	if transition != "" {
		_ = a.store.setObservedState(ctx, item.ServerID, transition, nil)
	}
	spec := agentServerSpec{ID: serverItem.ID, Runtime: serverItem.Runtime, Version: serverItem.Version, JavaVersion: serverItem.JavaVersion,
		MemoryMB: serverItem.MemoryMB, CPU: serverItem.CPU, DiskMB: serverItem.DiskMB,
		BindIP: serverItem.BindIP, Port: serverItem.Port, Config: serverItem.Config}
	var result any = map[string]any{}
	switch item.Action {
	case "provision", "update":
		err = a.agent.serverAction(ctx, item.ServerID, "provision", spec, &result)
		if err == nil {
			_ = a.store.setObservedState(ctx, item.ServerID, "offline", nil)
		}
	case "start", "stop", "restart":
		err = a.agent.serverAction(ctx, item.ServerID, item.Action, spec, &result)
		if err == nil {
			state := "running"
			if item.Action == "stop" {
				state = "offline"
			}
			_ = a.store.setObservedState(ctx, item.ServerID, state, nil)
		}
	case "delete":
		var payload struct {
			PurgeData bool `json:"purgeData"`
		}
		_ = json.Unmarshal(item.Payload, &payload)
		err = a.agent.serverAction(ctx, item.ServerID, "delete", payload, &result)
		if err == nil {
			err = a.store.finalizeDelete(ctx, item.ServerID)
		}
	default:
		err = a.executeFeatureJob(ctx, item, serverItem, &result)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err != nil {
		message := err.Error()
		_ = a.store.setObservedState(finishCtx, item.ServerID, "error", &message)
		log.Printf("job failed id=%s action=%s server=%s error=%v", item.ID, item.Action, item.ServerID, err)
	}
	if finishErr := a.store.finishJob(finishCtx, item.ID, result, err); finishErr != nil {
		log.Printf("finish job failed id=%s error=%v", item.ID, finishErr)
	}
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
					_ = a.store.setObservedState(ctx, item.ID, state.State, nil)
				}
				correctiveAction := ""
				if item.DesiredState == "running" && state.State == "offline" {
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
