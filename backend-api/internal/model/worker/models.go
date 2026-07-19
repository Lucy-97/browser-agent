package worker

import "time"

type Device struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	UserID        string         `json:"user_id,omitempty"`
	Name          string         `json:"name"`
	Platform      string         `json:"platform"`
	WorkerVersion string         `json:"worker_version"`
	Token         string         `json:"-"`
	Status        string         `json:"status"`
	Capabilities  []string       `json:"capabilities"`
	LastHeartbeat *time.Time     `json:"last_heartbeat,omitempty"`
	Metrics       map[string]any `json:"metrics,omitempty"`
	Revoked       bool           `json:"revoked"`
}

type Pairing struct {
	ID                  string    `json:"pairing_id"`
	TenantID            string    `json:"tenant_id,omitempty"`
	ApprovedByUserID    string    `json:"approved_by_user_id,omitempty"`
	Code                string    `json:"pairing_code"`
	VerificationURI     string    `json:"verification_uri"`
	ExpiresAt           time.Time `json:"expires_at"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
	Status              string    `json:"status"`
	DeviceID            string    `json:"device_id,omitempty"`
	Device              *Device   `json:"device,omitempty"`
	DeviceToken         string    `json:"device_token,omitempty"`
	DisplayName         string    `json:"display_name,omitempty"`
	Platform            string    `json:"platform,omitempty"`
	WorkerVersion       string    `json:"worker_version,omitempty"`
}

type PairingRequest struct {
	WorkerVersion string `json:"worker_version"`
	Platform      string `json:"platform"`
	HostnameHash  string `json:"hostname_hash"`
	DisplayName   string `json:"display_name"`
}

type HeartbeatRequest struct {
	WorkerVersion string         `json:"worker_version"`
	Status        string         `json:"status"`
	CurrentJobID  string         `json:"current_job_id"`
	CurrentRunID  string         `json:"current_run_id"`
	Capabilities  []string       `json:"capabilities"`
	Metrics       map[string]any `json:"metrics"`
}

type ListDevicesOptions struct {
	TenantID string
	Status   string
	Limit    int
	Offset   int
}
