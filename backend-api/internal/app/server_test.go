package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"qiyuan/backend-api/internal/config"
)

func TestAutomationMockJobLifecycle(t *testing.T) {
	server := NewServer().Handler()

	pairingResp := postJSON(t, server, "/worker/devices/pairing", map[string]any{
		"worker_version": "0.1.0",
		"platform":       "darwin-arm64",
		"display_name":   "test worker",
	}, "")
	if pairingResp.Code != http.StatusCreated {
		t.Fatalf("create pairing status = %d body=%s", pairingResp.Code, pairingResp.Body.String())
	}
	var pairing map[string]any
	decodeJSON(t, pairingResp, &pairing)

	getPairingReq := httptest.NewRequest(http.MethodGet, "/worker/devices/pairing/"+pairing["pairing_id"].(string), nil)
	getPairingResp := httptest.NewRecorder()
	server.ServeHTTP(getPairingResp, getPairingReq)
	if getPairingResp.Code != http.StatusOK {
		t.Fatalf("get pairing status = %d body=%s", getPairingResp.Code, getPairingResp.Body.String())
	}
	var approved map[string]any
	decodeJSON(t, getPairingResp, &approved)
	token := approved["device_token"].(string)
	device := approved["device"].(map[string]any)
	deviceID := device["id"].(string)

	heartbeatResp := postJSON(t, server, "/worker/devices/"+deviceID+"/heartbeat", map[string]any{
		"worker_version": "0.1.0",
		"status":         "idle",
		"capabilities":   []string{"adapter.mock.echo"},
	}, token)
	if heartbeatResp.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body=%s", heartbeatResp.Code, heartbeatResp.Body.String())
	}

	createJobResp := postJSON(t, server, "/admin/automation/jobs", map[string]any{
		"job_type": "generic.browser.script",
		"adapter":  "mock.echo",
		"target": map[string]any{
			"allowed_domains": []string{"example.com"},
		},
		"input": map[string]any{
			"message": "hello",
		},
	}, "")
	if createJobResp.Code != http.StatusCreated {
		t.Fatalf("create job status = %d body=%s", createJobResp.Code, createJobResp.Body.String())
	}

	nextReq := httptest.NewRequest(http.MethodGet, "/worker/automation/jobs/next", nil)
	nextReq.Header.Set("Authorization", "Bearer "+token)
	nextResp := httptest.NewRecorder()
	server.ServeHTTP(nextResp, nextReq)
	if nextResp.Code != http.StatusOK {
		t.Fatalf("next job status = %d body=%s", nextResp.Code, nextResp.Body.String())
	}
	var job map[string]any
	decodeJSON(t, nextResp, &job)
	runID := job["run_id"].(string)

	checkpointResp := postJSON(t, server, "/worker/automation/runs/"+runID+"/checkpoint", map[string]any{
		"status":  "completed",
		"summary": map[string]any{"ok": true},
	}, token)
	if checkpointResp.Code != http.StatusCreated {
		t.Fatalf("checkpoint status = %d body=%s", checkpointResp.Code, checkpointResp.Body.String())
	}

	artifactResp := postJSON(t, server, "/worker/automation/runs/"+runID+"/artifacts", map[string]any{
		"artifact_type": "mock.summary",
		"metadata":      map[string]any{"ok": true},
	}, token)
	if artifactResp.Code != http.StatusCreated {
		t.Fatalf("artifact status = %d body=%s", artifactResp.Code, artifactResp.Body.String())
	}

	completeResp := postJSON(t, server, "/worker/automation/runs/"+runID+"/complete", map[string]any{
		"job_id":  job["job_id"],
		"status":  "completed",
		"summary": map[string]any{"ok": true},
	}, token)
	if completeResp.Code != http.StatusOK {
		t.Fatalf("complete status = %d body=%s", completeResp.Code, completeResp.Body.String())
	}

	getJobResp := getJSON(t, server, "/admin/automation/jobs/"+job["job_id"].(string), "")
	if getJobResp.Code != http.StatusOK {
		t.Fatalf("get job status = %d body=%s", getJobResp.Code, getJobResp.Body.String())
	}

	getRunResp := getJSON(t, server, "/admin/automation/runs/"+runID, "")
	if getRunResp.Code != http.StatusOK {
		t.Fatalf("get run status = %d body=%s", getRunResp.Code, getRunResp.Body.String())
	}

	getArtifactsResp := getJSON(t, server, "/admin/automation/runs/"+runID+"/artifacts", "")
	if getArtifactsResp.Code != http.StatusOK {
		t.Fatalf("get artifacts status = %d body=%s", getArtifactsResp.Code, getArtifactsResp.Body.String())
	}
	var artifacts map[string]any
	decodeJSON(t, getArtifactsResp, &artifacts)
	if len(artifacts["artifacts"].([]any)) != 1 {
		t.Fatalf("artifacts len = %d, want 1", len(artifacts["artifacts"].([]any)))
	}

	getCheckpointsResp := getJSON(t, server, "/admin/automation/runs/"+runID+"/checkpoints", "")
	if getCheckpointsResp.Code != http.StatusOK {
		t.Fatalf("get checkpoints status = %d body=%s", getCheckpointsResp.Code, getCheckpointsResp.Body.String())
	}
	var checkpoints map[string]any
	decodeJSON(t, getCheckpointsResp, &checkpoints)
	if len(checkpoints["checkpoints"].([]any)) != 1 {
		t.Fatalf("checkpoints len = %d, want 1", len(checkpoints["checkpoints"].([]any)))
	}
}

func TestAutomationTraceAndCancel(t *testing.T) {
	artifactDir := t.TempDir()
	server := NewServerWithConfig(config.Config{ArtifactDir: artifactDir}).Handler()
	token := pairTestWorker(t, server)

	createJobResp := postJSON(t, server, "/admin/automation/jobs", map[string]any{
		"job_type": "generic.browser.script",
		"adapter":  "mock.echo",
		"target":   map[string]any{"allowed_domains": []string{"example.com"}},
		"input":    map[string]any{"message": "trace"},
	}, "")
	if createJobResp.Code != http.StatusCreated {
		t.Fatalf("create job status = %d body=%s", createJobResp.Code, createJobResp.Body.String())
	}

	nextReq := httptest.NewRequest(http.MethodGet, "/worker/automation/jobs/next", nil)
	nextReq.Header.Set("Authorization", "Bearer "+token)
	nextResp := httptest.NewRecorder()
	server.ServeHTTP(nextResp, nextReq)
	if nextResp.Code != http.StatusOK {
		t.Fatalf("next job status = %d body=%s", nextResp.Code, nextResp.Body.String())
	}
	var job map[string]any
	decodeJSON(t, nextResp, &job)
	runID := job["run_id"].(string)

	tracePath := filepath.Join(artifactDir, runID, "agent-trace.json")
	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracePath, []byte(`{"steps":[{"step":"action.start","action":"observe_page"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactResp := postJSON(t, server, "/worker/automation/runs/"+runID+"/artifacts", map[string]any{
		"artifact_type": "agent_trace",
		"local_path":    tracePath,
	}, token)
	if artifactResp.Code != http.StatusCreated {
		t.Fatalf("artifact status = %d body=%s", artifactResp.Code, artifactResp.Body.String())
	}

	traceResp := getJSON(t, server, "/admin/automation/runs/"+runID+"/trace", "")
	if traceResp.Code != http.StatusOK {
		t.Fatalf("trace status = %d body=%s", traceResp.Code, traceResp.Body.String())
	}
	var trace map[string]any
	decodeJSON(t, traceResp, &trace)
	steps := trace["trace"].(map[string]any)["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("trace steps len = %d, want 1", len(steps))
	}

	cancelResp := postJSON(t, server, "/admin/automation/runs/"+runID+"/cancel", map[string]any{"reason": "test cancel"}, "")
	if cancelResp.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s", cancelResp.Code, cancelResp.Body.String())
	}
	var cancelled map[string]any
	decodeJSON(t, cancelResp, &cancelled)
	if cancelled["status"] != "cancelled" {
		t.Fatalf("cancelled status = %v", cancelled["status"])
	}

	workerRunReq := httptest.NewRequest(http.MethodGet, "/worker/automation/runs/"+runID, nil)
	workerRunReq.Header.Set("Authorization", "Bearer "+token)
	workerRunResp := httptest.NewRecorder()
	server.ServeHTTP(workerRunResp, workerRunReq)
	if workerRunResp.Code != http.StatusOK {
		t.Fatalf("worker run status = %d body=%s", workerRunResp.Code, workerRunResp.Body.String())
	}
	var workerRun map[string]any
	decodeJSON(t, workerRunResp, &workerRun)
	if workerRun["status"] != "cancelled" {
		t.Fatalf("worker run status = %v", workerRun["status"])
	}
}

func TestRoleAuthTokens(t *testing.T) {
	server := NewServerWithConfig(config.Config{
		AdminAPIToken: "admin-token",
		WebAPIToken:   "web-token",
	}).Handler()

	adminDenied := getJSON(t, server, "/admin/automation/jobs", "")
	if adminDenied.Code != http.StatusUnauthorized {
		t.Fatalf("admin without token status = %d", adminDenied.Code)
	}
	adminAllowed := getJSON(t, server, "/admin/automation/jobs", "admin-token")
	if adminAllowed.Code != http.StatusOK {
		t.Fatalf("admin with token status = %d body=%s", adminAllowed.Code, adminAllowed.Body.String())
	}

	webDenied := getJSON(t, server, "/web/automation/jobs", "")
	if webDenied.Code != http.StatusUnauthorized {
		t.Fatalf("web without token status = %d", webDenied.Code)
	}
	webAllowed := getJSON(t, server, "/web/automation/jobs", "web-token")
	if webAllowed.Code != http.StatusOK {
		t.Fatalf("web with token status = %d body=%s", webAllowed.Code, webAllowed.Body.String())
	}
}

func TestCreateWebBrowserActJob(t *testing.T) {
	server := NewServer().Handler()

	resp := postJSON(t, server, "/web/automation/browser-act-jobs", map[string]any{
		"url":             "https://example.com",
		"task":            "open example",
		"allowed_domains": []string{"example.com"},
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create browser.act job status = %d body=%s", resp.Code, resp.Body.String())
	}
	var job map[string]any
	decodeJSON(t, resp, &job)
	if job["adapter"] != "browser.act" {
		t.Fatalf("adapter = %v", job["adapter"])
	}
	if job["job_type"] != "generic.browser.act" {
		t.Fatalf("job_type = %v", job["job_type"])
	}
	if got := job["input"].(map[string]any)["mode"]; got != "cli" {
		t.Fatalf("input.mode = %v", got)
	}
}

func TestCreateWebSocialUploadJob(t *testing.T) {
	server := NewServer().Handler()

	resp := postJSON(t, server, "/web/automation/social-upload-jobs", map[string]any{
		"platform":    "instagram",
		"video_path":  "/tmp/reel.mp4",
		"title":       "Launch reel",
		"description": "hello",
		"tags":        []string{"#AI", "automation", "AI"},
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create social upload job status = %d body=%s", resp.Code, resp.Body.String())
	}
	var job map[string]any
	decodeJSON(t, resp, &job)
	if job["adapter"] != "social.instagram.upload_video" {
		t.Fatalf("adapter = %v", job["adapter"])
	}
	if job["job_type"] != "social.instagram.upload_video" {
		t.Fatalf("job_type = %v", job["job_type"])
	}
	input := job["input"].(map[string]any)
	if input["title"] != "Launch reel" {
		t.Fatalf("input.title = %v", input["title"])
	}
	tags := input["tags"].([]any)
	if len(tags) != 2 || tags[0] != "AI" || tags[1] != "automation" {
		t.Fatalf("input.tags = %#v", tags)
	}
	policy := job["policy"].(map[string]any)
	if policy["manual_publish_required"] != false {
		t.Fatalf("manual_publish_required = %v", policy["manual_publish_required"])
	}

	manualResp := postJSON(t, server, "/web/automation/social-upload-jobs", map[string]any{
		"platform":                "instagram",
		"video_path":              "/tmp/reel.mp4",
		"title":                   "Manual review reel",
		"manual_publish_required": true,
	}, "")
	if manualResp.Code != http.StatusCreated {
		t.Fatalf("create manual social upload job status = %d body=%s", manualResp.Code, manualResp.Body.String())
	}
	var manualJob map[string]any
	decodeJSON(t, manualResp, &manualJob)
	manualPolicy := manualJob["policy"].(map[string]any)
	if manualPolicy["manual_publish_required"] != false {
		t.Fatalf("manual manual_publish_required = %v", manualPolicy["manual_publish_required"])
	}
}

func TestCreateWebSocialUploadJobRejectsInvalidInput(t *testing.T) {
	server := NewServer().Handler()

	unsupported := postJSON(t, server, "/web/automation/social-upload-jobs", map[string]any{
		"platform":   "xiaohongshu",
		"video_path": "/tmp/video.mp4",
		"title":      "title",
	}, "")
	if unsupported.Code != http.StatusBadRequest {
		t.Fatalf("unsupported platform status = %d body=%s", unsupported.Code, unsupported.Body.String())
	}

	missingVideo := postJSON(t, server, "/web/automation/social-upload-jobs", map[string]any{
		"platform": "instagram",
		"title":    "title",
	}, "")
	if missingVideo.Code != http.StatusBadRequest {
		t.Fatalf("missing video status = %d body=%s", missingVideo.Code, missingVideo.Body.String())
	}
}

func TestCreateWebWeixinDesktopSyncJob(t *testing.T) {
	server := NewServer().Handler()

	resp := postJSON(t, server, "/web/automation/weixin-desktop-sync-jobs", map[string]any{
		"source_dirs": []string{"/Users/mac/Library/Containers/com.tencent.xinWeChat"},
		"group_names": []string{"科研群"},
		"selected_groups": []map[string]any{
			{"group_id": "local-1", "display_name": "项目资料群"},
		},
		"max_files": 50,
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create weixin desktop sync job status = %d body=%s", resp.Code, resp.Body.String())
	}
	var job map[string]any
	decodeJSON(t, resp, &job)
	if job["adapter"] != "weixin.desktop_sync" {
		t.Fatalf("adapter = %v", job["adapter"])
	}
	if job["job_type"] != "weixin.desktop_sync" {
		t.Fatalf("job_type = %v", job["job_type"])
	}
	input := job["input"].(map[string]any)
	sourceDirs := input["source_dirs"].([]any)
	if len(sourceDirs) != 1 {
		t.Fatalf("source_dirs = %#v", sourceDirs)
	}
	groupNames := input["group_names"].([]any)
	if len(groupNames) != 2 {
		t.Fatalf("group_names = %#v", groupNames)
	}
	selectedGroups := input["selected_groups"].([]any)
	if len(selectedGroups) != 2 {
		t.Fatalf("selected_groups = %#v", selectedGroups)
	}
	policy := job["policy"].(map[string]any)
	if policy["max_files"] != float64(50) {
		t.Fatalf("max_files = %v", policy["max_files"])
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, payload map[string]any, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func pairTestWorker(t *testing.T, handler http.Handler) string {
	t.Helper()
	pairingResp := postJSON(t, handler, "/worker/devices/pairing", map[string]any{
		"worker_version": "0.1.0",
		"platform":       "darwin-arm64",
		"display_name":   "test worker",
	}, "")
	if pairingResp.Code != http.StatusCreated {
		t.Fatalf("create pairing status = %d body=%s", pairingResp.Code, pairingResp.Body.String())
	}
	var pairing map[string]any
	decodeJSON(t, pairingResp, &pairing)

	getPairingReq := httptest.NewRequest(http.MethodGet, "/worker/devices/pairing/"+pairing["pairing_id"].(string), nil)
	getPairingResp := httptest.NewRecorder()
	handler.ServeHTTP(getPairingResp, getPairingReq)
	if getPairingResp.Code != http.StatusOK {
		t.Fatalf("get pairing status = %d body=%s", getPairingResp.Code, getPairingResp.Body.String())
	}
	var approved map[string]any
	decodeJSON(t, getPairingResp, &approved)
	token := approved["device_token"].(string)
	device := approved["device"].(map[string]any)
	deviceID := device["id"].(string)

	heartbeatResp := postJSON(t, handler, "/worker/devices/"+deviceID+"/heartbeat", map[string]any{
		"worker_version": "0.1.0",
		"status":         "idle",
		"capabilities":   []string{"adapter.mock.echo"},
	}, token)
	if heartbeatResp.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body=%s", heartbeatResp.Code, heartbeatResp.Body.String())
	}
	return token
}

func decodeJSON(t *testing.T, resp *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(resp.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v body=%s", err, resp.Body.String())
	}
}

func getJSON(t *testing.T, handler http.Handler, path string, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
