package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type checkoutInput struct {
	PackageID      string `json:"packageId"`
	IdempotencyKey string `json:"idempotencyKey"`
	Name           string `json:"name"`
	Runtime        string `json:"runtime"`
	Version        string `json:"version"`
	JavaVersion    int    `json:"javaVersion"`
}

func scanPackage(row rowScanner) (hostingPackage, error) {
	var p hostingPackage
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.PriceIDR, &p.CPU, &p.MemoryMB, &p.DiskMB, &p.SortOrder, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

const packageColumns = `id,slug,name,description,price_idr,cpu,memory_mb,disk_mb,sort_order,active,created_at,updated_at`

func (s *store) listPackages(ctx context.Context, includeInactive bool) ([]hostingPackage, error) {
	rows, err := s.db.Query(ctx, `SELECT `+packageColumns+` FROM hosting_packages WHERE active OR $1 ORDER BY sort_order,name`, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]hostingPackage, 0)
	for rows.Next() {
		p, err := scanPackage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *store) packageByID(ctx context.Context, id string, activeOnly bool) (hostingPackage, error) {
	return scanPackage(s.db.QueryRow(ctx, `SELECT `+packageColumns+` FROM hosting_packages WHERE id=$1 AND (active OR NOT $2)`, id, activeOnly))
}

func validatePackage(p hostingPackage) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	if !usernamePattern.MatchString(p.Slug) || p.Name == "" || len(p.Name) > 64 || len(p.Description) > 500 || p.PriceIDR < 0 || p.CPU < 1 || p.CPU > 32 || p.MemoryMB < 1024 || p.MemoryMB > 131072 || p.DiskMB < 1024 || p.DiskMB > 1048576 {
		return errors.New("invalid hosting package")
	}
	return nil
}

func normalizePackage(p *hostingPackage) {
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	p.Description = strings.TrimSpace(p.Description)
}

func (s *store) savePackage(ctx context.Context, p hostingPackage) (hostingPackage, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
		return scanPackage(s.db.QueryRow(ctx, `INSERT INTO hosting_packages(id,slug,name,description,price_idr,cpu,memory_mb,disk_mb,sort_order,active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+packageColumns, p.ID, p.Slug, p.Name, p.Description, p.PriceIDR, p.CPU, p.MemoryMB, p.DiskMB, p.SortOrder, p.Active))
	}
	return scanPackage(s.db.QueryRow(ctx, `UPDATE hosting_packages SET slug=$2,name=$3,description=$4,price_idr=$5,cpu=$6,memory_mb=$7,disk_mb=$8,sort_order=$9,active=$10,updated_at=now() WHERE id=$1 RETURNING `+packageColumns, p.ID, p.Slug, p.Name, p.Description, p.PriceIDR, p.CPU, p.MemoryMB, p.DiskMB, p.SortOrder, p.Active))
}

func capacityAvailable(total, used capacitySummary) capacitySummary {
	return capacitySummary{CPU: max(0, total.CPU-used.CPU), MemoryMB: max(0, total.MemoryMB-used.MemoryMB), DiskMB: max(0, total.DiskMB-used.DiskMB), Ports: max(0, total.Ports-used.Ports)}
}
func (a *app) capacityTotal() capacitySummary {
	return capacitySummary{CPU: a.cfg.NodeCPUs, MemoryMB: a.cfg.NodeMemoryMB, DiskMB: a.cfg.NodeDiskMB, Ports: a.cfg.PortEnd - a.cfg.PortStart + 1}
}
func capacityFits(available capacitySummary, cpu, memory, disk int) bool {
	return available.CPU >= cpu && available.MemoryMB >= memory && available.DiskMB >= disk && available.Ports >= 1
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func capacityUsed(ctx context.Context, q queryRower) (capacitySummary, error) {
	var out capacitySummary
	err := q.QueryRow(ctx, `SELECT
COALESCE(sum(CASE WHEN sub.id IS NULL OR sub.status IN ('provisioning','active','action_required','grace') THEN s.cpu ELSE 0 END),0),
COALESCE(sum(CASE WHEN sub.id IS NULL OR sub.status IN ('provisioning','active','action_required','grace') THEN s.memory_mb ELSE 0 END),0),
COALESCE(sum(s.disk_mb),0),
(SELECT count(*) FROM allocations WHERE server_id IS NOT NULL)
FROM servers s LEFT JOIN subscriptions sub ON sub.server_id=s.id WHERE s.deleted_at IS NULL`).Scan(&out.CPU, &out.MemoryMB, &out.DiskMB, &out.Ports)
	return out, err
}

func (a *app) packages(w http.ResponseWriter, r *http.Request) {
	session, _ := currentSession(r.Context())
	includeInactive := session.Role == "owner" && r.URL.Query().Get("all") == "1"
	if r.Method == http.MethodGet {
		items, err := a.store.listPackages(r.Context(), includeInactive)
		if err != nil {
			internal(w, r, err)
			return
		}
		write(w, http.StatusOK, items)
		return
	}
	if session.Role != "owner" {
		forbidden(w, r)
		return
	}
	if r.Method == http.MethodPost {
		var p hostingPackage
		if decode(w, r, &p) != nil {
			return
		}
		p.ID = ""
		normalizePackage(&p)
		if err := validatePackage(p); err != nil {
			write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "invalid_package", RequestID: requestID(r.Context())})
			return
		}
		item, err := a.store.savePackage(r.Context(), p)
		if err != nil {
			write(w, http.StatusConflict, apiError{Error: "package slug is unavailable", Code: "package_conflict", RequestID: requestID(r.Context())})
			return
		}
		_ = a.store.audit(r.Context(), &session.UserID, "package.create", "package", item.ID, clientIP(r), map[string]any{})
		write(w, http.StatusCreated, item)
		return
	}
	method(w)
}

func (a *app) packageItem(w http.ResponseWriter, r *http.Request) {
	session, _ := currentSession(r.Context())
	if session.Role != "owner" {
		forbidden(w, r)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/packages/"), "/")
	if uuid.Validate(id) != nil {
		notFound(w, r)
		return
	}
	if r.Method == http.MethodPut {
		var p hostingPackage
		if decode(w, r, &p) != nil {
			return
		}
		p.ID = id
		normalizePackage(&p)
		if err := validatePackage(p); err != nil {
			write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "invalid_package", RequestID: requestID(r.Context())})
			return
		}
		item, err := a.store.savePackage(r.Context(), p)
		if err != nil {
			write(w, http.StatusConflict, apiError{Error: "package could not be updated", Code: "package_conflict", RequestID: requestID(r.Context())})
			return
		}
		_ = a.store.audit(r.Context(), &session.UserID, "package.update", "package", id, clientIP(r), map[string]any{})
		write(w, http.StatusOK, item)
		return
	}
	if r.Method == http.MethodDelete {
		ct, err := a.store.db.Exec(r.Context(), `UPDATE hosting_packages SET active=false,updated_at=now() WHERE id=$1`, id)
		if err != nil {
			internal(w, r, err)
			return
		}
		if ct.RowsAffected() == 0 {
			notFound(w, r)
			return
		}
		_ = a.store.audit(r.Context(), &session.UserID, "package.archive", "package", id, clientIP(r), map[string]any{})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	method(w)
}

func (a *app) capacity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	used, err := capacityUsed(r.Context(), a.store.db)
	if err != nil {
		internal(w, r, err)
		return
	}
	total := a.capacityTotal()
	available := capacityAvailable(total, used)
	packages, err := a.store.listPackages(r.Context(), false)
	if err != nil {
		internal(w, r, err)
		return
	}
	availability := map[string]bool{}
	for _, p := range packages {
		availability[p.ID] = capacityFits(available, p.CPU, p.MemoryMB, p.DiskMB)
	}
	session, _ := currentSession(r.Context())
	out := map[string]any{"available": available, "packageAvailability": availability}
	if session.Role == "owner" {
		out["total"] = total
		out["reserved"] = used
		if host, err := a.agent.nodeMetrics(r.Context()); err == nil {
			out["host"] = host
		} else {
			out["hostUnavailable"] = true
		}
	}
	write(w, http.StatusOK, out)
}

func validateCheckout(in *checkoutInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Runtime = strings.ToLower(strings.TrimSpace(in.Runtime))
	in.Version = strings.TrimSpace(in.Version)
	// Checkout is intentionally opinionated: derive Java from the selected
	// Minecraft version instead of trusting a client-supplied value.
	in.JavaVersion = requiredJavaVersion(in.Version)
	if uuid.Validate(in.PackageID) != nil || uuid.Validate(in.IdempotencyKey) != nil || in.Name == "" || len(in.Name) > 48 || !runtimeOK(in.Runtime) || !versionPattern.MatchString(in.Version) || !javaVersionOK(in.JavaVersion) {
		return errors.New("invalid checkout")
	}
	return nil
}

func scanOrder(row rowScanner) (order, error) {
	var o order
	var key string
	err := row.Scan(&o.ID, &o.UserID, &o.ServerID, &o.SubscriptionID, &o.PackageID, &o.Kind, &o.Status, &o.AmountIDR, &o.PackageName, &o.CPU, &o.MemoryMB, &o.DiskMB, &o.PaymentReference, &key, &o.PeriodStart, &o.PeriodEnd, &o.CreatedAt)
	return o, err
}

const orderColumns = `id,user_id,server_id,subscription_id,package_id,kind,status,amount_idr,package_name,cpu,memory_mb,disk_mb,payment_reference,idempotency_key,period_start,period_end,created_at`

func scanSubscription(row rowScanner) (subscription, error) {
	var s subscription
	err := row.Scan(&s.ID, &s.UserID, &s.ServerID, &s.PackageID, &s.PackageName, &s.PriceIDR, &s.CPU, &s.MemoryMB, &s.DiskMB, &s.Status, &s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.GraceEndsAt, &s.ResourceReleasedAt, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

const subscriptionColumns = `id,user_id,server_id,package_id,package_name,price_idr,cpu,memory_mb,disk_mb,status,current_period_start,current_period_end,grace_ends_at,resource_released_at,created_at,updated_at`

func (a *app) checkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	session, _ := currentSession(r.Context())
	if session.Role != "user" {
		forbidden(w, r)
		return
	}
	var in checkoutInput
	if decode(w, r, &in) != nil {
		return
	}
	if err := validateCheckout(&in); err != nil {
		write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "invalid_checkout", RequestID: requestID(r.Context())})
		return
	}
	srv, ord, sub, job, err := a.store.checkout(r.Context(), session, in, a.cfg, a.capacityTotal())
	if errors.Is(err, errCapacity) || errors.Is(err, errNoAllocation) {
		write(w, http.StatusConflict, apiError{Error: err.Error(), Code: "capacity_conflict", RequestID: requestID(r.Context())})
		return
	}
	if err != nil {
		internal(w, r, err)
		return
	}
	_ = a.store.audit(r.Context(), &session.UserID, "order.purchase", "order", ord.ID, clientIP(r), map[string]any{"serverId": srv.ID, "packageId": in.PackageID})
	write(w, http.StatusAccepted, map[string]any{"server": srv, "order": ord, "subscription": sub, "job": job})
}

func (s *store) checkout(ctx context.Context, session sessionRecord, in checkoutInput, cfg config, total capacitySummary) (server, order, subscription, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(74201)`); err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	if existing, e := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE user_id=$1 AND idempotency_key=$2`, session.UserID, in.IdempotencyKey)); e == nil {
		srv, e2 := s.get(ctx, existing.ServerID)
		if e2 != nil {
			return server{}, order{}, subscription{}, job{}, e2
		}
		sub, e2 := scanSubscription(tx.QueryRow(ctx, `SELECT `+subscriptionColumns+` FROM subscriptions WHERE id=$1`, existing.SubscriptionID))
		return srv, existing, sub, job{}, e2
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return server{}, order{}, subscription{}, job{}, e
	}
	p, err := scanPackage(tx.QueryRow(ctx, `SELECT `+packageColumns+` FROM hosting_packages WHERE id=$1 AND active FOR SHARE`, in.PackageID))
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	used, err := capacityUsed(ctx, tx)
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	if !capacityFits(capacityAvailable(total, used), p.CPU, p.MemoryMB, p.DiskMB) {
		return server{}, order{}, subscription{}, job{}, errCapacity
	}
	var port int
	if err = tx.QueryRow(ctx, `SELECT candidate FROM generate_series($1::integer,$2::integer) candidate WHERE NOT EXISTS(SELECT 1 FROM allocations WHERE node_id=$3 AND bind_ip=$4 AND port=candidate) ORDER BY candidate LIMIT 1`, cfg.PortStart, cfg.PortEnd, defaultNodeID, cfg.BindIP).Scan(&port); err != nil {
		return server{}, order{}, subscription{}, job{}, errNoAllocation
	}
	now := time.Now().UTC()
	end := now.Add(30 * 24 * time.Hour)
	srv := server{ID: uuid.NewString(), NodeID: defaultNodeID, OwnerUserID: session.UserID, OwnerUsername: session.Username, Name: in.Name, Runtime: in.Runtime, Version: in.Version, JavaVersion: in.JavaVersion, MemoryMB: p.MemoryMB, CPU: p.CPU, DiskMB: p.DiskMB, BindIP: cfg.BindIP, Port: port, DesiredState: "offline", State: "installing", Config: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO servers(id,node_id,owner_user_id,name,runtime,version,java_version,memory_mb,cpu,disk_mb,bind_ip,port,desired_state,observed_state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'offline','installing')`, srv.ID, srv.NodeID, srv.OwnerUserID, srv.Name, srv.Runtime, srv.Version, srv.JavaVersion, srv.MemoryMB, srv.CPU, srv.DiskMB, srv.BindIP, srv.Port)
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO allocations(id,node_id,server_id,bind_ip,port) VALUES($1,$2,$3,$4,$5)`, uuid.NewString(), srv.NodeID, srv.ID, srv.BindIP, srv.Port)
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	sub := subscription{ID: uuid.NewString(), UserID: session.UserID, ServerID: srv.ID, PackageID: &p.ID, PackageName: p.Name, PriceIDR: p.PriceIDR, CPU: p.CPU, MemoryMB: p.MemoryMB, DiskMB: p.DiskMB, Status: "provisioning", CurrentPeriodStart: now, CurrentPeriodEnd: end, GraceEndsAt: end.Add(7 * 24 * time.Hour), CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO subscriptions(id,user_id,server_id,package_id,package_name,price_idr,cpu,memory_mb,disk_mb,status,current_period_start,current_period_end,grace_ends_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, sub.ID, sub.UserID, sub.ServerID, p.ID, sub.PackageName, sub.PriceIDR, sub.CPU, sub.MemoryMB, sub.DiskMB, sub.Status, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.GraceEndsAt)
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	ord := order{ID: uuid.NewString(), UserID: session.UserID, ServerID: srv.ID, SubscriptionID: sub.ID, PackageID: &p.ID, Kind: "purchase", Status: "paid", AmountIDR: p.PriceIDR, PackageName: p.Name, CPU: p.CPU, MemoryMB: p.MemoryMB, DiskMB: p.DiskMB, PaymentReference: "SIM-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:16]), PeriodStart: now, PeriodEnd: end, CreatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO orders(id,user_id,server_id,subscription_id,package_id,kind,status,amount_idr,package_name,cpu,memory_mb,disk_mb,payment_reference,idempotency_key,period_start,period_end) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, ord.ID, ord.UserID, ord.ServerID, ord.SubscriptionID, p.ID, ord.Kind, ord.Status, ord.AmountIDR, ord.PackageName, ord.CPU, ord.MemoryMB, ord.DiskMB, ord.PaymentReference, in.IdempotencyKey, ord.PeriodStart, ord.PeriodEnd)
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	j, err := insertJob(ctx, tx, srv.ID, "provision", map[string]any{"source": "checkout", "subscriptionId": sub.ID})
	if err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return server{}, order{}, subscription{}, job{}, err
	}
	srv.CurrentJob = &j
	return srv, ord, sub, j, nil
}

func (a *app) orderCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	session, _ := currentSession(r.Context())
	query := `SELECT ` + orderColumns + ` FROM orders`
	args := []any{}
	if session.Role != "owner" {
		query += ` WHERE user_id=$1`
		args = append(args, session.UserID)
	}
	query += ` ORDER BY created_at DESC LIMIT 200`
	rows, err := a.store.db.Query(r.Context(), query, args...)
	if err != nil {
		internal(w, r, err)
		return
	}
	defer rows.Close()
	out := make([]order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			internal(w, r, err)
			return
		}
		out = append(out, o)
	}
	write(w, http.StatusOK, out)
}

func (a *app) subscriptionCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	session, _ := currentSession(r.Context())
	query := `SELECT ` + subscriptionColumns + ` FROM subscriptions`
	args := []any{}
	if session.Role != "owner" {
		query += ` WHERE user_id=$1`
		args = append(args, session.UserID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := a.store.db.Query(r.Context(), query, args...)
	if err != nil {
		internal(w, r, err)
		return
	}
	defer rows.Close()
	out := make([]subscription, 0)
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			internal(w, r, err)
			return
		}
		out = append(out, s)
	}
	write(w, http.StatusOK, out)
}

func (a *app) subscriptionItem(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/subscriptions/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || uuid.Validate(parts[0]) != nil || r.Method != http.MethodPost {
		notFound(w, r)
		return
	}
	if parts[1] == "retry" {
		a.retrySubscription(w, r, parts[0])
		return
	}
	if parts[1] != "renew" {
		notFound(w, r)
		return
	}
	var input struct {
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	if uuid.Validate(input.IdempotencyKey) != nil {
		write(w, http.StatusBadRequest, apiError{Error: "invalid idempotency key", Code: "invalid_checkout", RequestID: requestID(r.Context())})
		return
	}
	session, _ := currentSession(r.Context())
	ord, sub, j, err := a.store.renew(r.Context(), session, parts[0], input.IdempotencyKey, a.cfg, a.capacityTotal())
	if errors.Is(err, errCapacity) || errors.Is(err, errNoAllocation) {
		write(w, http.StatusConflict, apiError{Error: err.Error(), Code: "capacity_conflict", RequestID: requestID(r.Context())})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w, r)
		return
	}
	if err != nil {
		internal(w, r, err)
		return
	}
	_ = a.store.audit(r.Context(), &session.UserID, "order.renewal", "order", ord.ID, clientIP(r), map[string]any{"serverId": sub.ServerID})
	write(w, http.StatusAccepted, map[string]any{"order": ord, "subscription": sub, "job": j})
}

func (a *app) retrySubscription(w http.ResponseWriter, r *http.Request, id string) {
	session, _ := currentSession(r.Context())
	j, serverID, err := a.store.retrySubscription(r.Context(), session, id)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w, r)
		return
	}
	if err != nil {
		write(w, http.StatusConflict, apiError{Error: "another operation is already active", Code: "job_conflict", RequestID: requestID(r.Context())})
		return
	}
	_ = a.store.audit(r.Context(), &session.UserID, "subscription.retry", "subscription", id, clientIP(r), map[string]any{"serverId": serverID})
	write(w, http.StatusAccepted, map[string]any{"job": j})
}

func (s *store) retrySubscription(ctx context.Context, session sessionRecord, id string) (job, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return job{}, "", err
	}
	defer tx.Rollback(ctx)
	query := `UPDATE subscriptions SET status='provisioning',updated_at=now() WHERE id=$1 AND status='action_required'`
	args := []any{id}
	if session.Role != "owner" {
		query += ` AND user_id=$2`
		args = append(args, session.UserID)
	}
	query += ` RETURNING server_id`
	var serverID string
	if err = tx.QueryRow(ctx, query, args...).Scan(&serverID); err != nil {
		return job{}, "", err
	}
	j, err := insertJob(ctx, tx, serverID, "provision", map[string]any{"source": "subscription_retry", "subscriptionId": id})
	if err != nil {
		return job{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return job{}, "", err
	}
	return j, serverID, nil
}

func (s *store) renew(ctx context.Context, session sessionRecord, subscriptionID, idempotency string, cfg config, total capacitySummary) (order, subscription, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return order{}, subscription{}, job{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(74201)`); err != nil {
		return order{}, subscription{}, job{}, err
	}
	query := `SELECT ` + subscriptionColumns + ` FROM subscriptions WHERE id=$1`
	args := []any{subscriptionID}
	if session.Role != "owner" {
		query += ` AND user_id=$2`
		args = append(args, session.UserID)
	}
	query += ` FOR UPDATE`
	sub, err := scanSubscription(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return order{}, subscription{}, job{}, err
	}
	if existing, e := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE user_id=$1 AND idempotency_key=$2`, sub.UserID, idempotency)); e == nil {
		if existing.SubscriptionID != sub.ID {
			return order{}, subscription{}, job{}, errors.New("idempotency key belongs to another subscription")
		}
		return existing, sub, job{}, nil
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return order{}, subscription{}, job{}, e
	}
	if sub.Status != "active" && sub.Status != "grace" && sub.Status != "released" {
		return order{}, subscription{}, job{}, pgx.ErrNoRows
	}
	now := time.Now().UTC()
	start := sub.CurrentPeriodEnd
	if start.Before(now) {
		start = now
	}
	end := start.Add(30 * 24 * time.Hour)
	var j job
	if sub.Status == "released" {
		used, err := capacityUsed(ctx, tx)
		if err != nil {
			return order{}, subscription{}, job{}, err
		}
		available := capacityAvailable(total, used)
		if available.CPU < sub.CPU || available.MemoryMB < sub.MemoryMB || available.Ports < 1 {
			return order{}, subscription{}, job{}, errCapacity
		}
		var port int
		if err = tx.QueryRow(ctx, `SELECT candidate FROM generate_series($1::integer,$2::integer) candidate WHERE NOT EXISTS(SELECT 1 FROM allocations WHERE node_id=$3 AND bind_ip=$4 AND port=candidate) ORDER BY candidate LIMIT 1`, cfg.PortStart, cfg.PortEnd, defaultNodeID, cfg.BindIP).Scan(&port); err != nil {
			return order{}, subscription{}, job{}, errNoAllocation
		}
		if _, err = tx.Exec(ctx, `INSERT INTO allocations(id,node_id,server_id,bind_ip,port) VALUES($1,$2,$3,$4,$5)`, uuid.NewString(), defaultNodeID, sub.ServerID, cfg.BindIP, port); err != nil {
			return order{}, subscription{}, job{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE servers SET port=$2,observed_state='installing',updated_at=now() WHERE id=$1`, sub.ServerID, port); err != nil {
			return order{}, subscription{}, job{}, err
		}
		j, err = insertJob(ctx, tx, sub.ServerID, "provision", map[string]any{"source": "subscription_renewal", "subscriptionId": sub.ID})
		if err != nil {
			return order{}, subscription{}, job{}, err
		}
	}
	sub.Status = "active"
	if j.ID != "" {
		sub.Status = "provisioning"
	}
	sub.CurrentPeriodStart = start
	sub.CurrentPeriodEnd = end
	sub.GraceEndsAt = end.Add(7 * 24 * time.Hour)
	sub.ResourceReleasedAt = nil
	sub.UpdatedAt = now
	if _, err = tx.Exec(ctx, `UPDATE subscriptions SET status=$2,current_period_start=$3,current_period_end=$4,grace_ends_at=$5,resource_released_at=NULL,updated_at=now() WHERE id=$1`, sub.ID, sub.Status, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.GraceEndsAt); err != nil {
		return order{}, subscription{}, job{}, err
	}
	ord := order{ID: uuid.NewString(), UserID: sub.UserID, ServerID: sub.ServerID, SubscriptionID: sub.ID, PackageID: sub.PackageID, Kind: "renewal", Status: "paid", AmountIDR: sub.PriceIDR, PackageName: sub.PackageName, CPU: sub.CPU, MemoryMB: sub.MemoryMB, DiskMB: sub.DiskMB, PaymentReference: "SIM-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:16]), PeriodStart: start, PeriodEnd: end, CreatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO orders(id,user_id,server_id,subscription_id,package_id,kind,status,amount_idr,package_name,cpu,memory_mb,disk_mb,payment_reference,idempotency_key,period_start,period_end) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, ord.ID, ord.UserID, ord.ServerID, ord.SubscriptionID, ord.PackageID, ord.Kind, ord.Status, ord.AmountIDR, ord.PackageName, ord.CPU, ord.MemoryMB, ord.DiskMB, ord.PaymentReference, idempotency, ord.PeriodStart, ord.PeriodEnd)
	if err != nil {
		return order{}, subscription{}, job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return order{}, subscription{}, job{}, err
	}
	return ord, sub, j, nil
}

func (s *store) setSubscriptionProvisionResult(ctx context.Context, serverID string, jobErr error) error {
	status := "active"
	orderStatus := "paid"
	if jobErr != nil {
		status = "action_required"
		orderStatus = "action_required"
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE subscriptions SET status=$2,updated_at=now() WHERE server_id=$1 AND status='provisioning'`, serverID, status); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE orders SET status=$2 WHERE id=(SELECT id FROM orders WHERE server_id=$1 ORDER BY created_at DESC LIMIT 1)`, serverID, orderStatus); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) subscriptionStatus(ctx context.Context, serverID string) (string, error) {
	var status string
	err := s.db.QueryRow(ctx, `SELECT CASE
WHEN status IN ('active','provisioning','action_required') AND current_period_end<=now() THEN 'grace'
ELSE status END FROM subscriptions WHERE server_id=$1`, serverID).Scan(&status)
	return status, err
}

func (a *app) transferServer(w http.ResponseWriter, r *http.Request) {
	session, _ := currentSession(r.Context())
	if session.Role != "owner" {
		forbidden(w, r)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/servers/"), "/")
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[1] != "owner" || uuid.Validate(parts[0]) != nil || r.Method != http.MethodPut {
		notFound(w, r)
		return
	}
	var input struct {
		UserID string `json:"userId"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	target, err := a.store.findUserByID(r.Context(), input.UserID)
	if err != nil || target.Status != "active" || target.Role != "user" {
		write(w, http.StatusBadRequest, apiError{Error: "target user is unavailable", Code: "invalid_owner", RequestID: requestID(r.Context())})
		return
	}
	tx, err := a.store.db.Begin(r.Context())
	if err != nil {
		internal(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	ct, err := tx.Exec(r.Context(), `UPDATE servers SET owner_user_id=$2,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL AND EXISTS(SELECT 1 FROM subscriptions WHERE server_id=servers.id)`, parts[0], target.ID)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE subscriptions SET user_id=$2,updated_at=now() WHERE server_id=$1`, parts[0], target.ID)
	}
	if err != nil || ct.RowsAffected() == 0 {
		notFound(w, r)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		internal(w, r, err)
		return
	}
	_ = a.store.audit(r.Context(), &session.UserID, "server.owner.transfer", "server", parts[0], clientIP(r), map[string]any{"userId": target.ID})
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) runSubscriptionWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.processSubscriptionExpiry(ctx); err != nil {
				log.Printf("subscription expiry failed: %v", err)
			}
		}
	}
}
func (a *app) processSubscriptionExpiry(ctx context.Context) error {
	rows, err := a.store.db.Query(ctx, `WITH expired AS (
  UPDATE subscriptions SET status='grace',updated_at=now()
  WHERE status IN('active','provisioning','action_required') AND current_period_end<=now()
  RETURNING server_id
)
UPDATE servers SET desired_state='offline',updated_at=now()
WHERE id IN (SELECT server_id FROM expired) RETURNING id`)
	if err != nil {
		return err
	}
	var graceIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		graceIDs = append(graceIDs, id)
	}
	rows.Close()
	for _, id := range graceIDs {
		item, err := a.store.get(ctx, id)
		if err == nil && item.CurrentJob == nil {
			_, _ = a.store.createLifecycleJob(ctx, id, "stop", "offline", map[string]any{"source": "subscription_expiry"})
		}
	}
	rows, err = a.store.db.Query(ctx, `SELECT server_id FROM subscriptions WHERE status='grace' AND grace_ends_at<=now() AND resource_released_at IS NULL`)
	if err != nil {
		return err
	}
	var releaseIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		releaseIDs = append(releaseIDs, id)
	}
	rows.Close()
	for _, id := range releaseIDs {
		item, err := a.store.get(ctx, id)
		if err == nil && item.CurrentJob == nil {
			_, _ = a.store.createJob(ctx, id, "release", map[string]any{"source": "subscription_expiry"})
		}
	}
	return nil
}

func (s *store) finalizeRelease(ctx context.Context, id string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM allocations WHERE server_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE subscriptions SET status='released',resource_released_at=now(),updated_at=now() WHERE server_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE servers SET desired_state='offline',observed_state='offline',updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
