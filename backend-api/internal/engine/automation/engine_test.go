package automation

import (
	"context"
	"testing"
	"time"

	"qiyuan/backend-api/internal/lock"
	automationmodel "qiyuan/backend-api/internal/model/automation"
	workermodel "qiyuan/backend-api/internal/model/worker"
	automationrepo "qiyuan/backend-api/internal/repository/automation"
)

func TestNextJobSkipsRepositoryWhenClaimLockUnavailable(t *testing.T) {
	repo := &countingRepository{}
	engine := New(repo, denyingLocker{})

	_, err := engine.NextJob(workermodel.Device{ID: "wdev_1"})
	if err != automationrepo.ErrNoJobAvailable {
		t.Fatalf("err = %v, want ErrNoJobAvailable", err)
	}
	if repo.nextJobCalls != 0 {
		t.Fatalf("NextJob called %d times, want 0", repo.nextJobCalls)
	}
}

func TestNextJobUsesRepositoryWhenClaimLockAvailable(t *testing.T) {
	repo := &countingRepository{job: automationmodel.JobEnvelope{JobID: "job_1", RunID: "run_1"}}
	engine := New(repo, allowingLocker{})

	job, err := engine.NextJob(workermodel.Device{ID: "wdev_1"})
	if err != nil {
		t.Fatalf("NextJob returned error: %v", err)
	}
	if job.JobID != "job_1" {
		t.Fatalf("job id = %s, want job_1", job.JobID)
	}
	if repo.nextJobCalls != 1 {
		t.Fatalf("NextJob called %d times, want 1", repo.nextJobCalls)
	}
}

type countingRepository struct {
	nextJobCalls int
	job          automationmodel.JobEnvelope
}

func (repo *countingRepository) CreateJob(req automationmodel.CreateJobRequest) automationmodel.Job {
	return automationmodel.Job{}
}

func (repo *countingRepository) Job(jobID string) (automationmodel.Job, error) {
	return automationmodel.Job{}, nil
}

func (repo *countingRepository) ListJobs(opts automationmodel.ListJobsOptions) ([]automationmodel.Job, error) {
	return nil, nil
}

func (repo *countingRepository) NextJob(device workermodel.Device) (automationmodel.JobEnvelope, error) {
	repo.nextJobCalls++
	if repo.job.JobID == "" {
		return automationmodel.JobEnvelope{}, automationrepo.ErrNoJobAvailable
	}
	return repo.job, nil
}

func (repo *countingRepository) Run(runID string) (automationmodel.Run, error) {
	return automationmodel.Run{}, nil
}

func (repo *countingRepository) ListRuns(opts automationmodel.ListRunsOptions) ([]automationmodel.Run, error) {
	return nil, nil
}

func (repo *countingRepository) Heartbeat(runID string, req automationmodel.HeartbeatRequest) (automationmodel.Run, error) {
	return automationmodel.Run{}, nil
}

func (repo *countingRepository) Checkpoint(runID string, checkpoint automationmodel.Checkpoint) (automationmodel.Checkpoint, error) {
	return automationmodel.Checkpoint{}, nil
}

func (repo *countingRepository) Checkpoints(runID string) ([]automationmodel.Checkpoint, error) {
	return nil, nil
}

func (repo *countingRepository) CreateArtifact(runID string, artifact automationmodel.Artifact) (automationmodel.Artifact, error) {
	return automationmodel.Artifact{}, nil
}

func (repo *countingRepository) Artifacts(runID string) ([]automationmodel.Artifact, error) {
	return nil, nil
}

func (repo *countingRepository) Artifact(artifactID string) (automationmodel.Artifact, error) {
	return automationmodel.Artifact{}, nil
}

func (repo *countingRepository) CreateManualAction(runID string, action automationmodel.ManualAction) (automationmodel.ManualAction, error) {
	return automationmodel.ManualAction{}, nil
}

func (repo *countingRepository) ManualActions(runID string) ([]automationmodel.ManualAction, error) {
	return nil, nil
}

func (repo *countingRepository) ListManualActions(opts automationmodel.ListManualActionsOptions) ([]automationmodel.ManualAction, error) {
	return nil, nil
}

func (repo *countingRepository) ManualAction(actionID string) (automationmodel.ManualAction, error) {
	return automationmodel.ManualAction{}, nil
}

func (repo *countingRepository) ResolveManualAction(actionID string, req automationmodel.ResolveManualActionRequest) (automationmodel.ManualAction, error) {
	return automationmodel.ManualAction{}, nil
}

func (repo *countingRepository) CompleteRun(runID string, req automationmodel.CompleteRunRequest) (automationmodel.Run, error) {
	return automationmodel.Run{}, nil
}

func (repo *countingRepository) CancelRun(runID string, reason string) (automationmodel.Run, error) {
	return automationmodel.Run{}, nil
}

type denyingLocker struct{}

func (denyingLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (lock.Lock, bool, error) {
	return nil, false, nil
}

type allowingLocker struct{}

func (allowingLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (lock.Lock, bool, error) {
	return noopTestLock{}, true, nil
}

type noopTestLock struct{}

func (noopTestLock) Release(ctx context.Context) error {
	return nil
}
