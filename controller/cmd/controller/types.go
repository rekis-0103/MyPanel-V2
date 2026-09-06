package main

import (
	"encoding/json"
	"time"
)

const defaultNodeID = "00000000-0000-0000-0000-000000000001"

type server struct {
	ID                 string          `json:"id"`
	NodeID             string          `json:"nodeId"`
	OwnerUserID        string          `json:"ownerUserId"`
	OwnerUsername      string          `json:"ownerUsername"`
	SubscriptionStatus *string         `json:"subscriptionStatus"`
	SubscriptionEndsAt *time.Time      `json:"subscriptionEndsAt"`
	Name               string          `json:"name"`
	Runtime            string          `json:"runtime"`
	Version            string          `json:"version"`
	JavaVersion        int             `json:"javaVersion"`
	MemoryMB           int             `json:"memoryMb"`
	CPU                int             `json:"cpu"`
	DiskMB             int             `json:"diskMb"`
	BindIP             string          `json:"bindIp"`
	Port               int             `json:"port"`
	DesiredState       string          `json:"desiredState"`
	State              string          `json:"state"`
	Config             json.RawMessage `json:"config"`
	LastError          *string         `json:"lastError"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	CurrentJob         *job            `json:"currentJob,omitempty"`
}

type createServerInput struct {
	Name        string `json:"name"`
	Runtime     string `json:"runtime"`
	Version     string `json:"version"`
	JavaVersion int    `json:"javaVersion"`
	MemoryMB    int    `json:"memoryMb"`
	CPU         int    `json:"cpu"`
	DiskMB      int    `json:"diskMb"`
}

type job struct {
	ID          string          `json:"id"`
	NodeID      string          `json:"nodeId"`
	ServerID    string          `json:"serverId"`
	Action      string          `json:"action"`
	Status      string          `json:"status"`
	Payload     json.RawMessage `json:"payload"`
	Result      json.RawMessage `json:"result"`
	Error       *string         `json:"error"`
	Attempts    int             `json:"attempts"`
	CreatedAt   time.Time       `json:"createdAt"`
	StartedAt   *time.Time      `json:"startedAt"`
	CompletedAt *time.Time      `json:"completedAt"`
}

type backup struct {
	ID             string     `json:"id"`
	ServerID       string     `json:"serverId"`
	Name           string     `json:"name"`
	SizeBytes      int64      `json:"sizeBytes"`
	ChecksumSHA256 *string    `json:"checksumSha256"`
	Status         string     `json:"status"`
	Error          *string    `json:"error"`
	CreatedAt      time.Time  `json:"createdAt"`
	CompletedAt    *time.Time `json:"completedAt"`
}

type schedule struct {
	ID              string          `json:"id"`
	ServerID        string          `json:"serverId"`
	Name            string          `json:"name"`
	Action          string          `json:"action"`
	Payload         json.RawMessage `json:"payload"`
	IntervalMinutes int             `json:"intervalMinutes"`
	Enabled         bool            `json:"enabled"`
	NextRunAt       time.Time       `json:"nextRunAt"`
	LastRunAt       *time.Time      `json:"lastRunAt"`
	CreatedAt       time.Time       `json:"createdAt"`
}

type userRecord struct {
	ID                 string
	Username           string
	PasswordHash       string
	Role               string
	Status             string
	SessionVersion     int
	MustChangePassword bool
}

type sessionRecord struct {
	UserID             string `json:"userId"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	SessionVersion     int    `json:"sessionVersion"`
	MustChangePassword bool   `json:"mustChangePassword"`
	CSRFToken          string `json:"csrfToken"`
}

type publicUser struct {
	ID                 string    `json:"id"`
	Username           string    `json:"username"`
	Role               string    `json:"role"`
	Status             string    `json:"status"`
	MustChangePassword bool      `json:"mustChangePassword"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type hostingPackage struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	PriceIDR      int64     `json:"priceIdr"`
	CPU           int       `json:"cpu"`
	MemoryMB      int       `json:"memoryMb"`
	DiskMB        int       `json:"diskMb"`
	SortOrder     int       `json:"sortOrder"`
	Active        bool      `json:"active"`
	ThemeColor    string    `json:"themeColor"`
	Icon          string    `json:"icon"`
	IsPopular     bool      `json:"isPopular"`
	IsRecommended bool      `json:"isRecommended"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type subscription struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"userId"`
	ServerID           string     `json:"serverId"`
	PackageID          *string    `json:"packageId"`
	PackageName        string     `json:"packageName"`
	PriceIDR           int64      `json:"priceIdr"`
	CPU                int        `json:"cpu"`
	MemoryMB           int        `json:"memoryMb"`
	DiskMB             int        `json:"diskMb"`
	Status             string     `json:"status"`
	CurrentPeriodStart time.Time  `json:"currentPeriodStart"`
	CurrentPeriodEnd   time.Time  `json:"currentPeriodEnd"`
	GraceEndsAt        time.Time  `json:"graceEndsAt"`
	ResourceReleasedAt *time.Time `json:"resourceReleasedAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type order struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	ServerID         string    `json:"serverId"`
	SubscriptionID   string    `json:"subscriptionId"`
	PackageID        *string   `json:"packageId"`
	Kind             string    `json:"kind"`
	Status           string    `json:"status"`
	AmountIDR        int64     `json:"amountIdr"`
	PackageName      string    `json:"packageName"`
	CPU              int       `json:"cpu"`
	MemoryMB         int       `json:"memoryMb"`
	DiskMB           int       `json:"diskMb"`
	PaymentReference string    `json:"paymentReference"`
	PeriodStart      time.Time `json:"periodStart"`
	PeriodEnd        time.Time `json:"periodEnd"`
	CreatedAt        time.Time `json:"createdAt"`
}

type capacitySummary struct {
	CPU      int `json:"cpu"`
	MemoryMB int `json:"memoryMb"`
	DiskMB   int `json:"diskMb"`
	Ports    int `json:"ports"`
}

type agentServerSpec struct {
	ID          string          `json:"id"`
	Runtime     string          `json:"runtime"`
	Version     string          `json:"version"`
	JavaVersion int             `json:"javaVersion"`
	MemoryMB    int             `json:"memoryMb"`
	CPU         int             `json:"cpu"`
	DiskMB      int             `json:"diskMb"`
	BindIP      string          `json:"bindIp"`
	Port        int             `json:"port"`
	Config      json.RawMessage `json:"config"`
}

type agentState struct {
	State       string  `json:"state"`
	Reason      string  `json:"reason,omitempty"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes int64   `json:"memoryBytes"`
	DiskBytes   int64   `json:"diskBytes"`
	Players     int     `json:"players"`
}

type consoleEvent struct {
	ID        int64     `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type apiError struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}
