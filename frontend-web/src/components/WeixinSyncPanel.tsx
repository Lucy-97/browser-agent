"use client";

import { useState } from "react";
import { FolderSync, Loader2, Send, SlidersHorizontal } from "lucide-react";
import { api, errorMessage } from "@/lib/api";

const DEFAULT_SOURCE_DIR = "$HOME/Library/Containers/com.tencent.xinWeChat/Data";

export function WeixinSyncPanel() {
  const [groupNames, setGroupNames] = useState("");
  const [sourceDirs, setSourceDirs] = useState(DEFAULT_SOURCE_DIR);
  const [maxFiles, setMaxFiles] = useState(200);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const dirs = sourceDirs.split("\n").map((item) => item.trim()).filter(Boolean);
    const groups = groupNames.split("\n").map((item) => item.trim()).filter(Boolean);
    if (!groups.length) {
      setMessage("请输入要同步的微信群名称");
      return;
    }
    if (!dirs.length) {
      setMessage("请在高级设置中保留至少一个本机微信资料目录");
      return;
    }
    setLoading(true);
    setMessage("");
    try {
      const job = await api<{ job_id: string }>("/web/automation/weixin-desktop-sync-jobs", {
        method: "POST",
        body: {
          source_dirs: dirs,
          group_names: groups,
          selected_groups: groups.map((name) => ({ display_name: name })),
          max_files: maxFiles,
        },
      });
      setMessage(`已下发 ${groups.length} 个微信群资料同步任务 ${job.job_id}`);
    } catch (err) {
      setMessage(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <h2>
          <FolderSync size={18} style={{ display: "inline", verticalAlign: "middle", marginRight: 8 }} />
          微信群资料同步
        </h2>
        <span>场景三</span>
      </div>
      <form onSubmit={handleSubmit} style={{ display: "grid", gap: 14, padding: 14 }}>
        <div className="form-grid-2">
          <div className="form-group">
            <label className="form-label">要同步的微信群</label>
            <textarea
              className="form-textarea"
              value={groupNames}
              onChange={(e) => setGroupNames(e.target.value)}
              placeholder="每行一个群名&#10;例如：内容与客户&#10;项目资料群"
            />
          </div>
          <div className="form-group">
            <label className="form-label">当前阶段</label>
            <div className="weixin-sync-status">
              <span className="status status-running">指定群资料同步</span>
              <span className="status status-queued">微信桌面自动化接入中</span>
            </div>
          </div>
        </div>

        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => setShowAdvanced((value) => !value)}
          aria-expanded={showAdvanced}
        >
          <SlidersHorizontal size={16} />
          <span>{showAdvanced ? "收起高级设置" : "高级设置"}</span>
        </button>

        {showAdvanced ? (
          <div className="form-grid-2">
            <div className="form-group">
              <label className="form-label">本机微信资料目录</label>
              <textarea
                className="form-textarea"
                value={sourceDirs}
                onChange={(e) => setSourceDirs(e.target.value)}
                placeholder={DEFAULT_SOURCE_DIR}
              />
            </div>
            <div className="form-group">
              <label className="form-label">最多同步文件数</label>
              <input
                className="form-input"
                type="number"
                min={1}
                max={1000}
                value={maxFiles}
                onChange={(e) => setMaxFiles(Number(e.target.value))}
              />
            </div>
          </div>
        ) : null}

        <div className="section-hint" style={{ fontSize: 12 }}>
          当前版本按微信群名称匹配本机微信已下载资料路径；下一步会改为自动读取群列表并直接按勾选群同步。
        </div>
        <button type="submit" className="btn btn-primary" disabled={loading}>
          {loading ? <Loader2 size={16} className="spin" /> : <Send size={16} />}
          <span>下发微信资料同步任务</span>
        </button>
        {message ? <div className="message">{message}</div> : null}
      </form>
    </div>
  );
}
