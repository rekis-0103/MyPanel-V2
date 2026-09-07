package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// serverMetricsCollection collapses the browser's per-server metric fan-out
// into one ownership-filtered request. Agent calls remain bounded so a large
// owner fleet cannot exhaust controller connections.
func (a *app) serverMetricsCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	session, _ := currentSession(r.Context())
	servers, err := a.store.listForSession(r.Context(), session)
	if err != nil {
		internal(w, r, err)
		return
	}
	items := a.collectMetrics(r.Context(), servers)
	write(w, http.StatusOK, map[string]any{"sampledAt": time.Now().UTC(), "items": items})
}

func (a *app) collectMetrics(ctx context.Context, servers []server) map[string]agentState {
	items := make(map[string]agentState, len(servers))
	var mu sync.Mutex
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, server := range servers {
		serverID := server.ID
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			state, stateErr := a.agent.state(ctx, serverID)
			if stateErr != nil {
				return
			}
			mu.Lock()
			items[serverID] = state
			mu.Unlock()
		}()
	}
	wg.Wait()
	return items
}

func (a *app) runMetricsCollector(ctx context.Context) {
	// Collect immediately after startup, then on minute boundaries.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			servers, err := a.store.list(ctx)
			if err != nil {
				log.Printf("metric collector list failed error=%v", err)
			} else {
				for id, state := range a.collectMetrics(ctx, servers) {
					if err := a.store.putMetric(ctx, id, state); err != nil {
						log.Printf("metric sample failed server=%s error=%v", id, err)
					}
					for _, item := range servers {
						if item.ID == id {
							a.evaluateMetricAlerts(ctx, item, state)
							break
						}
					}
				}
				if err := a.store.maintainMetrics(ctx); err != nil {
					log.Printf("metric retention failed error=%v", err)
				}
			}
			timer.Reset(time.Minute)
		}
	}
}

func (a *app) evaluateMetricAlerts(ctx context.Context, item server, state agentState) {
	cpuRatio := state.CPUPercent / float64(max(item.CPU*100, 1))
	memoryRatio := float64(state.MemoryBytes) / float64(max(item.MemoryMB*1024*1024, 1))
	diskRatio := float64(state.DiskBytes) / float64(max(item.DiskMB*1024*1024, 1))
	_ = a.store.setAlert(ctx, item.ID, "cpu_high", "warning", "CPU server tinggi", fmt.Sprintf("%s memakai %.0f%% dari alokasi CPU selama sampel terakhir.", item.Name, cpuRatio*100), cpuRatio >= .9)
	_ = a.store.setAlert(ctx, item.ID, "memory_high", "danger", "RAM server hampir penuh", fmt.Sprintf("%s memakai %.0f%% dari alokasi RAM.", item.Name, memoryRatio*100), memoryRatio >= .9)
	_ = a.store.setAlert(ctx, item.ID, "disk_high", "danger", "Disk server hampir penuh", fmt.Sprintf("%s memakai %.0f%% dari alokasi disk.", item.Name, diskRatio*100), diskRatio >= .9)
	unexpected := item.DesiredState == "running" && (state.State == "offline" || state.State == "error")
	_ = a.store.setAlert(ctx, item.ID, "unexpected_stop", "danger", "Server berhenti tak terduga", fmt.Sprintf("%s berhenti ketika status yang diinginkan masih berjalan.", item.Name), unexpected)
}
