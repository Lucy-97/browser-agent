"use client";

import { useState } from "react";
import { Search, Loader2, ShieldAlert } from "lucide-react";
import { api, errorMessage } from "@/lib/api";

export function CopyrightDetectionForm() {
  const [keyword, setKeyword] = useState("");
  const [engine, setEngine] = useState("https://www.google.com");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!keyword.trim()) {
      setMessage("请输入检索关键词");
      return;
    }
    setLoading(true);
    setMessage("");
    
    const taskDescription = `围绕关键词 '${keyword.trim()}' 开展分阶段检索：先从中性搜索词和常见播放入口出发，必要时再逐步扩展到疑似侵权线索；进入相关页面后截图取证，并尽量保留可展示的结论与证据。`;
    
    try {
      const job = await api<{ job_id: string }>("/web/automation/browser-act-jobs", {
        method: "POST",
        body: {
          url: engine,
          task: taskDescription,
          allowed_domains: ["*"],
          allow_download: false,
        },
      });
      setMessage(`已下发 browser.act 取证任务 ${job.job_id}`);
      setKeyword("");
    } catch (err) {
      setMessage(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <h2><ShieldAlert size={18} style={{ display: "inline", verticalAlign: "middle", marginRight: 8 }} />短剧版权侵权检索</h2>
        <span>场景一</span>
      </div>
      <form onSubmit={handleSubmit} style={{ display: "grid", gap: 14, padding: 14 }}>
        <div className="form-group">
          <label className="form-label">检索关键词</label>
          <input
            className="form-input"
            type="text"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="例如：玫瑰的故事 免费全集在线观看"
            required
          />
        </div>
        <div className="form-group">
          <label className="form-label">搜索引擎</label>
          <select 
            className="form-input"
            value={engine} 
            onChange={(e) => setEngine(e.target.value)}
          >
            <option value="https://www.google.com">Google</option>
            <option value="https://www.baidu.com">百度</option>
            <option value="https://www.bing.com">Bing</option>
          </select>
        </div>
        <div className="section-hint" style={{ fontSize: 12, color: "#666" }}>
          基于大模型自动检索全网并固化盗版证据
        </div>
        <button type="submit" className="btn btn-primary" disabled={loading} style={{ backgroundColor: "#d93025", borderColor: "#d93025" }}>
          {loading ? <Loader2 size={16} className="spin" /> : <Search size={16} />}
          <span>提交侵权取证任务</span>
        </button>
        {message ? <div className="message">{message}</div> : null}
      </form>
    </div>
  );
}
