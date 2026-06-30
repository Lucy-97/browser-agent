"use client";

import { useCallback, useEffect, useState } from "react";
import { RefreshCw, Loader2 } from "lucide-react";
import { api, errorMessage, formatDateTime } from "@/lib/api";

type EvidenceRow = {
  run_id: string;
  job_id: string;
  adapter: string;
  status: string;
  completed_at?: string;
  query?: string;
  task?: string;
  start_url?: string;
  evidence_url?: string;
  evidence_title?: string;
  domain?: string;
  domain_location?: string;
  hit_keywords?: string[];
  notes?: string;
  screenshot_artifact_ids?: string[];
  trace_artifact_id?: string;
  piracy_found?: boolean;
  conclusion?: string;
};

type ReportSummary = {
  total_runs: number;
  detected: number;
  confirmed: number;
  candidates: number;
  unknown: number;
  conclusion?: string;
  highlights?: string[];
  findings?: string[];
};

export function CopyrightEvidenceReport() {
  const [rows, setRows] = useState<EvidenceRow[]>([]);
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const API_PREFIX = process.env.NEXT_PUBLIC_API_PREFIX || "/api";

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api<{ rows: EvidenceRow[]; summary?: ReportSummary }>(
        "/web/automation/reports/copyright-evidence?limit=50"
      );
      setRows(data.rows || []);
      setSummary(data.summary || null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="panel">
      <div className="panel-header">
        <h2>侵权取证报告表</h2>
        <button className="btn btn-sm" onClick={refresh} disabled={loading}>
          {loading ? <Loader2 size={14} className="spin" /> : <RefreshCw size={14} />}
          <span>刷新</span>
        </button>
      </div>
      {error ? (
        <div className="message error" style={{ margin: 14 }}>
          {error}
        </div>
      ) : null}

      <div style={{ padding: "0 14px 10px 14px", color: "var(--muted, #8a96a3)", fontSize: 12 }}>
        展示来自 Automation Runs 的取证产物聚合：URL、域名归属地（演示版）、截图证据、trace 与结论。
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, minmax(0, 1fr))", gap: 12, padding: "0 14px 14px 14px" }}>
        <SummaryCard label="总运行数" value={summary?.total_runs ?? rows.length} />
        <SummaryCard label="确认侵权" value={summary?.confirmed ?? 0} tone="danger" />
        <SummaryCard label="候选命中" value={summary?.candidates ?? 0} tone="warning" />
        <SummaryCard label="未确认" value={summary?.unknown ?? 0} tone="neutral" />
      </div>

      <div style={{ padding: "0 14px 10px 14px" }}>
        <div style={{ border: "1px solid var(--border, #cdd6e0)", borderRadius: 8, background: "#fff" }}>
          <div style={{ padding: "8px 12px", fontSize: 12, fontWeight: 700, color: "var(--primary, #214f6b)", borderBottom: "1px solid var(--border, #cdd6e0)" }}>
            结论摘要
          </div>
          <div style={{ padding: "10px 12px", fontSize: 13, lineHeight: 1.7 }}>
            <div style={{ fontWeight: 700, color: "var(--foreground, #18212b)", marginBottom: 8 }}>
              {summary?.conclusion || "暂无可展示的结论。"}
            </div>
            <div style={{ color: "var(--muted, #64748b)", marginBottom: 8 }}>
              {summary?.detected !== undefined
                ? `本次共发现 ${summary.detected} 个命中项：${(summary.findings || []).length ? "见下方清单。" : "暂无可列出的命中项。"}`
                : "本次尚未生成可统计的命中项。"}
            </div>
            {summary?.highlights?.length ? (
              <ul style={{ margin: 0, paddingLeft: 18 }}>
                {summary.highlights.map((item) => <li key={item}>{item}</li>)}
              </ul>
            ) : (
              <span style={{ color: "var(--muted, #8a96a3)" }}>暂无可展示的结论。当前数据里还没有确认到明确侵权页面。</span>
            )}
          </div>
        </div>
      </div>

      <div style={{ padding: "0 14px 10px 14px" }}>
        <div style={{ border: "1px solid var(--border, #cdd6e0)", borderRadius: 8, background: "#fff" }}>
          <div style={{ padding: "8px 12px", fontSize: 12, fontWeight: 700, color: "var(--primary, #214f6b)", borderBottom: "1px solid var(--border, #cdd6e0)" }}>
            本次命中项
          </div>
          <div style={{ padding: "10px 12px", fontSize: 13, lineHeight: 1.7 }}>
            {summary?.findings?.length ? (
              <ol style={{ margin: 0, paddingLeft: 20 }}>
                {summary.findings.map((item, idx) => <li key={`${idx}-${item}`}>{item}</li>)}
              </ol>
            ) : (
              <span style={{ color: "var(--muted, #8a96a3)" }}>暂无命中项。</span>
            )}
          </div>
        </div>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>完成时间</th>
              <th>状态</th>
              <th>发现盗版</th>
              <th>Evidence URL</th>
              <th>域名</th>
              <th>归属地</th>
              <th>命中关键词</th>
              <th>截图证据</th>
              <th>Trace</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const screenshots = row.screenshot_artifact_ids || [];
              const isConfirmed = row.piracy_found === true;
              return (
                <tr key={row.run_id} style={isConfirmed ? { background: "rgba(254, 226, 226, 0.35)" } : undefined}>
                  <td style={{ whiteSpace: "nowrap" }}>{formatDateTime(row.completed_at)}</td>
                  <td>
                    <span className={`status status-${row.status}`}>{row.status}</span>
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    {row.piracy_found === true ? (
                      <span className="status" style={{ backgroundColor: "#fee2e2", color: "#991b1b", padding: "2px 6px" }}>已确认</span>
                    ) : row.piracy_found === false ? (
                      <span className="status" style={{ backgroundColor: "#dcfce7", color: "#166534", padding: "2px 6px" }}>未命中</span>
                    ) : (
                      <span className="status" style={{ backgroundColor: "#f3f4f6", color: "#4b5563", padding: "2px 6px" }}>未知</span>
                    )}
                  </td>
                  <td className="truncate" title={row.evidence_url || ""}>
                    {row.evidence_url ? (
                      <a
                        href={row.evidence_url}
                        target="_blank"
                        rel="noreferrer"
                        style={{ color: "var(--primary, #214f6b)", textDecoration: "underline" }}
                      >
                        {row.evidence_title || row.evidence_url}
                      </a>
                    ) : "-"}
                  </td>
                  <td className="mono truncate" title={row.domain || ""}>{row.domain || "-"}</td>
                  <td style={{ whiteSpace: "nowrap" }}>{row.domain_location || "-"}</td>
                  <td className="truncate" title={row.conclusion || ""} style={{ maxWidth: 260 }}>
                    {row.conclusion || "-"}
                  </td>
                  <td className="truncate" title={(row.hit_keywords || []).join(", ")}
                    style={{ maxWidth: 260 }}
                  >
                    {(row.hit_keywords || []).slice(0, 3).join(", ") || "-"}
                    {(row.hit_keywords || []).length > 3 ? " ..." : ""}
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    {screenshots.length ? (
                      <a
                        href={`${API_PREFIX}/admin/automation/artifacts/${screenshots[0]}/download`}
                        target="_blank"
                        rel="noreferrer"
                        style={{ color: "var(--primary, #214f6b)", textDecoration: "underline" }}
                        title={`screenshots=${screenshots.length}`}
                      >
                        打开截图（{screenshots.length}）
                      </a>
                    ) : "-"}
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    {row.trace_artifact_id ? (
                      <a
                        href={`${API_PREFIX}/admin/automation/artifacts/${row.trace_artifact_id}/download`}
                        target="_blank"
                        rel="noreferrer"
                        style={{ color: "var(--primary, #214f6b)", textDecoration: "underline" }}
                      >
                        打开
                      </a>
                    ) : "-"}
                  </td>
                </tr>
              );
            })}
            {!rows.length ? (
              <tr>
                <td className="empty-cell" colSpan={10}>
                  暂无数据
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function SummaryCard({
  label,
  value,
  tone = "neutral",
}: {
  label: string;
  value: number;
  tone?: "neutral" | "warning" | "danger";
}) {
  const background =
    tone === "danger" ? "#fef2f2" :
    tone === "warning" ? "#fffbeb" : "#f8fafc";
  const color =
    tone === "danger" ? "#991b1b" :
    tone === "warning" ? "#92400e" : "#334155";
  return (
    <div style={{ border: "1px solid var(--border, #cdd6e0)", borderRadius: 8, padding: "10px 12px", background }}>
      <div style={{ fontSize: 12, color: "var(--muted, #64748b)" }}>{label}</div>
      <div style={{ fontSize: 24, fontWeight: 800, color }}>{value}</div>
    </div>
  );
}
