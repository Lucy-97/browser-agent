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
