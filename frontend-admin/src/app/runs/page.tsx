"use client";

import { useEffect, useState } from "react";
import { Topbar } from "@/components/Topbar";
import { DataSection, EmptyRow, StatusBadge, Metric } from "@/components/UI";
import { api, errorMessage } from "@/lib/api";
import { Run, Artifact, Checkpoint, ManualAction, TraceStep } from "@/lib/types";
import { formatDateTime, formatBytes, jsonPretty } from "@/lib/utils";
import { Ban, Download, Loader2 } from "lucide-react";

export default function RunsPage() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [selectedRunID, setSelectedRunID] = useState("");
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [checkpoints, setCheckpoints] = useState<Checkpoint[]>([]);
  const [runManualActions, setRunManualActions] = useState<ManualAction[]>([]);
  const [traceSteps, setTraceSteps] = useState<TraceStep[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState("");
  const [error, setError] = useState("");

  const fetchRuns = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await api<{ runs: Run[] }>("/admin/automation/runs?limit=50");
      setRuns(res.runs);
      if (!selectedRunID && res.runs[0]) {
        setSelectedRunID(res.runs[0].run_id);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchRuns();
  }, []);

  const loadRunDetails = async (runID: string) => {
    try {
      const [artifactResponse, checkpointResponse, manualResponse, traceResponse] = await Promise.all([
        api<{ artifacts: Artifact[] }>(`/admin/automation/runs/${runID}/artifacts`),
        api<{ checkpoints: Checkpoint[] }>(`/admin/automation/runs/${runID}/checkpoints`),
        api<{ manual_actions: ManualAction[] }>(`/admin/automation/runs/${runID}/manual-actions`),
        api<{ trace?: { steps?: TraceStep[] } }>(`/admin/automation/runs/${runID}/trace`)
      ]);
      setArtifacts(artifactResponse.artifacts);
      setCheckpoints(checkpointResponse.checkpoints);
      setRunManualActions(manualResponse.manual_actions);
      setTraceSteps(traceResponse.trace?.steps || []);
    } catch {
      setArtifacts([]);
      setCheckpoints([]);
      setRunManualActions([]);
      setTraceSteps([]);
    }
  };

  useEffect(() => {
    if (!selectedRunID) {
      setArtifacts([]);
      setCheckpoints([]);
      setRunManualActions([]);
      setTraceSteps([]);
      return;
    }
    void loadRunDetails(selectedRunID);
  }, [selectedRunID]);

  const cancelRun = async (runID: string) => {
    setActionLoading(`cancel-${runID}`);
    setError("");
    try {
      await api<Run>(`/admin/automation/runs/${runID}/cancel`, {
        method: "POST",
        body: { reason: "cancelled by admin UI" }
      });
      await fetchRuns();
      await loadRunDetails(runID);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setActionLoading("");
    }
  };

  const selectedRun = runs.find((run) => run.run_id === selectedRunID);
  const stats = {
    running: runs.filter((run) => run.status === "running").length
  };

  return (
    <>
      <Topbar onRefresh={fetchRuns} loading={loading} />
      <section className="metrics-grid">
        <Metric label="运行中" value={stats.running} />
      </section>
      {error && <div className="error-banner">{error}</div>}
      <section className="workspace">
        <div className="split-view">
          <DataSection title="执行记录" count={runs.length}>
            <table>
              <thead>
                <tr>
                  <th>执行 ID</th>
                  <th>状态</th>
                  <th>任务 ID</th>
                  <th>设备</th>
                  <th>开始时间</th>
                  <th>心跳</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => (
                  <tr
                    key={run.run_id}
                    className={selectedRunID === run.run_id ? "selected-row" : ""}
                    onClick={() => setSelectedRunID(run.run_id)}
                    style={{ cursor: "pointer" }}
                  >
                    <td className="mono" title={run.run_id}>{run.run_id}</td>
                    <td><StatusBadge status={run.status} /></td>
                    <td className="mono" title={run.job_id}>{run.job_id}</td>
                    <td className="mono" title={run.device_id}>{run.device_id}</td>
                    <td>{formatDateTime(run.started_at)}</td>
                    <td>{formatDateTime(run.last_heartbeat_at)}</td>
                  </tr>
                ))}
                {!runs.length && <EmptyRow colSpan={6} />}
              </tbody>
            </table>
          </DataSection>
          <section className="run-detail">
            <div className="detail-header">
              <div>
                <h2>执行详情</h2>
                <div className="mono">{selectedRunID || "-"}</div>
              </div>
              <button
                type="button"
                className="danger-button"
                onClick={() => cancelRun(selectedRunID)}
                disabled={!selectedRunID || selectedRun?.status !== "running" || actionLoading === `cancel-${selectedRunID}`}
                title="取消执行"
              >
                {actionLoading === `cancel-${selectedRunID}` ? <Loader2 className="spin" size={16} /> : <Ban size={16} />}
                <span>取消</span>
              </button>
            </div>
            <Timeline traceSteps={traceSteps} checkpoints={checkpoints} />
            <ArtifactsPanel artifacts={artifacts} />
            <RunManualActionsPanel manualActions={runManualActions} />
          </section>
        </div>
      </section>
    </>
  );
}

function Timeline({ traceSteps, checkpoints }: { traceSteps: TraceStep[]; checkpoints: Checkpoint[] }) {
  const items = traceSteps.length
    ? traceSteps.map((step, index) => ({
        id: `trace-${index}`,
        title: step.step,
        meta: step.action ? `action=${step.action}` : step.index !== undefined ? `index=${step.index}` : "",
        payload: step
      }))
    : checkpoints.map((checkpoint) => ({
        id: checkpoint.checkpoint_id,
        title: checkpoint.status || "checkpoint",
        meta: formatDateTime(checkpoint.created_at),
        payload: checkpoint.summary || checkpoint.cursor || {}
      }));
  return (
    <div className="detail-panel">
      <div className="detail-panel-header">
        <h3>Timeline</h3>
        <span>{items.length}</span>
      </div>
      <div className="timeline">
        {items.map((item) => (
          <div className="timeline-item" key={item.id}>
            <div className="timeline-dot" />
            <div className="timeline-body">
              <div className="timeline-title">{item.title}</div>
              {item.meta && <div className="timeline-meta">{item.meta}</div>}
              <pre>{jsonPretty(item.payload)}</pre>
            </div>
          </div>
        ))}
        {!items.length && <div className="empty-panel">暂无追踪或检查点</div>}
      </div>
    </div>
  );
}

function ArtifactsPanel({ artifacts }: { artifacts: Artifact[] }) {
  const API_PREFIX = process.env.NEXT_PUBLIC_API_PREFIX || "/api";
  return (
    <div className="detail-panel">
      <div className="detail-panel-header">
        <h3>Artifacts</h3>
        <span>{artifacts.length}</span>
      </div>
      <div className="artifact-list">
        {artifacts.map((artifact) => (
          <div className="artifact-item" key={artifact.artifact_id}>
            <div>
              <div className="artifact-type">{artifact.artifact_type}</div>
              <div className="artifact-meta">{formatBytes(artifact.size_bytes)} · {formatDateTime(artifact.created_at)}</div>
              <div className="truncate mono">{artifact.local_path || "-"}</div>
            </div>
            <a
              className="icon-link"
              href={`${API_PREFIX}/admin/automation/artifacts/${artifact.artifact_id}/download`}
              target="_blank"
              rel="noreferrer"
              title="下载 artifact"
            >
              <Download size={16} />
              <span>打开</span>
            </a>
          </div>
        ))}
        {!artifacts.length && <div className="empty-panel">暂无产物</div>}
      </div>
    </div>
  );
}

function RunManualActionsPanel({ manualActions }: { manualActions: ManualAction[] }) {
  return (
    <div className="detail-panel">
      <div className="detail-panel-header">
        <h3>Manual Actions</h3>
        <span>{manualActions.length}</span>
      </div>
      <div className="artifact-list">
        {manualActions.map((action) => (
          <div className="artifact-item" key={action.manual_action_id}>
            <div>
              <div className="artifact-type">{action.action_type}</div>
              <div className="artifact-meta">{action.status} · {formatDateTime(action.created_at)}</div>
              <div className="truncate">{action.message}</div>
            </div>
          </div>
        ))}
        {!manualActions.length && <div className="empty-panel">暂无人工干预</div>}
      </div>
    </div>
  );
}
