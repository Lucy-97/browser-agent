import { BrowserActSmokeForm, CopyrightDetectionForm } from "@/components/CopyrightDetectionForm";
import { SocialMediaOpsForm } from "@/components/SocialMediaOpsForm";
import { WorkerSetup } from "@/components/WorkerSetup";

export default function Home() {
  return (
    <div style={{ display: "grid", gap: 8 }}>
      {/* Section 1: Create Tasks */}
      <section className="section">
        <h3 className="section-title">下发任务指令</h3>
        <p className="section-hint">
          <strong>全链路浏览器自动化</strong>：直接将业务需求下发给本地大模型驱动的浏览器 Agent 进行自主执行。
        </p>
        <div className="grid-2">
          <CopyrightDetectionForm />
          <SocialMediaOpsForm />
        </div>
        <div style={{ marginTop: 16, maxWidth: 720 }}>
          <BrowserActSmokeForm />
        </div>
      </section>

      <hr className="section-divider" />

      {/* Section 2: Worker Setup */}
      <section className="section">
        <h3 className="section-title">本地 Worker 节点</h3>
        <p className="section-hint">
          任务由用户自有的本地 Worker 认领并控制真实浏览器执行，业务平台不会直接触及或记录您的三方平台账号 Cookie 和 Session。
        </p>
        <WorkerSetup />
      </section>
    </div>
  );
}
