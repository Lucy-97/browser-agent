package automation

import "time"

type Job struct {
	ID        string         `json:"job_id"`
	TenantID  string         `json:"tenant_id"`
	UserID    string         `json:"created_by_user_id,omitempty"`
	Type      string         `json:"job_type"`
	Adapter   string         `json:"adapter"`
	Target    map[string]any `json:"target"`
	Input     map[string]any `json:"input"`
	Policy    map[string]any `json:"policy"`
	Cursor    map[string]any `json:"cursor,omitempty"`
	Status    string         `json:"status"`
	Priority  int            `json:"priority"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Run struct {
	ID              string         `json:"run_id"`
	JobID           string         `json:"job_id"`
	TenantID        string         `json:"tenant_id"`
	UserID          string         `json:"created_by_user_id,omitempty"`
	DeviceID        string         `json:"device_id"`
	Status          string         `json:"status"`
	CurrentStep     string         `json:"current_step,omitempty"`
	LastCursor      map[string]any `json:"last_cursor,omitempty"`
	Summary         map[string]any `json:"summary,omitempty"`
	Error           map[string]any `json:"error,omitempty"`
	LastHeartbeatAt *time.Time     `json:"last_heartbeat_at,omitempty"`
	StartedAt       time.Time      `json:"started_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
}

type JobEnvelope struct {
	JobID    string         `json:"job_id"`
	RunID    string         `json:"run_id"`
	TenantID string         `json:"-"`
	UserID   string         `json:"-"`
	JobType  string         `json:"job_type"`
	Adapter  string         `json:"adapter"`
	Target   map[string]any `json:"target"`
	Input    map[string]any `json:"input"`
	Policy   map[string]any `json:"policy"`
	Cursor   map[string]any `json:"cursor,omitempty"`
}

type CreateJobRequest struct {
	TenantID string         `json:"-"`
	UserID   string         `json:"-"`
	JobType  string         `json:"job_type"`
	Adapter  string         `json:"adapter"`
	Target   map[string]any `json:"target"`
	Input    map[string]any `json:"input"`
	Policy   map[string]any `json:"policy"`
	Priority int            `json:"priority"`
}

type HeartbeatRequest struct {
	Status      string         `json:"status"`
	CurrentStep string         `json:"current_step"`
	Cursor      map[string]any `json:"cursor"`
	Message     string         `json:"message"`
}

type Checkpoint struct {
	ID        string         `json:"checkpoint_id"`
	RunID     string         `json:"run_id"`
	JobID     string         `json:"job_id"`
	Cursor    map[string]any `json:"cursor,omitempty"`
	Summary   map[string]any `json:"summary,omitempty"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
}

type PolicyTemplate struct {
	Name        string         `json:"name"`
	ProductLine string         `json:"product_line"`
	JobType     string         `json:"job_type"`
	Adapter     string         `json:"adapter"`
	Target      map[string]any `json:"target"`
	Policy      map[string]any `json:"policy"`
}

type Artifact struct {
	ID           string         `json:"artifact_id"`
	RunID        string         `json:"run_id"`
	TenantID     string         `json:"tenant_id"`
	ArtifactType string         `json:"artifact_type"`
	StorageKey   string         `json:"-"`
	Filename     string         `json:"filename,omitempty"`
	ContentType  string         `json:"content_type,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	SHA256       string         `json:"sha256,omitempty"`
	SizeBytes    *int64         `json:"size_bytes,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type ManualAction struct {
	ID         string         `json:"manual_action_id"`
	RunID      string         `json:"run_id"`
	TenantID   string         `json:"tenant_id"`
	ActionType string         `json:"action_type"`
	Message    string         `json:"message"`
	Payload    map[string]any `json:"payload"`
	Status     string         `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	ResolvedAt *time.Time     `json:"resolved_at,omitempty"`
}

type CompleteRunRequest struct {
	JobID      string         `json:"job_id"`
	Status     string         `json:"status"`
	Summary    map[string]any `json:"summary"`
	LastCursor map[string]any `json:"last_cursor"`
	Error      map[string]any `json:"error"`
}

type ListJobsOptions struct {
	TenantID string
	Status   string
	Adapter  string
	Limit    int
	Offset   int
}

type ListRunsOptions struct {
	TenantID string
	Status   string
	JobID    string
	DeviceID string
	Limit    int
	Offset   int
}

type ListManualActionsOptions struct {
	TenantID string
	Status   string
	RunID    string
	Limit    int
	Offset   int
}

type ResolveManualActionRequest struct {
	Status  string         `json:"status"`
	Payload map[string]any `json:"payload"`
}
