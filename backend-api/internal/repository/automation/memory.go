package automation

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"

	automationmodel "qiyuan/backend-api/internal/model/automation"
	workermodel "qiyuan/backend-api/internal/model/worker"
)

var (
	ErrJobNotFound          = errors.New("automation job not found")
	ErrRunNotFound          = errors.New("automation run not found")
	ErrArtifactNotFound     = errors.New("automation artifact not found")
	ErrManualActionNotFound = errors.New("manual action not found")
	ErrNoJobAvailable       = errors.New("no automation job available")
	ErrActiveRunExists      = errors.New("active run already exists")
)

type MemoryRepository struct {
	mu             sync.Mutex
	jobs           map[string]*automationmodel.Job
	runs           map[string]*automationmodel.Run
	activeRunByJob map[string]string
	checkpoints    map[string][]automationmodel.Checkpoint
	artifacts      map[string][]automationmodel.Artifact
	manualActions  map[string]*automationmodel.ManualAction
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		jobs:           map[string]*automationmodel.Job{},
		runs:           map[string]*automationmodel.Run{},
		activeRunByJob: map[string]string{},
		checkpoints:    map[string][]automationmodel.Checkpoint{},
		artifacts:      map[string][]automationmodel.Artifact{},
		manualActions:  map[string]*automationmodel.ManualAction{},
	}
}

func (repo *MemoryRepository) CreateJob(req automationmodel.CreateJobRequest) automationmodel.Job {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	now := time.Now().UTC()
	job := automationmodel.Job{
		ID:        newID("job"),
		Type:      req.JobType,
		Adapter:   req.Adapter,
		Target:    mapOrEmpty(req.Target),
		Input:     mapOrEmpty(req.Input),
		Policy:    mapOrEmpty(req.Policy),
		Status:    "queued",
		Priority:  req.Priority,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.jobs[job.ID] = &job
	return job
}

func (repo *MemoryRepository) Job(jobID string) (automationmodel.Job, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	job, ok := repo.jobs[jobID]
	if !ok {
		return automationmodel.Job{}, ErrJobNotFound
	}
	return *job, nil
}

func (repo *MemoryRepository) ListJobs(opts automationmodel.ListJobsOptions) ([]automationmodel.Job, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	jobs := make([]automationmodel.Job, 0, len(repo.jobs))
	for _, job := range repo.jobs {
		if opts.Status != "" && job.Status != opts.Status {
			continue
		}
		if opts.Adapter != "" && job.Adapter != opts.Adapter {
			continue
		}
		jobs = append(jobs, *job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return sliceJobs(jobs, opts.Offset, opts.Limit), nil
}

func (repo *MemoryRepository) NextJob(device workermodel.Device) (automationmodel.JobEnvelope, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	jobs := make([]*automationmodel.Job, 0, len(repo.jobs))
	for _, job := range repo.jobs {
		if job.Status == "queued" && adapterAllowed(job.Adapter, device.Capabilities) {
			jobs = append(jobs, job)
		}
	}
	if len(jobs) == 0 {
		return automationmodel.JobEnvelope{}, ErrNoJobAvailable
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Priority == jobs[j].Priority {
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		}
		return jobs[i].Priority > jobs[j].Priority
	})

	job := jobs[0]
	if _, ok := repo.activeRunByJob[job.ID]; ok {
		return automationmodel.JobEnvelope{}, ErrActiveRunExists
	}

	now := time.Now().UTC()
	run := automationmodel.Run{
		ID:        newID("run"),
		JobID:     job.ID,
		DeviceID:  device.ID,
		Status:    "running",
		StartedAt: now,
	}
	repo.runs[run.ID] = &run
	repo.activeRunByJob[job.ID] = run.ID
	job.Status = "running"
	job.UpdatedAt = now

	return automationmodel.JobEnvelope{
		JobID:   job.ID,
		RunID:   run.ID,
		JobType: job.Type,
		Adapter: job.Adapter,
		Target:  mapOrEmpty(job.Target),
		Input:   mapOrEmpty(job.Input),
		Policy:  mapOrEmpty(job.Policy),
		Cursor:  job.Cursor,
	}, nil
}

func (repo *MemoryRepository) Run(runID string) (automationmodel.Run, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	run, ok := repo.runs[runID]
	if !ok {
		return automationmodel.Run{}, ErrRunNotFound
	}
	return *run, nil
}

func (repo *MemoryRepository) ListRuns(opts automationmodel.ListRunsOptions) ([]automationmodel.Run, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	runs := make([]automationmodel.Run, 0, len(repo.runs))
	for _, run := range repo.runs {
		if opts.Status != "" && run.Status != opts.Status {
			continue
		}
		if opts.JobID != "" && run.JobID != opts.JobID {
			continue
		}
		if opts.DeviceID != "" && run.DeviceID != opts.DeviceID {
			continue
		}
		runs = append(runs, *run)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return sliceRuns(runs, opts.Offset, opts.Limit), nil
}

func (repo *MemoryRepository) Heartbeat(runID string, req automationmodel.HeartbeatRequest) (automationmodel.Run, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	run, ok := repo.runs[runID]
	if !ok {
		return automationmodel.Run{}, ErrRunNotFound
	}
	if run.Status == "cancelled" {
		return *run, nil
	}
	now := time.Now().UTC()
	run.Status = req.Status
	run.CurrentStep = req.CurrentStep
	run.LastCursor = req.Cursor
	run.LastHeartbeatAt = &now
	return *run, nil
}

func (repo *MemoryRepository) Checkpoint(runID string, payload automationmodel.Checkpoint) (automationmodel.Checkpoint, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	run, ok := repo.runs[runID]
	if !ok {
		return automationmodel.Checkpoint{}, ErrRunNotFound
	}
	checkpoint := automationmodel.Checkpoint{
		ID:        newID("chk"),
		RunID:     runID,
		JobID:     run.JobID,
		Cursor:    mapOrEmpty(payload.Cursor),
		Summary:   mapOrEmpty(payload.Summary),
		Status:    payload.Status,
		CreatedAt: time.Now().UTC(),
	}
	repo.checkpoints[runID] = append(repo.checkpoints[runID], checkpoint)
	return checkpoint, nil
}

func (repo *MemoryRepository) Checkpoints(runID string) ([]automationmodel.Checkpoint, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.runs[runID]; !ok {
		return nil, ErrRunNotFound
	}
	checkpoints := repo.checkpoints[runID]
	return append([]automationmodel.Checkpoint{}, checkpoints...), nil
}

func (repo *MemoryRepository) CreateArtifact(runID string, artifact automationmodel.Artifact) (automationmodel.Artifact, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.runs[runID]; !ok {
		return automationmodel.Artifact{}, ErrRunNotFound
	}
	artifact.ID = newID("art")
	artifact.RunID = runID
	artifact.CreatedAt = time.Now().UTC()
	repo.artifacts[runID] = append(repo.artifacts[runID], artifact)
	return artifact, nil
}

func (repo *MemoryRepository) Artifacts(runID string) ([]automationmodel.Artifact, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.runs[runID]; !ok {
		return nil, ErrRunNotFound
	}
	artifacts := repo.artifacts[runID]
	return append([]automationmodel.Artifact{}, artifacts...), nil
}

func (repo *MemoryRepository) Artifact(artifactID string) (automationmodel.Artifact, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	for _, artifacts := range repo.artifacts {
		for _, artifact := range artifacts {
			if artifact.ID == artifactID {
				return artifact, nil
			}
		}
	}
	return automationmodel.Artifact{}, ErrArtifactNotFound
}

func (repo *MemoryRepository) CreateManualAction(runID string, action automationmodel.ManualAction) (automationmodel.ManualAction, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.runs[runID]; !ok {
		return automationmodel.ManualAction{}, ErrRunNotFound
	}
	action.ID = newID("act")
	action.RunID = runID
	action.Status = "pending"
	action.CreatedAt = time.Now().UTC()
	repo.manualActions[action.ID] = &action
	return action, nil
}

func (repo *MemoryRepository) ManualActions(runID string) ([]automationmodel.ManualAction, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.runs[runID]; !ok {
		return nil, ErrRunNotFound
	}
	actions := make([]automationmodel.ManualAction, 0)
	for _, action := range repo.manualActions {
		if action.RunID == runID {
			actions = append(actions, *action)
		}
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].CreatedAt.Before(actions[j].CreatedAt)
	})
	return actions, nil
}

func (repo *MemoryRepository) ListManualActions(opts automationmodel.ListManualActionsOptions) ([]automationmodel.ManualAction, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	actions := make([]automationmodel.ManualAction, 0)
	for _, action := range repo.manualActions {
		if opts.Status != "" && action.Status != opts.Status {
			continue
		}
		if opts.RunID != "" && action.RunID != opts.RunID {
			continue
		}
		actions = append(actions, *action)
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].CreatedAt.After(actions[j].CreatedAt)
	})
	return sliceManualActions(actions, opts.Offset, opts.Limit), nil
}

func (repo *MemoryRepository) ManualAction(actionID string) (automationmodel.ManualAction, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	action, ok := repo.manualActions[actionID]
	if !ok {
		return automationmodel.ManualAction{}, ErrManualActionNotFound
	}
	return *action, nil
}

func (repo *MemoryRepository) ResolveManualAction(actionID string, req automationmodel.ResolveManualActionRequest) (automationmodel.ManualAction, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	action, ok := repo.manualActions[actionID]
	if !ok {
		return automationmodel.ManualAction{}, ErrManualActionNotFound
	}
	now := time.Now().UTC()
	action.Status = req.Status
	action.Payload = mapOrEmpty(req.Payload)
	action.ResolvedAt = &now
	return *action, nil
}

func (repo *MemoryRepository) CompleteRun(runID string, req automationmodel.CompleteRunRequest) (automationmodel.Run, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	run, ok := repo.runs[runID]
	if !ok {
		return automationmodel.Run{}, ErrRunNotFound
	}
	if run.Status == "cancelled" {
		return *run, nil
	}
	now := time.Now().UTC()
	run.Status = req.Status
	run.Summary = mapOrEmpty(req.Summary)
	run.LastCursor = mapOrEmpty(req.LastCursor)
	run.Error = mapOrEmpty(req.Error)
	run.CompletedAt = &now

	if job, ok := repo.jobs[run.JobID]; ok {
		job.Status = req.Status
		job.UpdatedAt = now
	}
	delete(repo.activeRunByJob, run.JobID)
	return *run, nil
}

func (repo *MemoryRepository) CancelRun(runID string, reason string) (automationmodel.Run, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	run, ok := repo.runs[runID]
	if !ok {
		return automationmodel.Run{}, ErrRunNotFound
	}
	now := time.Now().UTC()
	run.Status = "cancelled"
	run.Error = map[string]any{"code": "RUN_CANCELLED", "message": reason}
	run.CompletedAt = &now
	if job, ok := repo.jobs[run.JobID]; ok {
		job.Status = "cancelled"
		job.UpdatedAt = now
	}
	delete(repo.activeRunByJob, run.JobID)
	return *run, nil
}

func adapterAllowed(adapter string, capabilities []string) bool {
	if adapter == "mock.echo" {
		for _, capability := range capabilities {
			if capability == "adapter.mock.echo" {
				return true
			}
		}
		return false
	}
	return true
}

func mapOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func newID(prefix string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}

func sliceJobs(values []automationmodel.Job, offset int, limit int) []automationmodel.Job {
	if offset >= len(values) {
		return []automationmodel.Job{}
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return append([]automationmodel.Job{}, values[offset:end]...)
}

func sliceRuns(values []automationmodel.Run, offset int, limit int) []automationmodel.Run {
	if offset >= len(values) {
		return []automationmodel.Run{}
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return append([]automationmodel.Run{}, values[offset:end]...)
}

func sliceManualActions(values []automationmodel.ManualAction, offset int, limit int) []automationmodel.ManualAction {
	if offset >= len(values) {
		return []automationmodel.ManualAction{}
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return append([]automationmodel.ManualAction{}, values[offset:end]...)
}
