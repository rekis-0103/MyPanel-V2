package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

type app struct {
	cfg      config
	store    *store
	sessions *sessionStore
	agent    *agentClient
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://127.0.0.1:8080/api/v1/health/live")
		if err != nil || response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		response.Body.Close()
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st, err := openStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer st.db.Close()

	if len(os.Args) > 1 && os.Args[1] == "bootstrap-owner" {
		if err := bootstrapOwner(ctx, st, cfg); err != nil {
			log.Fatalf("bootstrap owner: %v", err)
		}
		log.Printf("owner account created")
		return
	}
	count, err := st.userCount(ctx)
	if err != nil {
		log.Fatalf("count users: %v", err)
	}
	if count == 0 {
		log.Fatal("no owner exists; run /controller bootstrap-owner with ADMIN_PASSWORD_FILE")
	}
	if err := st.recoverJobs(ctx); err != nil {
		log.Fatalf("recover interrupted jobs: %v", err)
	}
	sessions, err := openSessionStore(ctx, cfg)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer sessions.redis.Close()
	agent, err := newAgentClient(cfg)
	if err != nil {
		log.Fatalf("configure agent client: %v", err)
	}
	a := &app{cfg: cfg, store: st, sessions: sessions, agent: agent}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go a.runJobWorker(workerCtx)
	go a.runReconciler(workerCtx)
	go a.runScheduler(workerCtx)
	go a.runSubscriptionWorker(workerCtx)
	go a.runMetricsCollector(workerCtx)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware(a.routes()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	log.Printf("controller listening addr=%s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func bootstrapOwner(ctx context.Context, st *store, cfg config) error {
	count, err := st.userCount(ctx)
	if err != nil {
		return err
	}
	if count != 0 {
		log.Printf("owner already exists; bootstrap skipped")
		return nil
	}
	hash, err := hashPassword(cfg.BootstrapPassword)
	if err != nil {
		return err
	}
	return st.createOwner(ctx, cfg.BootstrapUsername, hash)
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health/live", a.live)
	mux.HandleFunc("/api/v1/health/ready", a.ready)
	mux.HandleFunc("/api/v1/health", a.live)
	mux.HandleFunc("/api/v1/auth/login", a.login)
	mux.HandleFunc("/api/v1/auth/register", a.register)
	mux.HandleFunc("/api/v1/auth/logout", a.auth(a.logout))
	mux.HandleFunc("/api/v1/auth/me", a.auth(a.me))
	mux.HandleFunc("/api/v1/auth/change-password", a.auth(a.changePassword))
	mux.HandleFunc("/api/v1/admin/users", a.auth(a.adminUsers))
	mux.HandleFunc("/api/v1/admin/users/", a.auth(a.adminUsers))
	mux.HandleFunc("/api/v1/catalog", a.auth(a.catalog))
	mux.HandleFunc("/api/v1/packages", a.auth(a.packages))
	mux.HandleFunc("/api/v1/packages/", a.auth(a.packageItem))
	mux.HandleFunc("/api/v1/capacity", a.auth(a.capacity))
	mux.HandleFunc("/api/v1/metrics/servers", a.auth(a.serverMetricsCollection))
	mux.HandleFunc("/api/v1/notifications", a.auth(a.notificationCollection))
	mux.HandleFunc("/api/v1/notifications/", a.auth(a.notificationItem))
	mux.HandleFunc("/api/v1/orders", a.auth(a.orderCollection))
	mux.HandleFunc("/api/v1/checkout", a.auth(a.checkout))
	mux.HandleFunc("/api/v1/subscriptions", a.auth(a.subscriptionCollection))
	mux.HandleFunc("/api/v1/subscriptions/", a.auth(a.subscriptionItem))
	mux.HandleFunc("/api/v1/admin/servers/", a.auth(a.transferServer))
	mux.HandleFunc("/api/v1/servers", a.auth(a.serverCollection))
	mux.HandleFunc("/api/v1/servers/", a.auth(a.serverItem))
	mux.HandleFunc("/api/v1/jobs/", a.auth(a.jobItem))
	mux.HandleFunc("/api/v1/audit", a.auth(a.auditCollection))
	return mux
}

func (a *app) live(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"database": "ok", "redis": "ok", "agent": "ok"}
	status := http.StatusOK
	if err := a.store.db.Ping(ctx); err != nil {
		checks["database"] = "unavailable"
		status = http.StatusServiceUnavailable
	}
	if err := a.sessions.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unavailable"
		status = http.StatusServiceUnavailable
	}
	if err := a.agent.health(ctx); err != nil {
		checks["agent"] = "unavailable"
		status = http.StatusServiceUnavailable
	}
	write(w, status, map[string]any{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "checks": checks})
}
