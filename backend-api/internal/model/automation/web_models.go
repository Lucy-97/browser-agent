package automation

import "time"

// WebEvidenceRow 是面向 Web 控制台的“取证报告表”单行数据结构。
// 注意：这里的字段是展示层模型，不等同于 run.summary 的原始结构。
//
// 设计目标：
// - 最小改动：不要求 Worker 立刻改造抽取字段即可用。
// - 可渐进增强：后续 Worker/LLM 抽取更结构化后，可继续填充更多字段。
// - 取证闭环：必须能一键打开 Screenshot Artifact。
type WebEvidenceRow struct {
	RunID           string         `json:"run_id"`
	JobID           string         `json:"job_id"`
	Adapter         string         `json:"adapter"`
	Status          string         `json:"status"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	Query           string         `json:"query,omitempty"`
	Task            string         `json:"task,omitempty"`
	StartURL        string         `json:"start_url,omitempty"`
	EvidenceURL     string         `json:"evidence_url,omitempty"`
	EvidenceTitle   string         `json:"evidence_title,omitempty"`
	Domain          string         `json:"domain,omitempty"`
	DomainLocation  string         `json:"domain_location,omitempty"`
	HitKeywords     []string       `json:"hit_keywords,omitempty"`
	Notes           string         `json:"notes,omitempty"`
	Conclusion      string         `json:"conclusion,omitempty"`
	ScreenshotIDs   []string       `json:"screenshot_artifact_ids,omitempty"`
	TraceArtifactID string         `json:"trace_artifact_id,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	PiracyFound     *bool          `json:"piracy_found,omitempty"`
}

type WebEvidenceReportSummary struct {
	TotalRuns  int      `json:"total_runs"`
	Detected   int      `json:"detected"`
	Confirmed  int      `json:"confirmed"`
	Candidates int      `json:"candidates"`
	Unknown    int      `json:"unknown"`
	Conclusion string   `json:"conclusion,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
	Findings   []string `json:"findings,omitempty"`
}
