"use client";

import { Loader2, Plus, RefreshCw } from "lucide-react";
import { formatDateTime } from "@/lib/utils";
import { useEffect, useState } from "react";

export function Topbar({
  title = "自动化控制台",
  onRefresh,
  loading,
  onCreateMock,
  actionLoading
}: {
  title?: string;
  onRefresh: () => void;
  loading: boolean;
  onCreateMock?: () => void;
  actionLoading?: string;
}) {
  const [time, setTime] = useState("");

  useEffect(() => {
    setTime(new Date().toISOString());
  }, []);

  return (
    <header className="topbar">
      <div>
        <h1>{title}</h1>
        <div className="topbar-meta">{formatDateTime(time)}</div>
      </div>
      <div className="toolbar">
        <button type="button" className="tool-button" onClick={onRefresh} disabled={loading} title="刷新">
          {loading ? <Loader2 className="spin" size={18} /> : <RefreshCw size={18} />}
          <span>刷新</span>
        </button>
        {onCreateMock && (
          <button
            type="button"
            className="primary-button"
            onClick={onCreateMock}
            disabled={actionLoading === "create-job"}
          >
            {actionLoading === "create-job" ? <Loader2 className="spin" size={18} /> : <Plus size={18} />}
            <span>模拟任务</span>
          </button>
        )}
      </div>
    </header>
  );
}
