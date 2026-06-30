"use client";

import { useCallback, useEffect, useState } from "react";
import { RefreshCw, Loader2, ChevronDown, ChevronRight } from "lucide-react";
import { api, errorMessage, formatDateTime } from "@/lib/api";
import { ExtractionDetail } from "./ExtractionDetail";

type LiteratureResult = {
  result_id: string;
  title: string;
  year?: number;
  doi?: string;
  parse_status: string;
  extracted?: Record<string, unknown>;
  updated_at: string;
};

export function ResultsTable() {
  const [results, setResults] = useState<LiteratureResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api<{ results: LiteratureResult[] }>("/web/literature/results?limit=30");
      setResults(data.results || []);
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
        <h2>文献结果</h2>
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
              <th>标题</th>
              <th>解析状态</th>
              <th>年份</th>
              <th>DOI</th>
              <th>更新时间</th>
            </tr>
          </thead>
          <tbody>
            {results.map((result) => {
              const isExpanded = expandedId === result.result_id;
              return (
                <ResultRow
                  key={result.result_id}
                  result={result}
                  isExpanded={isExpanded}
                  onToggle={() => toggleExpand(result.result_id)}
                />
              );
            })}
            {!results.length ? (
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

function ResultRow({
  result,
  isExpanded,
  onToggle,
}: {
  result: LiteratureResult;
  isExpanded: boolean;
  onToggle: () => void;
}) {

  return (
    <>
      <tr
        style={{ cursor: "pointer" }}
        onClick={onToggle}
      >
        <td style={{ textAlign: "center", color: "var(--muted, #8a96a3)" }}>
          {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </td>
        <td className="truncate" title={result.title}>{result.title || "-"}</td>
        <td>
          <span className={`status status-${result.parse_status}`}>{result.parse_status}</span>
        </td>
        <td>{result.year || "-"}</td>
        <td className="truncate">{result.doi || "-"}</td>
        <td>{formatDateTime(result.updated_at)}</td>
      </tr>
      {isExpanded ? (
        <tr>
          <td colSpan={6} style={{ padding: "0 16px 12px 40px", background: "rgba(59,130,246,0.02)" }}>
            <ExtractionDetail extracted={result.extracted} />
          </td>
        </tr>
      ) : null}
    </>
  );
}
