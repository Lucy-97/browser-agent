import { ReactNode } from "react";

export function DataSection({ title, count, children }: { title: string; count: number; children: ReactNode }) {
  return (
    <section className="data-section">
      <div className="section-header">
        <h2>{title}</h2>
        <span>{count}</span>
      </div>
      <div className="table-wrap">{children}</div>
    </section>
  );
}

export function EmptyRow({ colSpan }: { colSpan: number }) {
  return (
    <tr>
      <td className="empty-cell" colSpan={colSpan}>
        暂无数据
      </td>
    </tr>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const normalized = status || "unknown";
  return <span className={`status status-${normalized}`}>{normalized}</span>;
}

export function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="metric">
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
    </div>
  );
}
