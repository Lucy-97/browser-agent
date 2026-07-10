import fs from "node:fs/promises";
import path from "node:path";

import type { Agent, ChatRequest, ChatResponse } from "weixin-agent-sdk";

type SyncRecord = {
  id: string;
  conversationId: string;
  receivedAt: string;
  text: string;
  mediaType: "image" | "audio" | "video" | "file";
  mimeType: string;
  originalFileName: string;
  storedFileName: string;
  storedPath: string;
  sizeBytes: number;
};

type SyncConfig = {
  conversations: Record<string, { enabled: boolean; updatedAt: string }>;
};

export type QiyuanSyncAgentOptions = {
  storageDir: string;
  manifestPath?: string;
  configPath?: string;
  log?: (message: string) => void;
};

export class QiyuanSyncAgent implements Agent {
  private storageDir: string;
  private manifestPath: string;
  private configPath: string;
  private log: (message: string) => void;

  constructor(options: QiyuanSyncAgentOptions) {
    this.storageDir = path.resolve(options.storageDir);
    this.manifestPath = path.resolve(
      options.manifestPath ?? path.join(this.storageDir, "manifest.jsonl"),
    );
    this.configPath = path.resolve(options.configPath ?? path.join(this.storageDir, "groups.json"));
    this.log = options.log ?? (() => undefined);
  }

  async chat(request: ChatRequest): Promise<ChatResponse> {
    this.log(
      `收到微信消息 conversation=${shortId(request.conversationId)} text=${JSON.stringify(request.text)} media=${request.media?.type ?? "none"}`,
    );

    if (!request.media) {
      const text = request.text.trim();
      if (isEnableCommand(text)) {
        await this.setConversationEnabled(request.conversationId, true);
        return {
          text: [
            "已开启此会话资料同步",
            "之后发送到这里的图片、文件、视频和语音会自动归档。",
          ].join("\n"),
        };
      }
      if (isDisableCommand(text)) {
        await this.setConversationEnabled(request.conversationId, false);
        return { text: "已暂停此会话资料同步。" };
      }
      if (isStatusCommand(text)) {
        const enabled = await this.isConversationEnabled(request.conversationId);
        return {
          text: enabled
            ? "此会话资料同步已开启。"
            : "此会话资料同步未开启。发送“同步此群”或“开启同步”后再发资料。",
        };
      }
      if (text === "/help" || text === "帮助") {
        return {
          text:
            "把我拉进群后，在群里发送“同步此群”开启白名单；之后群文件、图片、视频或语音会自动保存并记录 manifest。发送“停止同步此群”可暂停。",
        };
      }
      return { text: "收到。发送“同步此群”开启资料同步，或直接发送“帮助”查看用法。" };
    }

    if (!(await this.isConversationEnabled(request.conversationId))) {
      this.log(`跳过未开启同步的会话 conversation=${shortId(request.conversationId)}`);
      return { text: "此会话尚未开启资料同步。请先发送“同步此群”或“开启同步”。" };
    }

    const record = await this.persistMedia(request);
    this.log(
      `已同步 ${record.mediaType} ${record.originalFileName} -> ${record.storedPath} (${formatBytes(record.sizeBytes)})`,
    );
    return {
      text: [
        "已同步资料",
        `文件名: ${record.originalFileName}`,
        `类型: ${record.mediaType}`,
        `大小: ${formatBytes(record.sizeBytes)}`,
        `记录: ${record.id}`,
      ].join("\n"),
    };
  }

  private async persistMedia(request: ChatRequest): Promise<SyncRecord> {
    const media = request.media;
    if (!media) {
      throw new Error("media is required");
    }

    const now = new Date();
    const datePart = now.toISOString().slice(0, 10);
    const targetDir = path.join(this.storageDir, datePart);
    await fs.mkdir(targetDir, { recursive: true });
    await fs.mkdir(path.dirname(this.manifestPath), { recursive: true });

    const originalFileName =
      sanitizeFileName(media.fileName) ||
      sanitizeFileName(path.basename(media.filePath)) ||
      `${media.type}.bin`;
    const id = `${datePart}-${randomId()}`;
    const storedFileName = `${id}-${originalFileName}`;
    const storedPath = path.join(targetDir, storedFileName);

    await fs.copyFile(media.filePath, storedPath);
    const stat = await fs.stat(storedPath);

    const record: SyncRecord = {
      id,
      conversationId: request.conversationId,
      receivedAt: now.toISOString(),
      text: request.text,
      mediaType: media.type,
      mimeType: media.mimeType,
      originalFileName,
      storedFileName,
      storedPath,
      sizeBytes: stat.size,
    };

    await fs.appendFile(this.manifestPath, `${JSON.stringify(record)}\n`, "utf8");
    this.log(`manifest 已追加: ${this.manifestPath}`);
    return record;
  }

  private async isConversationEnabled(conversationId: string): Promise<boolean> {
    if (!conversationId) {
      return false;
    }
    const config = await this.loadConfig();
    return config.conversations[conversationId]?.enabled === true;
  }

  private async setConversationEnabled(conversationId: string, enabled: boolean): Promise<void> {
    if (!conversationId) {
      throw new Error("conversationId is required");
    }
    const config = await this.loadConfig();
    config.conversations[conversationId] = { enabled, updatedAt: new Date().toISOString() };
    await fs.mkdir(path.dirname(this.configPath), { recursive: true });
    await fs.writeFile(this.configPath, `${JSON.stringify(config, null, 2)}\n`, "utf8");
    this.log(`${enabled ? "开启" : "暂停"}同步 conversation=${shortId(conversationId)} config=${this.configPath}`);
  }

  private async loadConfig(): Promise<SyncConfig> {
    try {
      const raw = await fs.readFile(this.configPath, "utf8");
      const parsed = JSON.parse(raw) as Partial<SyncConfig>;
      return {
        conversations:
          parsed.conversations && typeof parsed.conversations === "object"
            ? parsed.conversations
            : {},
      };
    } catch (err) {
      if (isNodeError(err) && err.code === "ENOENT") {
        return { conversations: {} };
      }
      throw err;
    }
  }
}

function sanitizeFileName(value: string | undefined): string {
  if (!value) {
    return "";
  }
  return value.replace(/[\\/:*?"<>|\u0000-\u001f]/g, "_").trim();
}

function randomId(): string {
  return Math.random().toString(36).slice(2, 10);
}

function isEnableCommand(text: string): boolean {
  return ["同步此群", "开启同步", "开始同步", "同步此会话"].includes(text);
}

function isDisableCommand(text: string): boolean {
  return ["停止同步此群", "暂停同步", "关闭同步", "停止同步此会话"].includes(text);
}

function isStatusCommand(text: string): boolean {
  return ["同步状态", "查看同步状态"].includes(text);
}

function shortId(value: string): string {
  if (!value) {
    return "-";
  }
  return value.length <= 12 ? value : `${value.slice(0, 6)}...${value.slice(-4)}`;
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}

function formatBytes(value: number): string {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
