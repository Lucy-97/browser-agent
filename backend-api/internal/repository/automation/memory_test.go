package automation

import (
	"testing"

	automationmodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/automation"
	workermodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/worker"
)

func TestNextJobRequiresMatchingAdapterCapability(t *testing.T) {
	repo := NewMemoryRepository()
	repo.CreateJob(automationmodel.CreateJobRequest{
		JobType: "weixin.desktop_sync",
		Adapter: "weixin.desktop_sync",
	})

	_, err := repo.NextJob(workermodel.Device{
		ID:           "wdev_without_weixin",
		Capabilities: []string{"adapter.mock.echo"},
	})
	if err != ErrNoJobAvailable {
		t.Fatalf("NextJob without matching capability err = %v, want ErrNoJobAvailable", err)
	}

	job, err := repo.NextJob(workermodel.Device{
		ID:           "wdev_with_weixin",
		Capabilities: []string{"adapter.weixin.desktop_sync"},
	})
	if err != nil {
		t.Fatalf("NextJob with matching capability returned error: %v", err)
	}
	if job.Adapter != "weixin.desktop_sync" {
		t.Fatalf("claimed adapter = %q, want weixin.desktop_sync", job.Adapter)
	}
}
