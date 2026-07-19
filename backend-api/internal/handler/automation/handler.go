package automation

import (
	"context"
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
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	automationengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/automation"
	basehandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler"
	workerhandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler/worker"
	"github.com/Lucy-97/browser-agent/backend-api/internal/identity"
	automationmodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/automation"
	workermodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/worker"
	automationrepo "github.com/Lucy-97/browser-agent/backend-api/internal/repository/automation"
	artifactstore "github.com/Lucy-97/browser-agent/backend-api/internal/storage/artifact"
)

type Handler struct {
	engine     *automationengine.Engine
	workerAuth *workerhandler.Handler
	store      artifactstore.Store
}

func New(engine *automationengine.Engine, workerAuth *workerhandler.Handler, store artifactstore.Store) *Handler {
	if store == nil {
		panic("artifact store is required")
	}
	return &Handler{engine: engine, workerAuth: workerAuth, store: store}
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
	mux.HandleFunc("POST /web/automation/social-upload-jobs", handler.createWebSocialUploadJob)
	mux.HandleFunc("POST /web/automation/weixin-desktop-sync-jobs", handler.createWebWeixinDesktopSyncJob)
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
	if _, ok := handler.tenantRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
	artifacts, err := handler.engine.Artifacts(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"artifacts": artifacts})
}

func (handler *Handler) webCopyrightEvidenceReport(w http.ResponseWriter, r *http.Request) {
	actor, ok := requestActor(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r)
	adapter := strings.TrimSpace(r.URL.Query().Get("adapter"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	runs, err := handler.engine.ListRuns(automationmodel.ListRunsOptions{
		TenantID: actor.TenantID,
		Status:   status,
		Limit:    limit,
		Offset:   offset,
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
		if err != nil || job.TenantID != actor.TenantID {
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
	job, ok := handler.createOwnedJob(w, r, req)
	if !ok {
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, job)
}

func (handler *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	actor, ok := requestActor(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r)
	jobs, err := handler.engine.ListJobs(automationmodel.ListJobsOptions{
		TenantID: actor.TenantID,
		Status:   r.URL.Query().Get("status"),
		Adapter:  r.URL.Query().Get("adapter"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "limit": limit, "offset": offset})
}

func (handler *Handler) listWebJobs(w http.ResponseWriter, r *http.Request) {
	actor, ok := requestActor(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r)
	jobs, err := handler.engine.ListJobs(automationmodel.ListJobsOptions{
		TenantID: actor.TenantID,
		Status:   r.URL.Query().Get("status"),
		Adapter:  r.URL.Query().Get("adapter"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "limit": limit, "offset": offset})
}

func (handler *Handler) listWebRuns(w http.ResponseWriter, r *http.Request) {
	actor, ok := requestActor(w, r)
	if !ok {
		return
	}
	jobID := r.PathValue("job_id")
	limit, offset := limitOffset(r)
	runs, err := handler.engine.ListRuns(automationmodel.ListRunsOptions{
		TenantID: actor.TenantID,
		JobID:    jobID,
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

func (handler *Handler) createWebBrowserAgentJob(w http.ResponseWriter, r *http.Request) {
	handler.createBrowserJob(w, r, browserJobRequestConfig{
		jobType:             "generic.browser.agent",
		adapter:             "generic.browser_agent",
		defaultMode:         "llm_plan",
		allowDeterministic:  true,
		allowedActions:      []string{"observe_page", "fill", "click", "click_element", "press", "extract", "screenshot", "wait_for"},
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

func (handler *Handler) createWebSocialUploadJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform              string   `json:"platform"`
		VideoPath             string   `json:"video_path"`
		ArtifactID            string   `json:"artifact_id"`
		Title                 string   `json:"title"`
		Description           string   `json:"description"`
		Tags                  []string `json:"tags"`
		Headed                *bool    `json:"headed"`
		ManualPublishRequired *bool    `json:"manual_publish_required"`
	}
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	config, ok := socialUploadPlatformConfig(platform)
	if !ok {
		basehandler.WriteError(w, http.StatusBadRequest, "SOCIAL_PLATFORM_UNSUPPORTED", "platform must be one of: youtube, tiktok, instagram", false)
		return
	}
	videoPath := strings.TrimSpace(req.VideoPath)
	artifactID := strings.TrimSpace(req.ArtifactID)
	if videoPath == "" && artifactID == "" {
		basehandler.WriteError(w, http.StatusBadRequest, "SOCIAL_VIDEO_REQUIRED", "video_path or artifact_id is required", false)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		basehandler.WriteError(w, http.StatusBadRequest, "SOCIAL_TITLE_REQUIRED", "title is required", false)
		return
	}
	tags := normalizeTags(req.Tags)
	headed := true
	if req.Headed != nil {
		headed = *req.Headed
	}
	manualPublishRequired := false

	job, created := handler.createOwnedJob(w, r, automationmodel.CreateJobRequest{
		JobType: config.jobType,
		Adapter: config.adapter,
		Target: map[string]any{
			"allowed_domains": config.allowedDomains,
		},
		Input: map[string]any{
			"video_path":  videoPath,
			"artifact_id": artifactID,
			"title":       title,
			"description": strings.TrimSpace(req.Description),
			"tags":        tags,
			"source":      "web_social_upload",
			"platform":    platform,
		},
		Policy: map[string]any{
			"headed":                  headed,
			"allowed_domains":         config.allowedDomains,
			"allowed_actions":         []string{"observe_page", "screenshot", "wait_for"},
			"manual_publish_required": manualPublishRequired,
		},
		Priority: int(time.Now().Unix()),
	})
	if !created {
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, job)
}

func (handler *Handler) createWebWeixinDesktopSyncJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceDirs     []string                     `json:"source_dirs"`
		GroupNames     []string                     `json:"group_names"`
		SelectedGroups []weixinSelectedGroupRequest `json:"selected_groups"`
		GroupKeywords  []string                     `json:"group_keywords"`
		MaxFiles       int                          `json:"max_files"`
		MaxFileBytes   int                          `json:"max_file_bytes"`
	}
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	sourceDirs := normalizeNonEmptyStrings(req.SourceDirs)
	if len(sourceDirs) == 0 {
		basehandler.WriteError(w, http.StatusBadRequest, "WEIXIN_SOURCE_DIRS_REQUIRED", "source_dirs is required", false)
		return
	}
	groupNames := normalizeWeixinGroupNames(req.GroupNames, req.SelectedGroups, req.GroupKeywords)
	if len(groupNames) == 0 {
		basehandler.WriteError(w, http.StatusBadRequest, "WEIXIN_GROUPS_REQUIRED", "group_names is required", false)
		return
	}
	maxFiles := req.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 200
	}
	if maxFiles > 1000 {
		maxFiles = 1000
	}
	maxFileBytes := req.MaxFileBytes
	if maxFileBytes <= 0 {
		maxFileBytes = 100 * 1024 * 1024
	}
	if maxFileBytes > 1024*1024*1024 {
		maxFileBytes = 1024 * 1024 * 1024
	}

	job, created := handler.createOwnedJob(w, r, automationmodel.CreateJobRequest{
		JobType: "weixin.desktop_sync",
		Adapter: "weixin.desktop_sync",
		Target: map[string]any{
			"source": "desktop_weixin",
		},
		Input: map[string]any{
			"source_dirs":     sourceDirs,
			"group_names":     groupNames,
			"selected_groups": normalizeWeixinSelectedGroups(req.SelectedGroups, groupNames),
			"group_keywords":  groupNames,
			"source":          "web_weixin_desktop_sync",
			"sync_mode":       "desktop_file_scan",
			"requires_local":  true,
		},
		Policy: map[string]any{
			"max_files":      maxFiles,
			"max_file_bytes": maxFileBytes,
		},
		Priority: int(time.Now().Unix()),
	})
	if !created {
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, job)
}

type weixinSelectedGroupRequest struct {
	GroupID     string `json:"group_id"`
	DisplayName string `json:"display_name"`
}

type normalizedWeixinSelectedGroup struct {
	GroupID     string `json:"group_id,omitempty"`
	DisplayName string `json:"display_name"`
}

func normalizeWeixinGroupNames(groupNames []string, selectedGroups []weixinSelectedGroupRequest, legacyKeywords []string) []string {
	values := normalizeNonEmptyStrings(groupNames)
	for _, group := range selectedGroups {
		values = append(values, strings.TrimSpace(group.DisplayName))
	}
	if len(values) == 0 {
		values = append(values, normalizeNonEmptyStrings(legacyKeywords)...)
	}
	return uniqueNonEmptyStrings(values)
}

func normalizeWeixinSelectedGroups(selectedGroups []weixinSelectedGroupRequest, groupNames []string) []normalizedWeixinSelectedGroup {
	groups := make([]normalizedWeixinSelectedGroup, 0, len(selectedGroups)+len(groupNames))
	seen := map[string]bool{}
	for _, group := range selectedGroups {
		name := strings.TrimSpace(group.DisplayName)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		groups = append(groups, normalizedWeixinSelectedGroup{
			GroupID:     strings.TrimSpace(group.GroupID),
			DisplayName: name,
		})
	}
	for _, name := range groupNames {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		groups = append(groups, normalizedWeixinSelectedGroup{DisplayName: name})
	}
	return groups
}

func uniqueNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

type browserJobRequestConfig struct {
	jobType             string
	adapter             string
	defaultMode         string
	allowDeterministic  bool
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
		Mode             string   `json:"mode"`
		Query            string   `json:"query"`
		InputSelector    string   `json:"input_selector"`
		SubmitSelector   string   `json:"submit_selector"`
		ResultSelector   string   `json:"result_selector"`
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
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = cfg.defaultMode
	}
	if mode != cfg.defaultMode && !(cfg.allowDeterministic && mode == "deterministic_search") {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_AGENT_MODE", "mode is not supported for this endpoint", false)
		return
	}
	allowedActions := append([]string{}, cfg.allowedActions...)
	if cfg.allowDownloadAction && req.AllowDownload {
		allowedActions = append(allowedActions, "download")
	}
	input := map[string]any{
		"url":  startURL,
		"task": task,
		"mode": mode,
	}
	if mode == "llm_plan" {
		input["query"] = task
	} else {
		query := strings.TrimSpace(req.Query)
		if query == "" {
			query = task
		}
		input["query"] = query
		if value := strings.TrimSpace(req.InputSelector); value != "" {
			input["input_selector"] = value
		}
		if value := strings.TrimSpace(req.SubmitSelector); value != "" {
			input["submit_selector"] = value
		}
		if value := strings.TrimSpace(req.ResultSelector); value != "" {
			input["result_selector"] = value
		}
	}
	job, created := handler.createOwnedJob(w, r, automationmodel.CreateJobRequest{
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
	if !created {
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, job)
}

func (handler *Handler) job(w http.ResponseWriter, r *http.Request) {
	job, err := handler.engine.Job(r.PathValue("job_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	if !requestOwnsTenant(w, r, job.TenantID) {
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, job)
}

func (handler *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	actor, ok := requestActor(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r)
	runs, err := handler.engine.ListRuns(automationmodel.ListRunsOptions{
		TenantID: actor.TenantID,
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
	run, ok := handler.tenantRun(w, r, r.PathValue("run_id"))
	if !ok {
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	if _, ok := handler.tenantRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
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
	if _, ok := handler.tenantRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
	checkpoints, err := handler.engine.Checkpoints(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"checkpoints": checkpoints})
}

func (handler *Handler) artifacts(w http.ResponseWriter, r *http.Request) {
	if _, ok := handler.tenantRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
	artifacts, err := handler.engine.Artifacts(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"artifacts": artifacts})
}

func (handler *Handler) trace(w http.ResponseWriter, r *http.Request) {
	if _, ok := handler.tenantRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
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
		if artifact.StorageKey == "" {
			break
		}
		object, err := handler.store.Get(r.Context(), artifact.StorageKey, "")
		if err != nil {
			status, retryable := artifactReadStatus(err)
			basehandler.WriteError(w, status, "TRACE_FILE_NOT_FOUND", err.Error(), retryable)
			return
		}
		defer object.Body.Close()
		const maxTraceBytes = 16 << 20
		raw, err := io.ReadAll(io.LimitReader(object.Body, maxTraceBytes+1))
		if err != nil {
			basehandler.WriteError(w, http.StatusBadGateway, "TRACE_READ_FAILED", err.Error(), true)
			return
		}
		if len(raw) > maxTraceBytes {
			basehandler.WriteError(w, http.StatusRequestEntityTooLarge, "TRACE_FILE_TOO_LARGE", "trace artifact exceeds 16 MiB", false)
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
	if !requestOwnsTenant(w, r, artifact.TenantID) {
		return
	}
	if artifact.StorageKey == "" {
		basehandler.WriteError(w, http.StatusNotFound, "ARTIFACT_FILE_NOT_FOUND", "artifact file is not available", false)
		return
	}
	object, err := handler.store.Get(r.Context(), artifact.StorageKey, r.Header.Get("Range"))
	if err != nil {
		status, retryable := artifactReadStatus(err)
		code := "ARTIFACT_STORAGE_UNAVAILABLE"
		if status == http.StatusNotFound {
			code = "ARTIFACT_FILE_NOT_FOUND"
		} else if status == http.StatusRequestedRangeNotSatisfiable {
			code = "INVALID_ARTIFACT_RANGE"
		}
		basehandler.WriteError(w, status, code, err.Error(), retryable)
		return
	}
	defer object.Body.Close()

	filename := artifactstore.SanitizeFilename(artifact.Filename)
	contentType := artifact.ContentType
	if contentType == "" {
		contentType = object.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Accept-Ranges", "bytes")
	if readSeeker, ok := object.Body.(io.ReadSeeker); ok {
		http.ServeContent(w, r, filename, object.LastModified, readSeeker)
		return
	}
	if object.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(object.ContentLength, 10))
	}
	if object.ContentRange != "" {
		w.Header().Set("Content-Range", object.ContentRange)
		w.WriteHeader(http.StatusPartialContent)
	}
	_, _ = io.Copy(w, object.Body)
}

func (handler *Handler) manualActions(w http.ResponseWriter, r *http.Request) {
	if _, ok := handler.tenantRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
	actions, err := handler.engine.ManualActions(r.PathValue("run_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"manual_actions": actions})
}

func (handler *Handler) listManualActions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requestActor(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r)
	actions, err := handler.engine.ListManualActions(automationmodel.ListManualActionsOptions{
		TenantID: actor.TenantID,
		Status:   r.URL.Query().Get("status"),
		RunID:    r.URL.Query().Get("run_id"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"manual_actions": actions, "limit": limit, "offset": offset})
}

func (handler *Handler) resolveManualAction(w http.ResponseWriter, r *http.Request) {
	action, err := handler.engine.ManualAction(r.PathValue("manual_action_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	if !requestOwnsTenant(w, r, action.TenantID) {
		return
	}
	var req automationmodel.ResolveManualActionRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	action, err = handler.engine.ResolveManualAction(r.PathValue("manual_action_id"), req)
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
	_, run, ok := handler.authorizedRun(w, r, r.PathValue("run_id"))
	if !ok {
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, run)
}

func (handler *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := handler.authorizedRun(w, r, r.PathValue("run_id")); !ok {
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
	if _, _, ok := handler.authorizedRun(w, r, r.PathValue("run_id")); !ok {
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
	if _, _, ok := handler.authorizedRun(w, r, r.PathValue("run_id")); !ok {
		return
	}
	var artifact automationmodel.Artifact
	if err := basehandler.DecodeJSON(r, &artifact); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	artifact.StorageKey = ""
	artifact.Filename = ""
	artifact.ContentType = ""
	artifact.SHA256 = ""
	artifact.SizeBytes = nil
	stored, err := handler.engine.CreateArtifact(r.PathValue("run_id"), artifact)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, stored)
}

func (handler *Handler) uploadArtifactFile(w http.ResponseWriter, r *http.Request) {
	_, run, ok := handler.authorizedRun(w, r, r.PathValue("run_id"))
	if !ok {
		return
	}
	const maxArtifactBytes int64 = 512 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxArtifactBytes+(2<<20))
	multipartReader, err := r.MultipartReader()
	if err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_MULTIPART", err.Error(), false)
		return
	}

	var (
		artifactType string
		metadataRaw  string
		storedObject artifactstore.StoredObject
		filename     string
		contentType  string
		hash         = sha256.New()
		counter      *countingReader
		fileStored   bool
	)
	cleanupStoredObject := func() {
		if !fileStored {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = handler.store.Delete(cleanupCtx, storedObject.Key)
	}

	for {
		part, nextErr := multipartReader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			cleanupStoredObject()
			basehandler.WriteError(w, http.StatusBadRequest, "INVALID_MULTIPART", nextErr.Error(), false)
			return
		}
		formName := part.FormName()
		switch formName {
		case "artifact_type", "metadata":
			raw, readErr := io.ReadAll(io.LimitReader(part, (1<<20)+1))
			part.Close()
			if readErr != nil || len(raw) > 1<<20 {
				cleanupStoredObject()
				basehandler.WriteError(w, http.StatusBadRequest, "INVALID_MULTIPART_FIELD", "artifact metadata field exceeds 1 MiB", false)
				return
			}
			if formName == "artifact_type" {
				artifactType = strings.TrimSpace(string(raw))
			} else {
				metadataRaw = string(raw)
			}
		case "file":
			if fileStored {
				part.Close()
				cleanupStoredObject()
				basehandler.WriteError(w, http.StatusBadRequest, "MULTIPLE_FILES_NOT_ALLOWED", "only one artifact file is allowed", false)
				return
			}
			filename = artifactstore.SanitizeFilename(part.FileName())
			contentType = normalizedContentType(part.Header.Get("Content-Type"), filename)
			counter = &countingReader{reader: io.TeeReader(io.LimitReader(part, maxArtifactBytes+1), hash)}
			storedObject, err = handler.store.Put(r.Context(), artifactstore.PutInput{
				TenantID:    run.TenantID,
				RunID:       run.ID,
				Filename:    filename,
				ContentType: contentType,
				SizeBytes:   -1,
				Body:        counter,
			})
			part.Close()
			if err != nil {
				var maxBytesError *http.MaxBytesError
				if errors.As(err, &maxBytesError) {
					basehandler.WriteError(w, http.StatusRequestEntityTooLarge, "ARTIFACT_TOO_LARGE", "artifact file exceeds 512 MiB", false)
					return
				}
				basehandler.WriteError(w, http.StatusBadGateway, "ARTIFACT_SAVE_FAILED", err.Error(), true)
				return
			}
			fileStored = true
			if counter.count > maxArtifactBytes {
				cleanupStoredObject()
				basehandler.WriteError(w, http.StatusRequestEntityTooLarge, "ARTIFACT_TOO_LARGE", "artifact file exceeds 512 MiB", false)
				return
			}
		default:
			part.Close()
		}
	}
	if !fileStored {
		basehandler.WriteError(w, http.StatusBadRequest, "FILE_REQUIRED", "multipart field file is required", false)
		return
	}
	if artifactType == "" {
		artifactType = "file"
	}
	metadata := parseMetadataFormValue(metadataRaw)
	metadata["filename"] = filename
	metadata["content_type"] = contentType
	sha := hex.EncodeToString(hash.Sum(nil))
	size := counter.count

	artifact, err := handler.engine.CreateArtifact(
		r.PathValue("run_id"),
		automationmodel.Artifact{
			ArtifactType: artifactType,
			StorageKey:   storedObject.Key,
			Filename:     filename,
			ContentType:  contentType,
			Metadata:     metadata,
			SHA256:       sha,
			SizeBytes:    &size,
		},
	)
	if err != nil {
		cleanupStoredObject()
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, artifact)
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	read, err := reader.reader.Read(buffer)
	reader.count += int64(read)
	return read, err
}

func (handler *Handler) createManualAction(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := handler.authorizedRun(w, r, r.PathValue("run_id")); !ok {
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
	action, err := handler.engine.ManualAction(r.PathValue("manual_action_id"))
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	if _, _, ok := handler.authorizedRun(w, r, action.RunID); !ok {
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, action)
}

func (handler *Handler) completeRun(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := handler.authorizedRun(w, r, r.PathValue("run_id")); !ok {
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

func (handler *Handler) authorizedRun(w http.ResponseWriter, r *http.Request, runID string) (workermodel.Device, automationmodel.Run, bool) {
	device, ok := handler.workerAuth.AuthenticatedDevice(r)
	if !ok {
		basehandler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid device token", false)
		return workermodel.Device{}, automationmodel.Run{}, false
	}
	run, err := handler.engine.Run(runID)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return workermodel.Device{}, automationmodel.Run{}, false
	}
	if run.TenantID != device.TenantID || run.DeviceID != device.ID {
		basehandler.WriteError(w, http.StatusNotFound, "NOT_FOUND", "automation run not found", false)
		return workermodel.Device{}, automationmodel.Run{}, false
	}
	return device, run, true
}

func (handler *Handler) createOwnedJob(w http.ResponseWriter, r *http.Request, req automationmodel.CreateJobRequest) (automationmodel.Job, bool) {
	actor, ok := requestActor(w, r)
	if !ok {
		return automationmodel.Job{}, false
	}
	if !actor.CanWriteTenantResources() {
		basehandler.WriteError(w, http.StatusForbidden, "TENANT_WRITE_FORBIDDEN", "tenant role cannot create automation jobs", false)
		return automationmodel.Job{}, false
	}
	req.TenantID = actor.TenantID
	req.UserID = actor.UserID
	return handler.engine.CreateJob(req), true
}

func (handler *Handler) tenantRun(w http.ResponseWriter, r *http.Request, runID string) (automationmodel.Run, bool) {
	run, err := handler.engine.Run(runID)
	if err != nil {
		status, code := mapAutomationError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return automationmodel.Run{}, false
	}
	if !requestOwnsTenant(w, r, run.TenantID) {
		return automationmodel.Run{}, false
	}
	return run, true
}

func requestActor(w http.ResponseWriter, r *http.Request) (identity.Actor, bool) {
	actor, ok := identity.FromRequest(r)
	if !ok {
		basehandler.WriteError(w, http.StatusUnauthorized, "TENANT_IDENTITY_REQUIRED", "authenticated tenant identity is required", false)
		return identity.Actor{}, false
	}
	return actor, true
}

func requestOwnsTenant(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	actor, ok := requestActor(w, r)
	if !ok {
		return false
	}
	if tenantID == "" || tenantID != actor.TenantID {
		basehandler.WriteError(w, http.StatusNotFound, "NOT_FOUND", "automation resource not found", false)
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

type socialUploadConfig struct {
	jobType        string
	adapter        string
	allowedDomains []string
}

func socialUploadPlatformConfig(platform string) (socialUploadConfig, bool) {
	switch platform {
	case "youtube":
		return socialUploadConfig{
			jobType:        "social.youtube.upload_video",
			adapter:        "social.youtube.upload_video",
			allowedDomains: []string{"studio.youtube.com", "*.youtube.com"},
		}, true
	case "tiktok":
		return socialUploadConfig{
			jobType:        "social.tiktok.upload_video",
			adapter:        "social.tiktok.upload_video",
			allowedDomains: []string{"www.tiktok.com", "*.tiktok.com"},
		}, true
	case "instagram":
		return socialUploadConfig{
			jobType:        "social.instagram.upload_video",
			adapter:        "social.instagram.upload_video",
			allowedDomains: []string{"www.instagram.com", "*.instagram.com"},
		}, true
	default:
		return socialUploadConfig{}, false
	}
}

func normalizeTags(values []string) []string {
	tags := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		tag := strings.TrimSpace(strings.TrimPrefix(value, "#"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

func normalizeNonEmptyStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
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
			Name:        "social.youtube.upload_video.publish",
			ProductLine: "social",
			JobType:     "social.youtube.upload_video",
			Adapter:     "social.youtube.upload_video",
			Target:      map[string]any{"allowed_domains": []string{"studio.youtube.com", "*.youtube.com"}},
			Policy: map[string]any{
				"allowed_actions":         []string{"observe_page", "screenshot", "wait_for"},
				"manual_publish_required": false,
			},
		},
		{
			Name:        "social.tiktok.upload_video.publish",
			ProductLine: "social",
			JobType:     "social.tiktok.upload_video",
			Adapter:     "social.tiktok.upload_video",
			Target:      map[string]any{"allowed_domains": []string{"www.tiktok.com", "*.tiktok.com"}},
			Policy: map[string]any{
				"allowed_actions":         []string{"observe_page", "screenshot", "wait_for"},
				"manual_publish_required": false,
			},
		},
		{
			Name:        "social.instagram.upload_video.publish",
			ProductLine: "social",
			JobType:     "social.instagram.upload_video",
			Adapter:     "social.instagram.upload_video",
			Target:      map[string]any{"allowed_domains": []string{"www.instagram.com", "*.instagram.com"}},
			Policy: map[string]any{
				"allowed_actions":         []string{"observe_page", "screenshot", "wait_for"},
				"manual_publish_required": false,
			},
		},
	}
}

func normalizedContentType(raw string, filename string) string {
	mediaType, _, parseErr := mime.ParseMediaType(strings.TrimSpace(raw))
	if parseErr == nil && mediaType != "" && mediaType != "application/octet-stream" {
		return mediaType
	}
	if detected := mime.TypeByExtension(path.Ext(filename)); detected != "" {
		if mediaType, _, err := mime.ParseMediaType(detected); err == nil {
			return mediaType
		}
	}
	if parseErr == nil && mediaType != "" {
		return mediaType
	}
	return "application/octet-stream"
}

func artifactReadStatus(err error) (int, bool) {
	switch {
	case errors.Is(err, artifactstore.ErrNotFound):
		return http.StatusNotFound, false
	case errors.Is(err, artifactstore.ErrInvalidRange):
		return http.StatusRequestedRangeNotSatisfiable, false
	default:
		return http.StatusBadGateway, true
	}
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
