package automation

import (
	"context"
	"time"

	"qiyuan/backend-api/internal/lock"
	automationmodel "qiyuan/backend-api/internal/model/automation"
	workermodel "qiyuan/backend-api/internal/model/worker"
	automationrepo "qiyuan/backend-api/internal/repository/automation"
)

type Repository interface {
	CreateJob(req automationmodel.CreateJobRequest) automationmodel.Job
	Job(jobID string) (automationmodel.Job, error)
	ListJobs(opts automationmodel.ListJobsOptions) ([]automationmodel.Job, error)
	NextJob(device workermodel.Device) (automationmodel.JobEnvelope, error)
	Run(runID string) (automationmodel.Run, error)
	ListRuns(opts automationmodel.ListRunsOptions) ([]automationmodel.Run, error)
	Heartbeat(runID string, req automationmodel.HeartbeatRequest) (automationmodel.Run, error)
	Checkpoint(runID string, checkpoint automationmodel.Checkpoint) (automationmodel.Checkpoint, error)
	Checkpoints(runID string) ([]automationmodel.Checkpoint, error)
	CreateArtifact(runID string, artifact automationmodel.Artifact) (automationmodel.Artifact, error)
	Artifacts(runID string) ([]automationmodel.Artifact, error)
	Artifact(artifactID string) (automationmodel.Artifact, error)
	CreateManualAction(runID string, action automationmodel.ManualAction) (automationmodel.ManualAction, error)
	ManualActions(runID string) ([]automationmodel.ManualAction, error)
	ListManualActions(opts automationmodel.ListManualActionsOptions) ([]automationmodel.ManualAction, error)
	ManualAction(actionID string) (automationmodel.ManualAction, error)
	ResolveManualAction(actionID string, req automationmodel.ResolveManualActionRequest) (automationmodel.ManualAction, error)
	CompleteRun(runID string, req automationmodel.CompleteRunRequest) (automationmodel.Run, error)
	CancelRun(runID string, reason string) (automationmodel.Run, error)
}

type Engine struct {
	repo   Repository
	locker lock.Locker
}

func New(repo Repository, locker lock.Locker) *Engine {
	if locker == nil {
		locker = lock.NoopLocker{}
	}
	return &Engine{repo: repo, locker: locker}
}

func (engine *Engine) CreateJob(req automationmodel.CreateJobRequest) automationmodel.Job {
	return engine.repo.CreateJob(req)
}

func (engine *Engine) Job(jobID string) (automationmodel.Job, error) {
	return engine.repo.Job(jobID)
}

func (engine *Engine) ListJobs(opts automationmodel.ListJobsOptions) ([]automationmodel.Job, error) {
	return engine.repo.ListJobs(normalizeJobListOptions(opts))
}

func (engine *Engine) NextJob(device workermodel.Device) (automationmodel.JobEnvelope, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	claimLock, ok, err := engine.locker.TryLock(ctx, "automation:jobs:claim", 5*time.Second)
	if err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	if !ok {
		return automationmodel.JobEnvelope{}, automationrepo.ErrNoJobAvailable
	}
	defer claimLock.Release(context.Background())

	return engine.repo.NextJob(device)
}

func (engine *Engine) Heartbeat(runID string, req automationmodel.HeartbeatRequest) (automationmodel.Run, error) {
	return engine.repo.Heartbeat(runID, req)
}

func (engine *Engine) Run(runID string) (automationmodel.Run, error) {
	return engine.repo.Run(runID)
}

func (engine *Engine) ListRuns(opts automationmodel.ListRunsOptions) ([]automationmodel.Run, error) {
	return engine.repo.ListRuns(normalizeRunListOptions(opts))
}

func (engine *Engine) Checkpoint(runID string, checkpoint automationmodel.Checkpoint) (automationmodel.Checkpoint, error) {
	return engine.repo.Checkpoint(runID, checkpoint)
}

func (engine *Engine) Checkpoints(runID string) ([]automationmodel.Checkpoint, error) {
	return engine.repo.Checkpoints(runID)
}

func (engine *Engine) CreateArtifact(runID string, artifact automationmodel.Artifact) (automationmodel.Artifact, error) {
	return engine.repo.CreateArtifact(runID, artifact)
}

func (engine *Engine) Artifacts(runID string) ([]automationmodel.Artifact, error) {
	return engine.repo.Artifacts(runID)
}

func (engine *Engine) Artifact(artifactID string) (automationmodel.Artifact, error) {
	return engine.repo.Artifact(artifactID)
}

func (engine *Engine) CreateManualAction(runID string, action automationmodel.ManualAction) (automationmodel.ManualAction, error) {
	return engine.repo.CreateManualAction(runID, action)
}

func (engine *Engine) ManualActions(runID string) ([]automationmodel.ManualAction, error) {
	return engine.repo.ManualActions(runID)
}

func (engine *Engine) ListManualActions(opts automationmodel.ListManualActionsOptions) ([]automationmodel.ManualAction, error) {
	return engine.repo.ListManualActions(normalizeManualActionListOptions(opts))
}

func (engine *Engine) ManualAction(actionID string) (automationmodel.ManualAction, error) {
	return engine.repo.ManualAction(actionID)
}

func (engine *Engine) ResolveManualAction(actionID string, req automationmodel.ResolveManualActionRequest) (automationmodel.ManualAction, error) {
	if req.Status == "" {
		req.Status = "resolved"
	}
	return engine.repo.ResolveManualAction(actionID, req)
}

func (engine *Engine) CompleteRun(runID string, req automationmodel.CompleteRunRequest) (automationmodel.Run, error) {
	return engine.repo.CompleteRun(runID, req)
}

func (engine *Engine) CancelRun(runID string, reason string) (automationmodel.Run, error) {
	return engine.repo.CancelRun(runID, reason)
}

func normalizeLimitOffset(limit int, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func normalizeJobListOptions(opts automationmodel.ListJobsOptions) automationmodel.ListJobsOptions {
	opts.Limit, opts.Offset = normalizeLimitOffset(opts.Limit, opts.Offset)
	return opts
}

func normalizeRunListOptions(opts automationmodel.ListRunsOptions) automationmodel.ListRunsOptions {
	opts.Limit, opts.Offset = normalizeLimitOffset(opts.Limit, opts.Offset)
	return opts
}

func normalizeManualActionListOptions(opts automationmodel.ListManualActionsOptions) automationmodel.ListManualActionsOptions {
	opts.Limit, opts.Offset = normalizeLimitOffset(opts.Limit, opts.Offset)
	return opts
}
