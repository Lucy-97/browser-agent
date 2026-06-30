"use client";

import { useEffect, useState } from "react";
import { Topbar } from "@/components/Topbar";
import { DataSection, EmptyRow, StatusBadge, Metric } from "@/components/UI";
import { api, errorMessage } from "@/lib/api";
import { ManualAction } from "@/lib/types";
import { formatDateTime } from "@/lib/utils";
import { CheckCircle2, Loader2 } from "lucide-react";

export default function ManualPage() {
  const [manualActions, setManualActions] = useState<ManualAction[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState("");
  const [error, setError] = useState("");

  const fetchManualActions = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await api<{ manual_actions: ManualAction[] }>("/admin/automation/manual-actions?limit=50");
      setManualActions(res.manual_actions);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchManualActions();
  }, []);

  const resolveManualAction = async (actionID: string) => {
    setActionLoading(actionID);
    setError("");
    try {
      await api<ManualAction>(`/admin/automation/manual-actions/${actionID}/resolve`, {
        method: "POST",
        body: { status: "resolved", payload: { resolved_by: "admin-ui" } }
      });
      await fetchManualActions();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setActionLoading("");
    }
  };

  const stats = {
    pendingManual: manualActions.filter((action) => action.status === "pending").length
  };

  return (
    <>
      <Topbar onRefresh={fetchManualActions} loading={loading} />
      <section className="metrics-grid">
        <Metric label="待人工处理" value={stats.pendingManual} />
      </section>
      {error && <div className="error-banner">{error}</div>}
      <section className="workspace">
        <DataSection title="人工干预" count={manualActions.length}>
          <table>
            <thead>
              <tr>
                <th>操作 ID</th>
                <th>状态</th>
                <th>类型</th>
                <th>执行 ID</th>
                <th>消息</th>
                <th>创建时间</th>
                <th className="right">操作</th>
              </tr>
            </thead>
            <tbody>
              {manualActions.map((action) => (
                <tr key={action.manual_action_id}>
                  <td className="mono" title={action.manual_action_id}>{action.manual_action_id}</td>
                  <td><StatusBadge status={action.status} /></td>
                  <td>{action.action_type}</td>
                  <td className="mono" title={action.run_id}>{action.run_id}</td>
                  <td className="truncate">{action.message}</td>
                  <td>{formatDateTime(action.created_at)}</td>
                  <td className="right">
                    <button
                      type="button"
                      className="tool-button compact"
                      onClick={() => resolveManualAction(action.manual_action_id)}
                      disabled={action.status !== "pending" || actionLoading === action.manual_action_id}
                      title="标记为已处理"
                    >
                      {actionLoading === action.manual_action_id ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
                      <span>处理</span>
                    </button>
                  </td>
                </tr>
              ))}
              {!manualActions.length && <EmptyRow colSpan={7} />}
            </tbody>
          </table>
        </DataSection>
      </section>
    </>
  );
}
