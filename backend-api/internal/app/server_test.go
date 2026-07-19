package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lucy-97/browser-agent/backend-api/internal/config"
	"github.com/Lucy-97/browser-agent/backend-api/internal/identity"
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

func TestAccountRegistrationLoginAndMembershipValidation(t *testing.T) {
	const internalSecret = "test-internal-secret"
	server := NewServerWithConfig(config.Config{
		InternalSecret:        internalSecret,
		RequireTenantIdentity: true,
		RequireMembership:     true,
		JWTSecret:             "test-jwt-secret-with-enough-entropy",
		JWTAccessTokenExpSec:  300,
		AuthCookieName:        "browser_agent_access",
		AllowRegistration:     true,
	}).Handler()

	register := postJSON(t, server, "/api/v1/auth/register", map[string]any{
		"email":       "owner@example.com",
		"password":    "correct horse battery staple",
		"nickname":    "Owner",
		"tenant_name": "Example Studio",
	}, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", register.Code, register.Body.String())
	}
	var registered map[string]any
	decodeJSON(t, register, &registered)
	if registered["access_token"] == "" || registered["role"] != identity.RoleTenantOwner {
		t.Fatalf("registered auth result = %#v", registered)
	}
	userID := registered["user"].(map[string]any)["user_id"].(string)
	tenantID := registered["tenant"].(map[string]any)["tenant_id"].(string)

	cookies := register.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "browser_agent_access" || !cookies[0].HttpOnly {
		t.Fatalf("register cookies = %#v", cookies)
	}
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(cookies[0])
	meResp := httptest.NewRecorder()
	server.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%s", meResp.Code, meResp.Body.String())
	}

	ownedJob := postJSONWithActor(t, server, "/web/automation/browser-act-jobs", map[string]any{
		"url":             "https://example.com",
		"task":            "membership test",
		"allowed_domains": []string{"example.com"},
	}, tenantID, userID, identity.RoleTenantOwner, internalSecret)
	if ownedJob.Code != http.StatusCreated {
		t.Fatalf("member create job status = %d body=%s", ownedJob.Code, ownedJob.Body.String())
	}

	wrongRole := getJSONWithActor(t, server, "/web/automation/jobs", tenantID, userID, identity.RoleTenantViewer, internalSecret)
	if wrongRole.Code != http.StatusForbidden {
		t.Fatalf("stale membership role status = %d body=%s", wrongRole.Code, wrongRole.Body.String())
	}
	unknownMember := getJSONWithActor(t, server, "/web/automation/jobs", tenantID, "unknown-user", identity.RoleTenantOwner, internalSecret)
	if unknownMember.Code != http.StatusForbidden {
		t.Fatalf("unknown membership status = %d body=%s", unknownMember.Code, unknownMember.Body.String())
	}

	wrongPassword := postJSON(t, server, "/api/v1/auth/login", map[string]any{
		"email": "owner@example.com", "password": "wrong-password-value",
	}, "")
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d body=%s", wrongPassword.Code, wrongPassword.Body.String())
	}
	login := postJSON(t, server, "/api/v1/auth/login", map[string]any{
		"email": "OWNER@example.com", "password": "correct horse battery staple", "tenant_id": tenantID,
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	duplicate := postJSON(t, server, "/api/v1/auth/register", map[string]any{
		"email":       "owner@example.com",
		"password":    "another correct password",
		"nickname":    "Duplicate",
		"tenant_name": "Duplicate Studio",
	}, "")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d body=%s", duplicate.Code, duplicate.Body.String())
	}
}

func TestPublicRegistrationIsClosedByDefault(t *testing.T) {
	server := NewServerWithConfig(config.Config{JWTSecret: "test-jwt-secret-with-at-least-32-bytes"}).Handler()
	resp := postJSON(t, server, "/api/v1/auth/register", map[string]any{
		"email":       "closed@example.com",
		"password":    "correct horse battery staple",
		"nickname":    "Closed",
		"tenant_name": "Closed Studio",
	}, "")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("closed registration status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestAuthenticationRejectsWeakSecretAndTrailingJSON(t *testing.T) {
	weakSecretServer := NewServerWithConfig(config.Config{
		JWTSecret:         "too-short",
		AllowRegistration: true,
	}).Handler()
	weakSecret := postJSON(t, weakSecretServer, "/api/v1/auth/register", map[string]any{
		"email": "weak@example.com", "password": "correct horse battery staple",
		"nickname": "Weak", "tenant_name": "Weak Secret",
	}, "")
	if weakSecret.Code != http.StatusServiceUnavailable {
		t.Fatalf("weak secret status = %d body=%s", weakSecret.Code, weakSecret.Body.String())
	}

	server := NewServerWithConfig(config.Config{
		JWTSecret:         "test-jwt-secret-with-at-least-32-bytes",
		AllowRegistration: true,
	}).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{} {}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTenantIsolationAndWorkerOwnership(t *testing.T) {
	const internalSecret = "test-internal-secret"
	server := NewServerWithConfig(config.Config{
		ArtifactDir:           t.TempDir(),
		InternalSecret:        internalSecret,
		RequireTenantIdentity: true,
	}).Handler()

	missingIdentity := getJSON(t, server, "/web/automation/jobs", "")
	if missingIdentity.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant identity status = %d body=%s", missingIdentity.Code, missingIdentity.Body.String())
	}
	untrustedReq := httptest.NewRequest(http.MethodGet, "/web/automation/jobs", nil)
	setActorHeaders(untrustedReq, "tenant_a", "user_a", identity.RoleTenantOwner, "")
	untrustedResp := httptest.NewRecorder()
	server.ServeHTTP(untrustedResp, untrustedReq)
	if untrustedResp.Code != http.StatusForbidden {
		t.Fatalf("untrusted tenant headers status = %d body=%s", untrustedResp.Code, untrustedResp.Body.String())
	}

	tokenA, deviceA := pairStrictWorker(t, server, "tenant_a", "user_a", internalSecret)
	tokenB, _ := pairStrictWorker(t, server, "tenant_b", "user_b", internalSecret)

	jobAResp := postJSONWithActor(t, server, "/admin/automation/jobs", map[string]any{
		"job_type": "generic.browser.script",
		"adapter":  "mock.echo",
		"target":   map[string]any{"allowed_domains": []string{"example.com"}},
		"input":    map[string]any{"message": "tenant a"},
		"priority": 10,
	}, "tenant_a", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if jobAResp.Code != http.StatusCreated {
		t.Fatalf("create tenant A job status = %d body=%s", jobAResp.Code, jobAResp.Body.String())
	}
	var jobA map[string]any
	decodeJSON(t, jobAResp, &jobA)

	jobBResp := postJSONWithActor(t, server, "/admin/automation/jobs", map[string]any{
		"job_type": "generic.browser.script",
		"adapter":  "mock.echo",
		"target":   map[string]any{"allowed_domains": []string{"example.com"}},
		"input":    map[string]any{"message": "tenant b"},
		"priority": 100,
	}, "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if jobBResp.Code != http.StatusCreated {
		t.Fatalf("create tenant B job status = %d body=%s", jobBResp.Code, jobBResp.Body.String())
	}
	var jobB map[string]any
	decodeJSON(t, jobBResp, &jobB)

	nextA := getJSON(t, server, "/worker/automation/jobs/next", tokenA)
	if nextA.Code != http.StatusOK {
		t.Fatalf("tenant A next job status = %d body=%s", nextA.Code, nextA.Body.String())
	}
	var claimedA map[string]any
	decodeJSON(t, nextA, &claimedA)
	if claimedA["job_id"] != jobA["job_id"] {
		t.Fatalf("tenant A claimed job = %v, want %v", claimedA["job_id"], jobA["job_id"])
	}
	runA := claimedA["run_id"].(string)

	nextB := getJSON(t, server, "/worker/automation/jobs/next", tokenB)
	if nextB.Code != http.StatusOK {
		t.Fatalf("tenant B next job status = %d body=%s", nextB.Code, nextB.Body.String())
	}
	var claimedB map[string]any
	decodeJSON(t, nextB, &claimedB)
	if claimedB["job_id"] != jobB["job_id"] {
		t.Fatalf("tenant B claimed job = %v, want %v", claimedB["job_id"], jobB["job_id"])
	}

	foreignCheckpoint := postJSON(t, server, "/worker/automation/runs/"+runA+"/checkpoint", map[string]any{
		"status": "running",
	}, tokenB)
	if foreignCheckpoint.Code != http.StatusNotFound {
		t.Fatalf("foreign worker checkpoint status = %d body=%s", foreignCheckpoint.Code, foreignCheckpoint.Body.String())
	}

	artifactResp := postJSON(t, server, "/worker/automation/runs/"+runA+"/artifacts", map[string]any{
		"artifact_type": "mock.summary",
		"metadata":      map[string]any{"tenant": "a"},
	}, tokenA)
	if artifactResp.Code != http.StatusCreated {
		t.Fatalf("tenant A artifact status = %d body=%s", artifactResp.Code, artifactResp.Body.String())
	}
	var artifact map[string]any
	decodeJSON(t, artifactResp, &artifact)
	manualActionResp := postJSON(t, server, "/worker/automation/runs/"+runA+"/manual-actions", map[string]any{
		"action_type": "confirmation",
		"message":     "tenant A confirmation",
	}, tokenA)
	if manualActionResp.Code != http.StatusCreated {
		t.Fatalf("tenant A manual action status = %d body=%s", manualActionResp.Code, manualActionResp.Body.String())
	}
	var manualAction map[string]any
	decodeJSON(t, manualActionResp, &manualAction)

	listA := getJSONWithActor(t, server, "/admin/automation/jobs", "tenant_a", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	assertSingleTenantJob(t, listA, jobA["job_id"].(string), "tenant_a")
	listB := getJSONWithActor(t, server, "/admin/automation/jobs", "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	assertSingleTenantJob(t, listB, jobB["job_id"].(string), "tenant_b")

	foreignJob := getJSONWithActor(t, server, "/admin/automation/jobs/"+jobA["job_id"].(string), "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if foreignJob.Code != http.StatusNotFound {
		t.Fatalf("foreign job status = %d body=%s", foreignJob.Code, foreignJob.Body.String())
	}
	foreignArtifacts := getJSONWithActor(t, server, "/admin/automation/runs/"+runA+"/artifacts", "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if foreignArtifacts.Code != http.StatusNotFound {
		t.Fatalf("foreign run artifacts status = %d body=%s", foreignArtifacts.Code, foreignArtifacts.Body.String())
	}
	foreignDownload := getJSONWithActor(t, server, "/admin/automation/artifacts/"+artifact["artifact_id"].(string)+"/download", "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if foreignDownload.Code != http.StatusNotFound {
		t.Fatalf("foreign artifact download status = %d body=%s", foreignDownload.Code, foreignDownload.Body.String())
	}
	foreignCancel := postJSONWithActor(t, server, "/admin/automation/runs/"+runA+"/cancel", map[string]any{
		"reason": "cross tenant cancel",
	}, "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if foreignCancel.Code != http.StatusNotFound {
		t.Fatalf("foreign run cancel status = %d body=%s", foreignCancel.Code, foreignCancel.Body.String())
	}
	foreignResolve := postJSONWithActor(t, server, "/admin/automation/manual-actions/"+manualAction["manual_action_id"].(string)+"/resolve", map[string]any{
		"status": "resolved",
	}, "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if foreignResolve.Code != http.StatusNotFound {
		t.Fatalf("foreign manual action resolve status = %d body=%s", foreignResolve.Code, foreignResolve.Body.String())
	}
	foreignRevoke := postJSONWithActor(t, server, "/admin/worker/devices/"+deviceA+"/revoke", map[string]any{}, "tenant_b", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	if foreignRevoke.Code != http.StatusNotFound {
		t.Fatalf("foreign device revoke status = %d body=%s", foreignRevoke.Code, foreignRevoke.Body.String())
	}
	devicesA := getJSONWithActor(t, server, "/admin/worker/devices", "tenant_a", "platform_admin", identity.RolePlatformAdmin, internalSecret)
	assertSingleTenantDevice(t, devicesA, deviceA, "tenant_a")
}

func TestTenantViewerCannotCreateJobsOrApproveDevices(t *testing.T) {
	const internalSecret = "test-internal-secret"
	server := NewServerWithConfig(config.Config{
		InternalSecret:        internalSecret,
		RequireTenantIdentity: true,
	}).Handler()

	createJob := postJSONWithActor(t, server, "/web/automation/browser-act-jobs", map[string]any{
		"url":             "https://example.com",
		"task":            "open example",
		"allowed_domains": []string{"example.com"},
	}, "tenant_a", "viewer_a", identity.RoleTenantViewer, internalSecret)
	if createJob.Code != http.StatusForbidden {
		t.Fatalf("viewer create job status = %d body=%s", createJob.Code, createJob.Body.String())
	}

	pairingResp := postJSON(t, server, "/worker/devices/pairing", map[string]any{
		"worker_version": "0.1.0",
		"platform":       "windows-amd64",
	}, "")
	var pairing map[string]any
	decodeJSON(t, pairingResp, &pairing)
	approve := postJSONWithActor(t, server, "/web/worker/pairings/"+pairing["pairing_code"].(string)+"/approve", map[string]any{}, "tenant_a", "viewer_a", identity.RoleTenantViewer, internalSecret)
	if approve.Code != http.StatusForbidden {
		t.Fatalf("viewer approve device status = %d body=%s", approve.Code, approve.Body.String())
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

func TestCreateWebBrowserAgentDeterministicJob(t *testing.T) {
	server := NewServer().Handler()

	resp := postJSON(t, server, "/web/automation/browser-agent-jobs", map[string]any{
		"url":             "data:text/html,test",
		"task":            "search fixture",
		"allowed_domains": []string{"data:"},
		"mode":            "deterministic_search",
		"query":           "LiFePO4",
		"input_selector":  "#search",
		"submit_selector": "#submit",
		"result_selector": ".result",
		"headed":          false,
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create deterministic browser agent job status = %d body=%s", resp.Code, resp.Body.String())
	}
	var job map[string]any
	decodeJSON(t, resp, &job)
	if job["adapter"] != "generic.browser_agent" {
		t.Fatalf("adapter = %v", job["adapter"])
	}
	input := job["input"].(map[string]any)
	if input["mode"] != "deterministic_search" || input["query"] != "LiFePO4" {
		t.Fatalf("input = %#v", input)
	}
	if input["input_selector"] != "#search" || input["result_selector"] != ".result" {
		t.Fatalf("selectors = %#v", input)
	}
}

func TestCreateWebBrowserActJobRejectsDeterministicMode(t *testing.T) {
	server := NewServer().Handler()

	resp := postJSON(t, server, "/web/automation/browser-act-jobs", map[string]any{
		"url":             "https://example.com",
		"task":            "open example",
		"allowed_domains": []string{"example.com"},
		"mode":            "deterministic_search",
	}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("unsupported browser.act mode status = %d body=%s", resp.Code, resp.Body.String())
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

func pairStrictWorker(t *testing.T, handler http.Handler, tenantID string, userID string, internalSecret string) (string, string) {
	t.Helper()
	pairingResp := postJSON(t, handler, "/worker/devices/pairing", map[string]any{
		"worker_version": "0.1.0",
		"platform":       "windows-amd64",
		"display_name":   "strict test worker",
	}, "")
	if pairingResp.Code != http.StatusCreated {
		t.Fatalf("create strict pairing status = %d body=%s", pairingResp.Code, pairingResp.Body.String())
	}
	var pairing map[string]any
	decodeJSON(t, pairingResp, &pairing)
	pairingID := pairing["pairing_id"].(string)
	pairingCode := pairing["pairing_code"].(string)

	pendingResp := getJSON(t, handler, "/worker/devices/pairing/"+pairingID, "")
	if pendingResp.Code != http.StatusOK {
		t.Fatalf("get pending pairing status = %d body=%s", pendingResp.Code, pendingResp.Body.String())
	}
	var pending map[string]any
	decodeJSON(t, pendingResp, &pending)
	if pending["status"] != "pending" || pending["device_token"] != nil {
		t.Fatalf("strict pairing before approval = %#v", pending)
	}

	approveResp := postJSONWithActor(t, handler, "/web/worker/pairings/"+pairingCode+"/approve", map[string]any{}, tenantID, userID, identity.RoleTenantOwner, internalSecret)
	if approveResp.Code != http.StatusOK {
		t.Fatalf("approve strict pairing status = %d body=%s", approveResp.Code, approveResp.Body.String())
	}

	approvedResp := getJSON(t, handler, "/worker/devices/pairing/"+pairingID, "")
	if approvedResp.Code != http.StatusOK {
		t.Fatalf("get approved pairing status = %d body=%s", approvedResp.Code, approvedResp.Body.String())
	}
	var approved map[string]any
	decodeJSON(t, approvedResp, &approved)
	token, ok := approved["device_token"].(string)
	if !ok || token == "" {
		t.Fatalf("approved pairing missing device token: %#v", approved)
	}
	device := approved["device"].(map[string]any)
	deviceID := device["id"].(string)
	if device["tenant_id"] != tenantID {
		t.Fatalf("device tenant = %v, want %s", device["tenant_id"], tenantID)
	}

	heartbeatResp := postJSON(t, handler, "/worker/devices/"+deviceID+"/heartbeat", map[string]any{
		"worker_version": "0.1.0",
		"status":         "idle",
		"capabilities":   []string{"adapter.mock.echo"},
	}, token)
	if heartbeatResp.Code != http.StatusOK {
		t.Fatalf("strict worker heartbeat status = %d body=%s", heartbeatResp.Code, heartbeatResp.Body.String())
	}
	return token, deviceID
}

func postJSONWithActor(t *testing.T, handler http.Handler, path string, payload map[string]any, tenantID string, userID string, role string, internalSecret string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setActorHeaders(req, tenantID, userID, role, internalSecret)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func getJSONWithActor(t *testing.T, handler http.Handler, path string, tenantID string, userID string, role string, internalSecret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	setActorHeaders(req, tenantID, userID, role, internalSecret)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func setActorHeaders(req *http.Request, tenantID string, userID string, role string, internalSecret string) {
	req.Header.Set(identity.TenantIDHeader, tenantID)
	req.Header.Set(identity.UserIDHeader, userID)
	req.Header.Set(identity.TenantRoleHeader, role)
	if internalSecret != "" {
		req.Header.Set("X-Internal-Secret", internalSecret)
	}
}

func assertSingleTenantJob(t *testing.T, resp *httptest.ResponseRecorder, jobID string, tenantID string) {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("list tenant jobs status = %d body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Jobs []map[string]any `json:"jobs"`
	}
	decodeJSON(t, resp, &payload)
	if len(payload.Jobs) != 1 {
		t.Fatalf("tenant jobs = %#v, want one job", payload.Jobs)
	}
	if payload.Jobs[0]["job_id"] != jobID || payload.Jobs[0]["tenant_id"] != tenantID {
		t.Fatalf("tenant job = %#v, want job %s tenant %s", payload.Jobs[0], jobID, tenantID)
	}
}

func assertSingleTenantDevice(t *testing.T, resp *httptest.ResponseRecorder, deviceID string, tenantID string) {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("list tenant devices status = %d body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Devices []map[string]any `json:"devices"`
	}
	decodeJSON(t, resp, &payload)
	if len(payload.Devices) != 1 {
		t.Fatalf("tenant devices = %#v, want one device", payload.Devices)
	}
	if payload.Devices[0]["id"] != deviceID || payload.Devices[0]["tenant_id"] != tenantID {
		t.Fatalf("tenant device = %#v, want device %s tenant %s", payload.Devices[0], deviceID, tenantID)
	}
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
