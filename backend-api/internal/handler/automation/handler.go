package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	automationengine "qiyuan/backend-api/internal/engine/automation"
	basehandler "qiyuan/backend-api/internal/handler"
	workerhandler "qiyuan/backend-api/internal/handler/worker"
	automationmodel "qiyuan/backend-api/internal/model/automation"
	automationrepo "qiyuan/backend-api/internal/repository/automation"
)

type Handler struct {
	engine      *automationengine.Engine
	workerAuth  *workerhandler.Handler
	artifactDir string
}

func New(engine *automationengine.Engine, workerAuth *workerhandler.Handler, artifactDir string) *Handler {
	if artifactDir == "" {
		artifactDir = "artifacts"
	}
	absArtifactDir, err := filepath.Abs(artifactDir)
	if err == nil {
		artifactDir = absArtifactDir
	}
	return &Handler{engine: engine, workerAuth: workerAuth, artifactDir: artifactDir}
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/automation/jobs", handler.createJob)
	mux.HandleFunc("GET /admin/automation/jobs", handler.listJobs)
	mux.HandleFunc("GET /admin/automation/jobs/{job_id}", handler.job)
	mux.HandleFunc("GET /admin/automation/policy-templates", handler.policyTemplates)
	mux.HandleFunc("GET /web/automation/jobs", handler.listWebJobs)
	mux.HandleFunc("GET /web/automation/jobs/{job_id}/runs", handler.listWebRuns)
	mux.HandleFunc("GET /web/automation/runs/{run_id}/artifacts", handler.webArtifacts)
	mux.HandleFunc("GET /web/automation/reports/copyright-evidence", handler.webCopyrightEvidenceReport)
	mux.HandleFunc("POST /web/automation/browser-agent-jobs", handler.createWebBrowserAgentJob)
	mux.HandleFunc("POST /web/automation/browser-act-jobs", handler.createWebBrowserActJob)
	mux.HandleFunc("GET /admin/automation/runs", handler.listRuns)
	mux.HandleFunc("GET /admin/automation/runs/{run_id}", handler.run)
	mux.HandleFunc("POST /admin/automation/runs/{run_id}/cancel", handler.cancelRun)
	mux.HandleFunc("GET /admin/automation/runs/{run_id}/checkpoints", handler.checkpoints)
	mux.HandleFunc("GET /admin/automation/runs/{run_id}/artifacts", handler.artifacts)
	mux.HandleFunc("GET /admin/automation/runs/{run_id}/trace", handler.trace)
	mux.HandleFunc("GET /admin/automation/artifacts/{artifact_id}/download", handler.downloadArtifact)
	mux.HandleFunc("GET /admin/automation/runs/{run_id}/manual-actions", handler.manualActions)
	mux.HandleFunc("GET /admin/automation/manual-actions", handler.listManualActions)
	mux.HandleFunc("POST /admin/automation/manual-actions/{manual_action_id}/resolve", handler.resolveManualAction)
	mux.HandleFunc("GET /worker/automation/jobs/next", handler.nextJob)
	mux.HandleFunc("GET /worker/automation/runs/{run_id}", handler.workerRun)
	mux.HandleFunc("POST /worker/automation/runs/{run_id}/heartbeat", handler.heartbeat)
	mux.HandleFunc("POST /worker/automation/runs/{run_id}/checkpoint", handler.checkpoint)
	mux.HandleFunc("POST /worker/automation/runs/{run_id}/artifacts", handler.createArtifact)
	mux.HandleFunc("POST /worker/automation/runs/{run_id}/artifact-files", handler.uploadArtifactFile)
	mux.HandleFunc("POST /worker/automation/runs/{run_id}/manual-actions", handler.createManualAction)
	mux.HandleFunc("GET /worker/automation/manual-actions/{manual_action_id}", handler.manualAction)
	mux.HandleFunc("POST /worker/automation/runs/{run_id}/complete", handler.completeRun)
}

func (handler *Handler) webArtifacts(w http.ResponseWriter, r *http.Request) {
	artifacts, err := handler.engine.Artifacts(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"artifacts": artifacts})
}

func (handler *Handler) webCopyrightEvidenceReport(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	adapter := strings.TrimSpace(r.URL.Query().Get("adapter"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	runs, err := handler.engine.ListRuns(automationmodel.ListRunsOptions{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		statusCode, code := mapAutomationError(err)
		basehandler.WriteError(w, statusCode, code, err.Error(), false)
		return
	}

	rows := make([]automationmodel.WebEvidenceRow, 0, len(runs))
	summary := automationmodel.WebEvidenceReportSummary{}
	for _, run := range runs {
		if adapter != "" && run.LastCursor != nil {
			if v, ok := run.LastCursor["source"].(string); ok && v != adapter {
				// skip
				continue
			}
		}
		job, err := handler.engine.Job(run.JobID)
		if err != nil {
			continue
		}
		query, _ := job.Input["query"].(string)
		task, _ := job.Input["task"].(string)
		startURL, _ := job.Input["url"].(string)

		evidenceURL := firstNonEmptyString(
			anyToString(run.Summary["url"]),
			anyToString(run.LastCursor["url"]),
			startURL,
		)
		evidenceTitle := anyToString(run.Summary["title"])
		domain := hostnameOf(evidenceURL)
		domainLocation := "unknown"
		if domain != "" {
			domainLocation = guessDomainLocation(domain)
		}

		artifacts, err := handler.engine.Artifacts(run.ID)
		if err != nil {
			artifacts = nil
		}
		screenshotIDs := make([]string, 0)
		traceID := ""
		for _, artifact := range artifacts {
			switch artifact.ArtifactType {
			case "screenshot":
				screenshotIDs = append(screenshotIDs, artifact.ID)
			case "agent_trace":
				if traceID == "" {
					traceID = artifact.ID
				}
			}
		}
		sort.Strings(screenshotIDs)

		hitKeywords := guessHitKeywords(task, query)

		piracyFound := parsePiracyFound(run.Summary)
		conclusion := buildEvidenceConclusion(piracyFound, evidenceTitle, domain, evidenceURL, startURL)
		summary.TotalRuns++
		switch {
		case piracyFound != nil && *piracyFound:
			summary.Detected++
			summary.Confirmed++
			summary.Highlights = append(summary.Highlights, conclusion)
			summary.Findings = append(summary.Findings, buildFindingLabel("已确认", evidenceTitle, domain, evidenceURL))
		case piracyFound == nil && evidenceURL != "" && evidenceURL != startURL:
			summary.Detected++
			summary.Candidates++
			summary.Highlights = append(summary.Highlights, conclusion)
			summary.Findings = append(summary.Findings, buildFindingLabel("候选命中", evidenceTitle, domain, evidenceURL))
		default:
			summary.Unknown++
		}

		rows = append(rows, automationmodel.WebEvidenceRow{
			RunID:           run.ID,
			JobID:           run.JobID,
			Adapter:         job.Adapter,
			Status:          run.Status,
			CompletedAt:     run.CompletedAt,
			Query:           query,
			Task:            task,
			StartURL:        startURL,
			EvidenceURL:     evidenceURL,
			EvidenceTitle:   evidenceTitle,
			Domain:          domain,
			DomainLocation:  domainLocation,
			HitKeywords:     hitKeywords,
			Notes:           "",
			Conclusion:      conclusion,
			ScreenshotIDs:   screenshotIDs,
			TraceArtifactID: traceID,
			Metadata:        map[string]any{"screenshot_count": len(screenshotIDs)},
			PiracyFound:     piracyFound,
		})
	}
	summary.Conclusion = buildReportConclusion(summary)

	basehandler.WriteJSON(w, http.StatusOK, map[string]any{
		"rows":    rows,
		"summary": summary,
		"limit":   limit,
		"offset":  offset,
	})
}

func parsePiracyFound(summary map[string]any) *bool {
	if summary == nil {
		return nil
	}
	rawExtracts, ok := summary["extracts"].([]any)
	if !ok {
		return nil
	}
	for _, ext := range rawExtracts {
		extractMap, ok := ext.(map[string]any)
		if !ok {
			continue
		}
		fields, ok := extractMap["fields"].(map[string]any)
		if !ok {
			continue
		}
		pf, ok := fields["piracy_found"]
		if !ok {
			continue
		}
		pfStr := strings.ToLower(anyToString(pf))
		val := pfStr == "true" || pfStr == "1" || pfStr == "yes" || pfStr == "found"
		return &val
	}
	return nil
}

func buildEvidenceConclusion(piracyFound *bool, title, domain, evidenceURL, startURL string) string {
	label := firstNonEmptyString(title, domain, evidenceURL)
	if label == "" {
		label = "未识别到命中页面"
	}
	switch {
	case piracyFound != nil && *piracyFound:
		return "确认发现疑似侵权页面: " + label
	case piracyFound == nil && evidenceURL != "" && evidenceURL != startURL:
		return "发现候选页面证据: " + label
	default:
		return "未识别到明确侵权页面"
	}
}

func buildReportConclusion(summary automationmodel.WebEvidenceReportSummary) string {
	switch {
	case summary.TotalRuns == 0:
		return "本次未生成可分析的取证结果。"
	case summary.Confirmed > 0:
		return fmt.Sprintf("本次共发现 %d 个命中项，其中 %d 个已确认疑似侵权。", summary.Detected, summary.Confirmed)
	case summary.Candidates > 0:
		return fmt.Sprintf("本次共发现 %d 个候选命中项，尚未确认明确侵权。", summary.Detected)
	default:
		return "本次未识别到明确侵权页面。"
	}
}

func buildFindingLabel(prefix, title, domain, evidenceURL string) string {
	label := firstNonEmptyString(title, domain, evidenceURL)
	if label == "" {
		label = "未命中页面"
	}
	return prefix + "：" + label
}

func anyToString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func hostnameOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		// 兼容没有 scheme 的输入
		u, err = url.Parse("https://" + raw)
		if err != nil {
			return ""
		}
		host = u.Hostname()
	}
	return strings.ToLower(strings.TrimSpace(host))
}

func guessDomainLocation(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "unknown"
	}
	// IP 直接标记为 ip
	if ip := net.ParseIP(host); ip != nil {
		return "ip"
	}
	// 极简的基于 TLD 的归属地猜测（满足 BRD 演示，后续可接入 whois/geo 服务）
	if strings.HasSuffix(host, ".cn") || strings.HasSuffix(host, ".com.cn") || strings.HasSuffix(host, ".net.cn") {
		return "CN"
	}
	if strings.HasSuffix(host, ".ru") {
		return "RU"
	}
	if strings.HasSuffix(host, ".jp") {
		return "JP"
	}
	if strings.HasSuffix(host, ".kr") {
		return "KR"
	}
	if strings.HasSuffix(host, ".hk") {
		return "HK"
	}
	if strings.HasSuffix(host, ".tw") {
		return "TW"
	}
	if strings.HasSuffix(host, ".uk") {
		return "UK"
	}
	if strings.HasSuffix(host, ".us") {
		return "US"
	}
	return "overseas"
}

func guessHitKeywords(task string, query string) []string {
	text := strings.TrimSpace(task)
	if text == "" {
		text = strings.TrimSpace(query)
	}
	if text == "" {
		return nil
	}
	// 粗略提取：
	// - 引号内关键词
	// - 常见网盘关键词
	var hits []string
	re := regexp.MustCompile(`"([^"]{1,64})"|“([^”]{1,64})”`)
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		for i := 1; i < len(match); i++ {
			kw := strings.TrimSpace(match[i])
			if kw != "" {
				hits = append(hits, kw)
			}
		}
	}
	for _, kw := range []string{"网盘", "百度网盘", "夸克", "提取码", "完整版", "全集", "免费观看", "在线播放"} {
		if strings.Contains(text, kw) {
			hits = append(hits, kw)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	// 去重
	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(hits))
	for _, h := range hits {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		uniq = append(uniq, h)
	}
	sort.Strings(uniq)
	return uniq
}

func (handler *Handler) policyTemplates(w http.ResponseWriter, r *http.Request) {
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"templates": policyTemplates()})
}

func (handler *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req automationmodel.CreateJobRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	if req.JobType == "" || req.Adapter == "" {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JOB", "job_type and adapter are required", false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, handler.engine.CreateJob(req))
}

func (handler *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	jobs, err := handler.engine.ListJobs(automationmodel.ListJobsOptions{
		Status:  r.URL.Query().Get("status"),
		Adapter: r.URL.Query().Get("adapter"),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "limit": limit, "offset": offset})
}

func (handler *Handler) listWebJobs(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	jobs, err := handler.engine.ListJobs(automationmodel.ListJobsOptions{
		Status:  r.URL.Query().Get("status"),
		Adapter: r.URL.Query().Get("adapter"),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "limit": limit, "offset": offset})
}

func (handler *Handler) listWebRuns(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	limit, offset := limitOffset(r)
	runs, err := handler.engine.ListRuns(automationmodel.ListRunsOptions{
		JobID:  jobID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"runs": runs, "limit": limit, "offset": offset})
}

func (handler *Handler) createWebBrowserAgentJob(w http.ResponseWriter, r *http.Request) {
	handler.createBrowserJob(w, r, browserJobRequestConfig{
		jobType:        "generic.browser.agent",
		adapter:        "generic.browser_agent",
		defaultMode:    "llm_plan",
		allowedActions:  []string{"observe_page", "fill", "click", "click_element", "press", "extract", "screenshot", "wait_for"},
		allowDownloadAction: true,
	})
}

func (handler *Handler) createWebBrowserActJob(w http.ResponseWriter, r *http.Request) {
	handler.createBrowserJob(w, r, browserJobRequestConfig{
		jobType:        "generic.browser.act",
		adapter:        "browser.act",
		defaultMode:    "cli",
		allowedActions: []string{},
	})
}

type browserJobRequestConfig struct {
	jobType             string
	adapter             string
	defaultMode         string
	allowedActions      []string
	allowDownloadAction bool
}

func (handler *Handler) createBrowserJob(w http.ResponseWriter, r *http.Request, cfg browserJobRequestConfig) {
	var req struct {
		URL              string   `json:"url"`
		Task             string   `json:"task"`
		AllowedDomains   []string `json:"allowed_domains"`
		AllowDownload    bool     `json:"allow_download"`
		Headed           *bool    `json:"headed"`
		ActionTimeoutSec int      `json:"action_timeout_seconds"`
	}
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	startURL := strings.TrimSpace(req.URL)
	task := strings.TrimSpace(req.Task)
	if startURL == "" || task == "" {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_AGENT_JOB", "url and task are required", false)
		return
	}
	allowedDomains := normalizeDomains(req.AllowedDomains)
	if len(allowedDomains) == 0 {
		basehandler.WriteError(w, http.StatusBadRequest, "ALLOWED_DOMAINS_REQUIRED", "allowed_domains is required", false)
		return
	}
	headed := true
	if req.Headed != nil {
		headed = *req.Headed
	}
	timeout := req.ActionTimeoutSec
	if timeout <= 0 {
		timeout = 120
	}
	if timeout > 120 {
		timeout = 120
	}
	allowedActions := append([]string{}, cfg.allowedActions...)
	if cfg.allowDownloadAction && req.AllowDownload {
		allowedActions = append(allowedActions, "download")
	}
	input := map[string]any{
		"url":  startURL,
		"task": task,
		"mode": cfg.defaultMode,
	}
	if cfg.defaultMode == "llm_plan" {
		input["query"] = task
	}
	job := handler.engine.CreateJob(automationmodel.CreateJobRequest{
		JobType: cfg.jobType,
		Adapter: cfg.adapter,
		Target: map[string]any{
			"allowed_domains": allowedDomains,
		},
		Input: input,
		Policy: map[string]any{
			"headed":                 headed,
			"allowed_actions":        allowedActions,
			"allowed_domains":        allowedDomains,
			"action_timeout_seconds": timeout,
			"max_download_bytes":     25 * 1024 * 1024,
		},
		Priority: int(time.Now().Unix()),
	})
	basehandler.WriteJSON(w, http.StatusCreated, job)
}

func (handler *Handler) job(w http.ResponseWriter, r *http.Request) {
	job, err := handler.engine.Job(r.PathValue("job_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, job)
}

func (handler *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	runs, err := handler.engine.ListRuns(automationmodel.ListRunsOptions{
		Status:   r.URL.Query().Get("status"),
		JobID:    r.URL.Query().Get("job_id"),
		DeviceID: r.URL.Query().Get("device_id"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"runs": runs, "limit": limit, "offset": offset})
}

func (handler *Handler) run(w http.ResponseWriter, r *http.Request) {
	run, err := handler.engine.Run(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "cancelled by admin"
	}
	run, err := handler.engine.CancelRun(r.PathValue("run_id"), reason)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) checkpoints(w http.ResponseWriter, r *http.Request) {
	checkpoints, err := handler.engine.Checkpoints(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"checkpoints": checkpoints})
}

func (handler *Handler) artifacts(w http.ResponseWriter, r *http.Request) {
	artifacts, err := handler.engine.Artifacts(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"artifacts": artifacts})
}

func (handler *Handler) trace(w http.ResponseWriter, r *http.Request) {
	artifacts, err := handler.engine.Artifacts(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	for _, artifact := range artifacts {
		if artifact.ArtifactType != "agent_trace" {
			continue
		}
		path, ok := handler.safeArtifactPath(artifact.LocalPath)
		if !ok {
			break
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			basehandler.WriteError(w, http.StatusNotFound, "TRACE_FILE_NOT_FOUND", err.Error(), false)
			return
		}
		var trace any
		if err := json.Unmarshal(raw, &trace); err != nil {
			basehandler.WriteError(w, http.StatusInternalServerError, "TRACE_JSON_INVALID", err.Error(), false)
			return
		}
		basehandler.WriteJSON(w, http.StatusOK, map[string]any{"artifact": artifact, "trace": trace})
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"trace": map[string]any{"steps": []any{}}, "artifact": nil})
}

func (handler *Handler) downloadArtifact(w http.ResponseWriter, r *http.Request) {
	artifact, err := handler.engine.Artifact(r.PathValue("artifact_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	path, ok := handler.safeArtifactPath(artifact.LocalPath)
	if !ok {
		basehandler.WriteError(w, http.StatusNotFound, "ARTIFACT_FILE_NOT_FOUND", "artifact file is not available locally", false)
		return
	}
	if _, err := os.Stat(path); err != nil {
		basehandler.WriteError(w, http.StatusNotFound, "ARTIFACT_FILE_NOT_FOUND", err.Error(), false)
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", "inline; filename="+strconv.Quote(filepath.Base(path)))
	http.ServeFile(w, r, path)
}

func (handler *Handler) manualActions(w http.ResponseWriter, r *http.Request) {
	actions, err := handler.engine.ManualActions(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"manual_actions": actions})
}

func (handler *Handler) listManualActions(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	actions, err := handler.engine.ListManualActions(automationmodel.ListManualActionsOptions{
		Status: r.URL.Query().Get("status"),
		RunID:  r.URL.Query().Get("run_id"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"manual_actions": actions, "limit": limit, "offset": offset})
}

func (handler *Handler) resolveManualAction(w http.ResponseWriter, r *http.Request) {
	var req automationmodel.ResolveManualActionRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	action, err := handler.engine.ResolveManualAction(r.PathValue("manual_action_id"), req)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, action)
}

func (handler *Handler) nextJob(w http.ResponseWriter, r *http.Request) {
	device, ok := handler.workerAuth.AuthenticatedDevice(r)
	if !ok {
		basehandler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid device token", false)
		return
	}
	job, err := handler.engine.NextJob(device)
	if err != nil {
		if errors.Is(err, automationrepo.ErrNoJobAvailable) {
			basehandler.WriteJSON(w, http.StatusNoContent, nil)
			return
		}
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, job)
}

func (handler *Handler) workerRun(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	run, err := handler.engine.Run(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	var req automationmodel.HeartbeatRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	run, err := handler.engine.Heartbeat(r.PathValue("run_id"), req)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) checkpoint(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	var checkpoint automationmodel.Checkpoint
	if err := basehandler.DecodeJSON(r, &checkpoint); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	stored, err := handler.engine.Checkpoint(r.PathValue("run_id"), checkpoint)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, stored)
}

func (handler *Handler) createArtifact(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	var artifact automationmodel.Artifact
	if err := basehandler.DecodeJSON(r, &artifact); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	stored, err := handler.engine.CreateArtifact(r.PathValue("run_id"), artifact)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, stored)
}

func (handler *Handler) uploadArtifactFile(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_MULTIPART", err.Error(), false)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "FILE_REQUIRED", "multipart field file is required", false)
		return
	}
	defer file.Close()

	artifactType := strings.TrimSpace(r.FormValue("artifact_type"))
	if artifactType == "" {
		artifactType = "file"
	}
	metadata := parseMetadataFormValue(r.FormValue("metadata"))
	storedPath, sha, size, err := handler.saveArtifactFile(r.PathValue("run_id"), header.Filename, file)
	if err != nil {
		basehandler.WriteError(w, http.StatusInternalServerError, "ARTIFACT_SAVE_FAILED", err.Error(), true)
		return
	}
	metadata["filename"] = header.Filename
	metadata["content_type"] = header.Header.Get("Content-Type")

	artifact, err := handler.engine.CreateArtifact(
		r.PathValue("run_id"),
		automationmodel.Artifact{
			ArtifactType: artifactType,
			LocalPath:    storedPath,
			Metadata:     metadata,
			SHA256:       sha,
			SizeBytes:    &size,
		},
	)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, artifact)
}

func (handler *Handler) createManualAction(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	var action automationmodel.ManualAction
	if err := basehandler.DecodeJSON(r, &action); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	stored, err := handler.engine.CreateManualAction(r.PathValue("run_id"), action)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, stored)
}

func (handler *Handler) manualAction(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	action, err := handler.engine.ManualAction(r.PathValue("manual_action_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, action)
}

func (handler *Handler) completeRun(w http.ResponseWriter, r *http.Request) {
	if !handler.authorized(w, r) {
		return
	}
	var req automationmodel.CompleteRunRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	run, err := handler.engine.CompleteRun(r.PathValue("run_id"), req)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}

	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) authorized(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := handler.workerAuth.AuthenticatedDevice(r); !ok {
		basehandler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid device token", false)
		return false
	}
	return true
}

func mapAutomationError(err error) (int, string) {
	switch {
	case errors.Is(err, automationrepo.ErrJobNotFound),
		errors.Is(err, automationrepo.ErrRunNotFound),
		errors.Is(err, automationrepo.ErrArtifactNotFound),
		errors.Is(err, automationrepo.ErrManualActionNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, automationrepo.ErrActiveRunExists):
		return http.StatusConflict, "ACTIVE_RUN_EXISTS"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

func limitOffset(r *http.Request) (int, int) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
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

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseMetadataFormValue(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return map[string]any{"raw_metadata": raw}
	}
	return metadata
}

func normalizeDomains(values []string) []string {
	domains := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		domain := strings.ToLower(strings.TrimSpace(value))
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimSuffix(domain, "/")
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		domains = append(domains, domain)
	}
	return domains
}

func policyTemplates() []automationmodel.PolicyTemplate {
	return []automationmodel.PolicyTemplate{
		{
			Name:        "browser_agent.generic.llm_plan",
			ProductLine: "browser_agent",
			JobType:     "generic.browser.agent",
			Adapter:     "generic.browser_agent",
			Target:      map[string]any{"allowed_domains": []string{}},
			Policy: map[string]any{
				"allowed_actions":        []string{"observe_page", "click", "click_element", "fill", "press", "extract", "screenshot", "wait_for"},
				"action_timeout_seconds": 30,
			},
		},
		{
			Name:        "browser_agent.generic.download",
			ProductLine: "browser_agent",
			JobType:     "generic.browser.agent",
			Adapter:     "generic.browser_agent",
			Target:      map[string]any{"allowed_domains": []string{}},
			Policy: map[string]any{
				"allowed_actions":        []string{"observe_page", "click", "click_element", "fill", "press", "extract", "screenshot", "wait_for", "download"},
				"action_timeout_seconds": 30,
				"max_download_bytes":     25 * 1024 * 1024,
			},
		},
		{
			Name:        "social.youtube.upload_video.draft",
			ProductLine: "social",
			JobType:     "social.youtube.upload_video",
			Adapter:     "social.youtube.upload_video",
			Target:      map[string]any{"allowed_domains": []string{"studio.youtube.com", "*.youtube.com"}},
			Policy: map[string]any{
				"allowed_actions":         []string{"observe_page", "screenshot", "wait_for"},
				"manual_publish_required": true,
			},
		},
		{
			Name:        "social.tiktok.upload_video.draft",
			ProductLine: "social",
			JobType:     "social.tiktok.upload_video",
			Adapter:     "social.tiktok.upload_video",
			Target:      map[string]any{"allowed_domains": []string{"www.tiktok.com", "*.tiktok.com"}},
			Policy: map[string]any{
				"allowed_actions":         []string{"observe_page", "screenshot", "wait_for"},
				"manual_publish_required": true,
			},
		},
	}
}

func (handler *Handler) saveArtifactFile(runID string, filename string, file io.Reader) (string, string, int64, error) {
	cleanRunID := safePathSegment(runID)
	cleanFilename := safeFilename(filename)
	dir := filepath.Join(handler.artifactDir, cleanRunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", 0, err
	}
	path := filepath.Join(dir, cleanFilename)
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(cleanFilename)
		base := strings.TrimSuffix(cleanFilename, ext)
		path = filepath.Join(dir, base+"-"+strconv.FormatInt(time.Now().UnixNano(), 10)+ext)
	}
	output, err := os.Create(path)
	if err != nil {
		return "", "", 0, err
	}
	defer output.Close()

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(output, hash), file)
	if err != nil {
		return "", "", 0, err
	}
	return path, hex.EncodeToString(hash.Sum(nil)), size, nil
}

func (handler *Handler) safeArtifactPath(rawPath string) (string, bool) {
	if rawPath == "" {
		return "", false
	}
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", false
	}
	absRoot, err := filepath.Abs(handler.artifactDir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", false
	}
	return absPath, true
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '-'
	}, value)
}

func safeFilename(value string) string {
	name := filepath.Base(strings.TrimSpace(value))
	if name == "." || name == "/" || name == "" {
		name = "artifact.bin"
	}
	return safePathSegment(name)
}

func stringFromMetadata(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func stringSliceFromMetadata(metadata map[string]any, key string) []string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				values = append(values, text)
			}
		}
		return values
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	default:
		return nil
	}
}

func intPtrFromMetadata(metadata map[string]any, key string) *int {
	value, ok := metadata[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case int:
		return &typed
	case float64:
		n := int(typed)
		return &n
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return &n
		}
	}
	return nil
}
