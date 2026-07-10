"use client";

import { useMemo, useState } from "react";
import { Loader2, UploadCloud } from "lucide-react";
import { api, errorMessage } from "@/lib/api";

const PLATFORMS = [
  { value: "instagram", label: "Instagram Reels" },
  { value: "tiktok", label: "TikTok" },
  { value: "youtube", label: "YouTube Shorts" },
];

export function SocialUploadForm() {
  const [platform, setPlatform] = useState("instagram");
  const [videoPath, setVideoPath] = useState("");
  const [artifactId, setArtifactId] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState("");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [isError, setIsError] = useState(false);

  const tagList = useMemo(
    () =>
      tags
        .split(",")
        .map((tag) => tag.trim().replace(/^#/, ""))
        .filter(Boolean),
    [tags],
  );

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsError(false);
    if (!videoPath.trim() && !artifactId.trim()) {
      setMessage("请填写本地视频路径或 artifact_id");
      setIsError(true);
      return;
    }
    if (!title.trim()) {
      setMessage("请填写标题");
      setIsError(true);
      return;
    }
    setLoading(true);
    setMessage("");

    try {
      const job = await api<{ job_id: string }>("/web/automation/social-upload-jobs", {
        method: "POST",
        body: {
          platform,
          video_path: videoPath.trim(),
          artifact_id: artifactId.trim(),
          title: title.trim(),
          description: description.trim(),
          tags: tagList,
          headed: true,
          manual_publish_required: false,
        },
      });
      setMessage(`已下发 ${platform} 上传任务 ${job.job_id}`);
      setTitle("");
      setDescription("");
      setTags("");
    } catch (err) {
      setMessage(errorMessage(err));
      setIsError(true);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <h2>
          <UploadCloud size={18} style={{ display: "inline", verticalAlign: "middle", marginRight: 8 }} />
          国外平台视频上传
        </h2>
        <span>场景四</span>
      </div>
      <form onSubmit={handleSubmit} style={{ display: "grid", gap: 14, padding: 14 }}>
        <div className="form-grid-2">
          <div className="form-group">
            <label className="form-label">目标平台</label>
            <select className="form-input" value={platform} onChange={(e) => setPlatform(e.target.value)}>
              {PLATFORMS.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="form-grid-2">
          <div className="form-group">
            <label className="form-label">本地视频路径</label>
            <input
              className="form-input"
              type="text"
              value={videoPath}
              onChange={(e) => setVideoPath(e.target.value)}
              placeholder="/Users/me/Videos/reel.mp4"
            />
          </div>
          <div className="form-group">
            <label className="form-label">artifact_id</label>
            <input
              className="form-input"
              type="text"
              value={artifactId}
              onChange={(e) => setArtifactId(e.target.value)}
              placeholder="art_xxx"
            />
          </div>
        </div>

        <div className="form-group">
          <label className="form-label">标题</label>
          <input
            className="form-input"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="例如：AI Agent 自动化运营演示"
            required
          />
        </div>
        <div className="form-group">
          <label className="form-label">描述</label>
          <textarea
            className="form-textarea"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="用于 YouTube description 或 Instagram/TikTok caption"
          />
        </div>
        <div className="form-group">
          <label className="form-label">话题标签</label>
          <input
            className="form-input"
            type="text"
            value={tags}
            onChange={(e) => setTags(e.target.value)}
            placeholder="AI, automation, agents"
          />
        </div>

        <div className="section-hint" style={{ fontSize: 12 }}>
          Worker 会复用本机浏览器登录态，并自动完成上传与发布。
        </div>
        <button type="submit" className="btn btn-primary" disabled={loading}>
          {loading ? <Loader2 size={16} className="spin" /> : <UploadCloud size={16} />}
          <span>下发上传任务</span>
        </button>
        {message ? <div className={`message${isError ? " error" : ""}`}>{message}</div> : null}
      </form>
    </div>
  );
}
