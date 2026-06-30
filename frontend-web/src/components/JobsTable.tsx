"use client";

import { useCallback, useEffect, useState } from "react";
import { RefreshCw, Loader2, ChevronDown, ChevronRight } from "lucide-react";
import { api, errorMessage, formatDateTime } from "@/lib/api";

type Job = {
  job_id: string;
  job_type: string;
  adapter: string;
  status: string;
  created_at: string;
  input?: Record<string, unknown>;
};

type Run = {
  run_id: string;
  status: string;
  summary?: Record<string, unknown>;
  error?: { code: string; message: string } | null;
  completed_at?: string;
};

type Artifact = {
  artifact_id: string;
  artifact_type: string;
  local_path?: string;
  metadata?: Record<string, unknown>;
  size_bytes?: number | null;
  created_at?: string;
};

export function JobsTable() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api<{ jobs: Job[] }>("/web/automation/jobs?limit=30");
      setJobs(data.jobs || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  function toggleExpand(id: string) {
    setExpandedId((prev) => (prev === id ? null : id));
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <h2>任务列表</h2>
        <button className="btn btn-sm" onClick={refresh} disabled={loading}>
          {loading ? <Loader2 size={14} className="spin" /> : <RefreshCw size={14} />}
          <span>刷新</span>
        </button>
      </div>
      {error ? <div className="message error" style={{ margin: 14 }}>{error}</div> : null}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th style={{ width: 28 }}></th>
              <th>任务 ID</th>
              <th>状态</th>
              <th>任务 / 查询</th>
              <th>适配器</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map((job) => {
              const isExpanded = expandedId === job.job_id;
              return (
                <JobRow
                  key={job.job_id}
                  job={job}
                  isExpanded={isExpanded}
                  onToggle={() => toggleExpand(job.job_id)}
                />
              );
            })}
            {!jobs.length ? (
              <tr>
                <td className="empty-cell" colSpan={6}>暂无数据</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function JobRow({
  job,
  isExpanded,
  onToggle,
}: {
  job: Job;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const [runs, setRuns] = useState<Run[] | null>(null);
  const [loadingRuns, setLoadingRuns] = useState(false);

  useEffect(() => {
    if (!isExpanded || runs) return;
    setLoadingRuns(true);
    api<{ runs: Run[] }>(`/web/automation/jobs/${job.job_id}/runs`)
      .then((data) => setRuns(data.runs || []))
      .catch(() => setRuns([]))
      .finally(() => setLoadingRuns(false));
  }, [isExpanded, job.job_id, runs]);

  const query = (job.input as Record<string, string>)?.query || "";
  const task = (job.input as Record<string, string>)?.task || query || "-";
  const url = (job.input as Record<string, string>)?.url || "";

  return (
    <>
      <tr style={{ cursor: "pointer" }} onClick={onToggle}>
        <td style={{ textAlign: "center", color: "var(--muted, #8a96a3)" }}>
          {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </td>
        <td className="mono">{job.job_id}</td>
        <td>
          <span className={`status status-${job.status}`}>{job.status}</span>
        </td>
        <td className="truncate" title={task}>{task}</td>
        <td>{job.adapter}</td>
        <td>{formatDateTime(job.created_at)}</td>
      </tr>
      {isExpanded ? (
        <tr>
          <td colSpan={6} style={{ padding: "0 16px 12px 40px", background: "rgba(59,130,246,0.02)" }}>
            {loadingRuns ? (
              <div style={{ padding: 12, color: "var(--muted, #8a96a3)" }}>
                <Loader2 size={14} className="spin" style={{ marginRight: 6 }} />
                加载中...
              </div>
            ) : runs && runs.length > 0 ? (
              runs.map((run) => (
                <RunDetail key={run.run_id} run={run} task={task} url={url} />
              ))
            ) : (
              <div style={{ padding: 12, color: "var(--muted, #8a96a3)" }}>暂无执行记录</div>
            )}
          </td>
        </tr>
      ) : null}
    </>
  );
}

function RunDetail({ run, task, url }: { run: Run; task: string; url: string }) {
  const summary = run.summary || {};
  const extracts = (summary.extracts as Array<{ fields?: Record<string, string> }>) || [];
  const hasExtracts = extracts.length > 0 && extracts.some((e) => e.fields && Object.keys(e.fields).length > 0);
  const findingItems = Array.isArray(summary.findings) ? summary.findings : [];
  const conclusion = typeof summary.conclusion === "string" ? summary.conclusion : "";
  const detected = typeof summary.detected === "number" ? summary.detected : null;

  const [artifacts, setArtifacts] = useState<Artifact[] | null>(null);
  const [loadingArtifacts, setLoadingArtifacts] = useState(false);

  useEffect(() => {
    if (!run.run_id || artifacts) return;
    setLoadingArtifacts(true);
    api<{ artifacts: Artifact[] }>(`/web/automation/runs/${run.run_id}/artifacts`)
      .then((data) => setArtifacts(data.artifacts || []))
      .catch(() => setArtifacts([]))
      .finally(() => setLoadingArtifacts(false));
  }, [artifacts, run.run_id]);

  const screenshotArtifacts = (artifacts || []).filter((a) => a.artifact_type === "screenshot");
  const traceArtifacts = (artifacts || []).filter((a) => a.artifact_type === "agent_trace");
  const hasArtifacts = !!artifacts && artifacts.length > 0;
  const API_PREFIX = process.env.NEXT_PUBLIC_API_PREFIX || "/api";

  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
        <span className="mono" style={{ fontSize: 11, color: "var(--muted, #4a5663)" }}>{run.run_id}</span>
        <span className={`status status-${run.status}`}>{run.status}</span>
        {run.completed_at ? (
          <span style={{ fontSize: 11, color: "var(--muted, #4a5663)" }}>
            {formatDateTime(run.completed_at)}
          </span>
        ) : null}
        {run.error?.code ? (
          <span style={{ fontSize: 11, color: "var(--danger, #9c2d2a)" }}>
            {run.error.code}: {run.error.message}
          </span>
        ) : null}
      </div>

      {url ? (
        <div style={{ fontSize: 12, marginBottom: 8, color: "var(--muted, #4a5663)" }}>
          URL: <span className="mono">{url}</span>
        </div>
      ) : null}

      <div style={{
        border: "1px solid var(--border, #cdd6e0)",
        borderRadius: 6,
        background: "rgba(33,79,107,0.03)",
        padding: "8px 10px",
        marginBottom: 10,
      }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: "var(--primary, #214f6b)", marginBottom: 4 }}>
          结论
        </div>
        <div style={{ fontSize: 13, color: "var(--foreground, #18212b)", lineHeight: 1.6 }}>
          {conclusion || "暂无可展示的结论"}
        </div>
        {detected !== null ? (
          <div style={{ fontSize: 12, color: "var(--muted, #4a5663)", marginTop: 4 }}>
            命中项：{detected}
          </div>
        ) : null}
        {findingItems.length ? (
          <div style={{ marginTop: 6 }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: "var(--primary, #214f6b)", marginBottom: 4 }}>
              本次命中项
            </div>
            <ul style={{ margin: 0, paddingLeft: 18, fontSize: 12, color: "var(--foreground, #18212b)" }}>
              {findingItems.map((item, idx) => (
                <li key={`${idx}-${item}`}>{item}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>

      {hasExtracts ? (
        <div style={{
          border: "1px solid var(--border, #cdd6e0)",
          borderRadius: 6,
          overflow: "hidden",
        }}>
          <div style={{
            fontSize: 12,
            fontWeight: 700,
            padding: "6px 12px",
            background: "var(--primary-light, #eaf2ff)",
            color: "var(--primary, #214f6b)",
            borderBottom: "1px solid var(--border, #cdd6e0)",
          }}>
            抽取结果
          </div>
          {extracts.map((extract, idx) => (
            <div key={idx}>
              {extract.fields ? (
                <table style={{ width: "100%", fontSize: 13, borderCollapse: "collapse" }}>
                  <tbody>
                    {Object.entries(extract.fields).map(([key, value]) => (
                      <tr key={key} style={{ borderBottom: "1px solid var(--border-light, #e2e8ee)" }}>
                        <td style={{
                          padding: "6px 12px",
                          fontWeight: 600,
                          color: "var(--primary, #214f6b)",
                          whiteSpace: "nowrap",
                          verticalAlign: "top",
                          width: 180,
                          background: "rgba(33,79,107,0.04)",
                        }}>
                          {key}
                        </td>
                        <td style={{
                          padding: "6px 12px",
                          color: "var(--foreground, #18212b)",
                          lineHeight: 1.5,
                        }}>
                          {value}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}

      <div style={{ marginTop: 10 }}>
        <div style={{
          fontSize: 12,
          fontWeight: 700,
          color: "var(--primary, #214f6b)",
          marginBottom: 6,
        }}>
          取证产物
        </div>

        {loadingArtifacts ? (
          <div style={{ fontSize: 12, color: "var(--muted, #4a5663)" }}>
            <Loader2 size={14} className="spin" style={{ marginRight: 6 }} />
            加载中...
          </div>
        ) : hasArtifacts ? (
          <div style={{ display: "grid", gap: 6 }}>
            {screenshotArtifacts.length ? (
              <div style={{
                border: "1px solid var(--border, #cdd6e0)",
                borderRadius: 6,
                padding: "8px 10px",
                background: "rgba(34,197,94,0.04)",
              }}>
                <div style={{ fontSize: 12, fontWeight: 700, marginBottom: 6 }}>
                  截图证据（{screenshotArtifacts.length}）
                </div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  {screenshotArtifacts.slice(0, 6).map((artifact) => (
                    <a
                      key={artifact.artifact_id}
                      href={`${API_PREFIX}/admin/automation/artifacts/${artifact.artifact_id}/download`}
                      target="_blank"
                      rel="noreferrer"
                      style={{
                        fontSize: 12,
                        color: "var(--primary, #214f6b)",
                        textDecoration: "underline",
                      }}
                      title={artifact.local_path || artifact.artifact_type}
                    >
                      打开截图
                    </a>
                  ))}
                  {screenshotArtifacts.length > 6 ? (
                    <span style={{ fontSize: 12, color: "var(--muted, #4a5663)" }}>
                      +{screenshotArtifacts.length - 6} 更多
                    </span>
                  ) : null}
                </div>
              </div>
            ) : null}

            {traceArtifacts.length ? (
              <div style={{
                border: "1px solid var(--border, #cdd6e0)",
                borderRadius: 6,
                padding: "8px 10px",
                background: "rgba(59,130,246,0.04)",
              }}>
                <div style={{ fontSize: 12, fontWeight: 700, marginBottom: 6 }}>
                  运行 Trace（{traceArtifacts.length}）
                </div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  {traceArtifacts.slice(0, 3).map((artifact) => (
                    <a
                      key={artifact.artifact_id}
                      href={`${API_PREFIX}/admin/automation/artifacts/${artifact.artifact_id}/download`}
                      target="_blank"
                      rel="noreferrer"
                      style={{
                        fontSize: 12,
                        color: "var(--primary, #214f6b)",
                        textDecoration: "underline",
                      }}
                      title={artifact.local_path || artifact.artifact_type}
                    >
                      打开 trace
                    </a>
                  ))}
                </div>
              </div>
            ) : null}

            {!screenshotArtifacts.length && !traceArtifacts.length ? (
              <div style={{ fontSize: 12, color: "var(--muted, #4a5663)" }}>
                已生成 {artifacts.length} 个 artifacts，但暂无 screenshot / agent_trace 类型。
              </div>
            ) : null}
          </div>
        ) : (
          <div style={{ fontSize: 12, color: "var(--muted, #4a5663)" }}>
            暂无产物
          </div>
        )}
      </div>

      {summary.action_count ? (
        <div style={{ fontSize: 12, color: "var(--muted, #4a5663)", marginTop: 10 }}>
          <div style={{ fontWeight: 600, marginBottom: 4 }}>
            Actions: {summary.action_count as number}
            {(summary.screenshot_count as number) > 0 ? `, Screenshots: ${summary.screenshot_count}` : ""}
            {(summary.download_count as number) > 0 ? `, Downloads: ${summary.download_count}` : ""}
          </div>
          {summary.actions_history && Array.isArray(summary.actions_history) ? (
            <ul style={{ margin: 0, paddingLeft: 16, fontFamily: "monospace", fontSize: 11 }}>
              {summary.actions_history.map((action, idx) => (
                <li key={idx} style={{ marginBottom: 2 }}>{action}</li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
