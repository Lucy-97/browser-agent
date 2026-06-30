"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Database, FileText } from "lucide-react";

type Triplet = Record<string, unknown>;

type ExtractionEntry = {
  template_name?: string;
  template_version?: number;
  success?: boolean;
  extractions?: Triplet[];
  error_code?: string;
  error_message?: string;
  llm_model?: string;
  // Browser Agent format: fields is a flat key-value dict
  fields?: Record<string, unknown>;
};

type Props = {
  extracted?: Record<string, unknown>;
};

export function ExtractionDetail({ extracted }: Props) {
  const [showText, setShowText] = useState(false);
  const llmExtractions = (extracted?.llm_extractions || []) as ExtractionEntry[];
  const rawText = (extracted?.text as string) || "";
  const source = (extracted?.source as string) || "";
  const pageCount = extracted?.page_count as number | undefined;
  const filename = (extracted?.filename as string) || "";

  return (
    <div style={{ padding: "12px 0", display: "grid", gap: 12 }}>
        {/* Status bar */}
      <div style={{ display: "flex", alignItems: "center", gap: 16, flexWrap: "wrap" }}>
        {source ? (
          <span style={{ fontSize: 11, color: "var(--muted, #8a96a3)" }}>
            来源: {source}
          </span>
        ) : null}
        {filename ? (
          <span style={{ fontSize: 11, color: "var(--muted, #8a96a3)" }}>
            文件: {filename}
          </span>
        ) : null}
        {pageCount ? (
          <span style={{ fontSize: 11, color: "var(--muted, #8a96a3)" }}>
            页数: {pageCount}
          </span>
        ) : null}
      </div>

      {/* LLM Extractions */}
      {llmExtractions.length > 0 ? (
        llmExtractions.map((entry, idx) => {
          // Browser Agent format: entry has `fields` (flat key-value) instead of `extractions` (triplet array)
          const isBrowserAgent = source === "browser_agent" && entry.fields && Object.keys(entry.fields).length > 0;
          // Determine success: explicit `success` field, or browser_agent with data = success
          const isSuccess = entry.success !== undefined ? entry.success : (isBrowserAgent ? true : false);

          return (
          <div key={idx} style={{ display: "grid", gap: 6 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, fontWeight: 600 }}>
              <span style={{ color: isSuccess ? "#22c55e" : "#ef4444" }}>
                {isSuccess ? "抽取成功" : "抽取失败"}
              </span>
              {entry.template_name ? (
                <span style={{ color: "var(--muted, #8a96a3)", fontWeight: 400 }}>
                  模板: {entry.template_name}
                  {entry.template_version ? ` v${entry.template_version}` : ""}
                </span>
              ) : null}
              {entry.llm_model ? (
                <span style={{ color: "var(--muted, #8a96a3)", fontWeight: 400 }}>
                  模型: {entry.llm_model}
                </span>
              ) : null}
            </div>
            {entry.error_message ? (
              <div className="message error" style={{ fontSize: 11, padding: "4px 8px" }}>
                {entry.error_code}: {entry.error_message}
              </div>
            ) : null}
            {/* Browser Agent: render fields as key-value table */}
            {isBrowserAgent ? (
              <div className="table-wrap" style={{ maxHeight: 300, overflow: "auto" }}>
                <table style={{ fontSize: 11, width: "100%", borderCollapse: "collapse" }}>
                  <tbody>
                    {Object.entries(entry.fields!).map(([k, v]) => (
                      <tr key={k} style={{ borderBottom: "1px solid var(--border-light, #e2e8ee)" }}>
                        <td style={{
                          padding: "6px 10px", fontWeight: 600, color: "var(--primary, #214f6b)",
                          whiteSpace: "nowrap", verticalAlign: "top", width: 140,
                          background: "rgba(33,79,107,0.04)",
                        }}>{k}</td>
                        <td style={{ padding: "6px 10px", color: "var(--foreground, #18212b)", lineHeight: 1.5 }}>
                          {v == null ? "-" : String(v)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : entry.extractions && entry.extractions.length > 0 ? (
              <div className="table-wrap" style={{ maxHeight: 300, overflow: "auto" }}>
                <table style={{ fontSize: 11 }}>
                  <thead>
                    <tr>
                      <th>#</th>
                      {Object.keys(entry.extractions[0] || {}).map((k) => (
                        <th key={k}>{k}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {entry.extractions.map((triplet, tIdx) => (
                      <tr key={tIdx}>
                        <td style={{ color: "var(--muted, #8a96a3)" }}>{tIdx + 1}</td>
                        {Object.keys(entry.extractions![0] || {}).map((k) => (
                          <td key={k} style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                            title={String(triplet[k] ?? "")}
                          >
                            {triplet[k] == null ? "-" : String(triplet[k])}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div style={{ fontSize: 11, color: "var(--muted, #8a96a3)" }}>暂无抽取数据</div>
            )}
          </div>
          );
        })
      ) : (
        <div style={{ fontSize: 12, color: "var(--muted, #8a96a3)" }}>
          暂无抽取结果。
        </div>
      )}

      {/* Raw text preview */}
      {rawText ? (
        <div>
          <button
            onClick={() => setShowText(!showText)}
            style={{
              display: "flex", alignItems: "center", gap: 4,
              background: "none", border: "none", cursor: "pointer",
              color: "var(--accent, #3b82f6)", fontSize: 12, padding: 0, fontWeight: 600,
            }}
          >
            {showText ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            <FileText size={14} />
            <span>原文预览 ({rawText.length} 字符)</span>
          </button>
          {showText ? (
            <pre
              style={{
                marginTop: 6,
                padding: 10,
                background: "#f8fafc",
                borderRadius: 6,
                fontSize: 10,
                lineHeight: 1.5,
                maxHeight: 300,
                overflow: "auto",
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
                color: "#374151",
              }}
            >
              {rawText.substring(0, 2000)}
              {rawText.length > 2000 ? "\n\n…（已截断）" : ""}
            </pre>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
