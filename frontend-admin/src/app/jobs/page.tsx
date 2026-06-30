"use client";

import { useEffect, useState } from "react";
import { Topbar } from "@/components/Topbar";
import { DataSection, EmptyRow, StatusBadge, Metric } from "@/components/UI";
import { api, errorMessage } from "@/lib/api";
import { Job } from "@/lib/types";
import { formatDateTime, jsonShort } from "@/lib/utils";
import { useRouter } from "next/navigation";

export default function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState("");
  const [error, setError] = useState("");
  const router = useRouter();

  const fetchJobs = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await api<{ jobs: Job[] }>("/admin/automation/jobs?limit=50");
      setJobs(res.jobs);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchJobs();
  }, []);

  async function createMockJob() {
    setActionLoading("create-job");
    setError("");
    try {
      await api<Job>("/admin/automation/jobs", {
        method: "POST",
        body: {
          job_type: "generic.browser.script",
          adapter: "mock.echo",
          target: { allowed_domains: ["example.com"] },
          input: { message: `admin-${new Date().toISOString()}` },
          policy: {},
          priority: Math.floor(Date.now() / 1000)
        }
      });
      await fetchJobs();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setActionLoading("");
    }
  }

  const stats = {
    queued: jobs.filter((job) => job.status === "queued").length
  };

  return (
    <>
      <Topbar
        onRefresh={fetchJobs}
        loading={loading}
        onCreateMock={createMockJob}
        actionLoading={actionLoading}
      />
      <section className="metrics-grid">
        <Metric label="排队任务" value={stats.queued} />
      </section>
      {error && <div className="error-banner">{error}</div>}
      <section className="workspace">
        <DataSection title="任务列表" count={jobs.length}>
          <table>
            <thead>
              <tr>
                <th>任务 ID</th>
                <th>状态</th>
                <th>类型</th>
                <th>适配器</th>
                <th>优先级</th>
                <th>创建时间</th>
                <th>输入</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job) => (
                <tr key={job.job_id}>
                  <td className="mono" title={job.job_id}>{job.job_id}</td>
                  <td><StatusBadge status={job.status} /></td>
                  <td>{job.job_type}</td>
                  <td>{job.adapter}</td>
                  <td>{job.priority}</td>
                  <td>{formatDateTime(job.created_at)}</td>
                  <td className="truncate">{jsonShort(job.input)}</td>
                </tr>
              ))}
              {!jobs.length && <EmptyRow colSpan={7} />}
            </tbody>
          </table>
        </DataSection>
      </section>
    </>
  );
}
