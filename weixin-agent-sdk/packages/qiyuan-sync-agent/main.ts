#!/usr/bin/env node

import { homedir } from "node:os";
import path from "node:path";

import { login, start } from "weixin-agent-sdk";

import { QiyuanSyncAgent } from "./src/sync-agent.js";

const command = process.argv[2];

async function main() {
  switch (command) {
    case "login": {
      await login();
      break;
    }

    case "start": {
      const storageDir =
        process.env.QIYUAN_WEIXIN_SYNC_DIR ??
        path.join(homedir(), ".qiyuan", "weixin-sync");
      const manifestPath = process.env.QIYUAN_WEIXIN_MANIFEST;

      const agent = new QiyuanSyncAgent({ storageDir, manifestPath, log: console.log });
      const ac = new AbortController();

      process.on("SIGINT", () => {
        console.log("\n正在停止微信同步 agent...");
        ac.abort();
      });
      process.on("SIGTERM", () => ac.abort());

      console.log(`微信资料同步目录: ${storageDir}`);
      const bot = start(agent, { abortSignal: ac.signal });
      await bot.wait();
      break;
    }

    default:
      console.log(`qiyuan-weixin-sync-agent

用法:
  pnpm --filter qiyuan-weixin-sync-agent run login
  pnpm --filter qiyuan-weixin-sync-agent run start

环境变量:
  QIYUAN_WEIXIN_SYNC_DIR    同步文件保存目录，默认 ~/.qiyuan/weixin-sync
  QIYUAN_WEIXIN_MANIFEST    JSONL manifest 路径，默认在同步目录下 manifest.jsonl

微信指令:
  同步此群                  开启当前群/会话资料同步
  停止同步此群              暂停当前群/会话资料同步
  同步状态                  查看当前群/会话是否开启同步`);
      break;
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
