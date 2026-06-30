"use client";

import { useState } from "react";
import { Send, Loader2, Globe } from "lucide-react";
import { api, errorMessage } from "@/lib/api";

export function SocialMediaOpsForm() {
  const [platform, setPlatform] = useState("https://www.reddit.com");
  const [topic, setTopic] = useState("");
  const [actionDesc, setActionDesc] = useState("");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!topic.trim() || !actionDesc.trim()) {
      setMessage("请完善任务信息");
      return;
    }
    setLoading(true);
    setMessage("");
    
    const taskDescription = `前往该社交平台，搜索主题 '${topic.trim()}'。${actionDesc.trim()}`;
    
    try {
      const job = await api<{ job_id: string }>("/web/automation/browser-agent-jobs", {
        method: "POST",
        body: {
          url: platform,
          task: taskDescription,
          allowed_domains: ["*"],
          allow_download: false,
        },
      });
      setMessage(`已下发社媒运营任务 ${job.job_id}`);
      setTopic("");
      setActionDesc("");
    } catch (err) {
      setMessage(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <h2><Globe size={18} style={{ display: "inline", verticalAlign: "middle", marginRight: 8 }} />海外社交媒体自动化运营</h2>
        <span>场景二</span>
      </div>
      <form onSubmit={handleSubmit} style={{ display: "grid", gap: 14, padding: 14 }}>
        <div className="form-group">
          <label className="form-label">目标平台</label>
          <select 
            className="form-input"
            value={platform} 
            onChange={(e) => setPlatform(e.target.value)}
          >
            <option value="https://www.reddit.com">Reddit</option>
            <option value="https://twitter.com">Twitter (X)</option>
            <option value="https://discord.com/app">Discord</option>
          </select>
        </div>
        <div className="form-group">
          <label className="form-label">账号主题/版块</label>
          <input
            className="form-input"
            type="text"
            value={topic}
            onChange={(e) => setTopic(e.target.value)}
            placeholder="例如：AI Agents 或 specific subreddit"
            required
          />
        </div>
        <div className="form-group">
          <label className="form-label">操作要求</label>
          <textarea
            className="form-textarea"
            value={actionDesc}
            onChange={(e) => setActionDesc(e.target.value)}
            placeholder="例如：浏览最新帖子，提取热点讨论，并对提及某产品的帖子点赞"
            required
          />
        </div>
        <div className="section-hint" style={{ fontSize: 12, color: "#666" }}>
          人机协同：如遇验证码将自动暂停并等待人工接入
        </div>
        <button type="submit" className="btn btn-primary" disabled={loading} style={{ backgroundColor: "#1d9bf0", borderColor: "#1d9bf0" }}>
          {loading ? <Loader2 size={16} className="spin" /> : <Send size={16} />}
          <span>下发运营指令</span>
        </button>
        {message ? <div className="message">{message}</div> : null}
      </form>
    </div>
  );
}
