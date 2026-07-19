import Link from "next/link";

export function WorkerSetup() {
  return (
    <div className="panel">
      <div className="panel-header">
        <h2>Worker</h2>
        <span>本地节点</span>
      </div>
      <div style={{ display: "grid", gap: 8, padding: 14 }}>
        <code className="code-block">bash deploy-local/tools/run-worker-host-local.sh init</code>
        <code className="code-block">bash deploy-local/tools/run-worker-host-local.sh pair</code>
        <code className="code-block">bash deploy-local/tools/run-worker-host-local.sh start</code>
        <Link href="/worker/pair" className="btn btn-primary" style={{ width: "fit-content", textDecoration: "none" }}>
          输入配对码
        </Link>
      </div>
      <div
        style={{
          borderTop: "1px solid var(--border-light)",
          color: "var(--muted)",
          padding: "12px 14px 14px",
          fontSize: 14,
          lineHeight: 1.5,
        }}
      >
        任务由用户自有的本地 Worker 拉取执行，第三方凭据仅保留在本地浏览器配置中，平台不会采集。
      </div>
    </div>
  );
}
