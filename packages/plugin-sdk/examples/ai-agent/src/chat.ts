// 对话状态：把 pi Agent 的事件流整理成界面消息（文本、思考、工具调用卡片），并把会话持久化到 ctx.storage。
import type { Context } from 'lunabox';
import { Agent } from '@earendil-works/pi-agent-core';
import { buildModel, readSettings, type AgentSettings } from './model';
import { clip, createTools, type LibraryTool } from './tools';

export type ToolStatus = 'streaming' | 'running' | 'done' | 'error';

export interface UIToolCall {
  id: string;
  name: string;
  label: string;
  icon?: string;
  /** 参数的 JSON 文本（生成中时是模型正在输出的片段） */
  args: string;
  status: ToolStatus;
  result?: string;
  durationMs?: number;
}

export type UIBlock =
  | { kind: 'text'; text: string }
  | { kind: 'thinking'; text: string }
  | { kind: 'tool'; call: UIToolCall };

export interface UIMessage {
  id: string;
  role: 'user' | 'assistant';
  blocks: UIBlock[];
  error?: string;
  streaming?: boolean;
  at: number;
}

interface StoredSession {
  id: string;
  title: string;
  createdAt: number;
  updatedAt: number;
  /** pi 的消息历史（交给模型的上下文） */
  messages: unknown[];
  /** 界面消息 */
  ui: UIMessage[];
}

export interface SessionMeta {
  id: string;
  title: string;
  updatedAt: number;
}

export interface ChatSnapshot {
  sessions: SessionMeta[];
  activeId: string;
  messages: UIMessage[];
  running: boolean;
  settings: AgentSettings;
  /** 输入框内容（侧边栏页与抽屉共用） */
  draft: string;
  /** 每次发送后递增，用来重建输入框（清空宿主侧保存的输入状态） */
  inputGen: number;
}

const STORAGE_KEY = 'sessions';
const MAX_SESSIONS = 30;
const NOTIFY_INTERVAL = 60;

let seq = 0;
const newId = (prefix: string) => `${prefix}-${Date.now().toString(36)}-${(seq++).toString(36)}`;

function systemPrompt(ctx: Context, s: AgentSettings): string {
  const today = new Date().toISOString().slice(0, 10);
  return [
    '你是 LunaBox 的游戏库助手。LunaBox 是一个本地游戏（以视觉小说 / Galgame 为主）管理与启动器。',
    `今天是 ${today}，用户的系统是 ${ctx.process.platform}。`,
    '你可以通过工具查询和整理用户的游戏库、从元数据源或网页获取游戏信息并入库、在用户确认后执行命令。',
    '原则：',
    '- 先用工具查清楚再回答，不要编造游戏库里的数据；需要 id 时先搜索。',
    '- 批量修改前先简要说明计划；工具调用被用户拒绝时停止该操作并询问用户。',
    '- 从网页整理游戏信息时，优先用 search_metadata 找到条目后 add_game_from_metadata；找不到时再用 web_fetch 读取官网并 create_game。',
    '- run_command 只在确有必要时使用，并在 reason 里写清楚目的。',
    '- 不剧透剧情。用用户使用的语言回答，简洁，适当使用 Markdown。',
    s.systemPrompt ? `\n用户的附加要求：${s.systemPrompt}` : '',
  ]
    .filter(Boolean)
    .join('\n');
}

/** pi 工具结果里的 details 只给界面用，持久化时去掉，避免把整份查询结果写进存储 */
function compactHistory(messages: unknown[]): unknown[] {
  return messages.map((m: any) => (m && m.role === 'toolResult' ? { ...m, details: undefined } : m));
}

function contentText(content: unknown): string {
  if (!Array.isArray(content)) return typeof content === 'string' ? content : '';
  return content
    .map((c: any) => (c?.type === 'text' ? c.text : c?.type === 'image' ? '[图片]' : ''))
    .filter(Boolean)
    .join('\n');
}

export class ChatStore {
  private sessions: StoredSession[];
  private activeId: string;
  private listeners = new Set<() => void>();
  private agent: Agent | null = null;
  private notifyTimer: ReturnType<typeof setTimeout> | null = null;
  private draft = '';
  private inputGen = 0;
  private tools: LibraryTool[];
  private settings: AgentSettings;

  constructor(private ctx: Context) {
    this.settings = readSettings(ctx);
    this.tools = createTools(ctx, () => this.settings);
    const saved = ctx.storage.get<StoredSession[]>(STORAGE_KEY);
    this.sessions = Array.isArray(saved) ? saved.filter((s) => s && s.id) : [];
    // 上次退出时还在生成的消息不会再继续
    for (const s of this.sessions) {
      for (const m of s.ui || []) if (m.streaming) m.streaming = false;
    }
    if (!this.sessions.length) this.sessions.push(this.blankSession());
    this.activeId = this.sessions[0].id;
  }

  // ─────────── 订阅 ───────────

  subscribe(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** 流式输出时合并通知，避免每个 token 都重渲染整棵描述树 */
  private notify(immediate = false) {
    if (immediate) {
      if (this.notifyTimer) clearTimeout(this.notifyTimer);
      this.notifyTimer = null;
      this.listeners.forEach((fn) => fn());
      return;
    }
    if (this.notifyTimer) return;
    this.notifyTimer = setTimeout(() => {
      this.notifyTimer = null;
      this.listeners.forEach((fn) => fn());
    }, NOTIFY_INTERVAL);
  }

  reloadSettings() {
    this.settings = readSettings(this.ctx);
    this.notify(true);
  }

  snapshot(): ChatSnapshot {
    const active = this.active();
    return {
      sessions: this.sessions.map((s) => ({ id: s.id, title: s.title, updatedAt: s.updatedAt })),
      activeId: active.id,
      messages: active.ui,
      running: !!this.agent,
      settings: this.settings,
      draft: this.draft,
      inputGen: this.inputGen,
    };
  }

  // ─────────── 会话 ───────────

  private blankSession(): StoredSession {
    const now = Date.now();
    return { id: newId('s'), title: '新对话', createdAt: now, updatedAt: now, messages: [], ui: [] };
  }

  private active(): StoredSession {
    return this.sessions.find((s) => s.id === this.activeId) || this.sessions[0];
  }

  private persist() {
    this.sessions.sort((a, b) => b.updatedAt - a.updatedAt);
    this.sessions = this.sessions.slice(0, MAX_SESSIONS);
    this.ctx.storage.set(
      STORAGE_KEY,
      this.sessions.map((s) => ({ ...s, messages: compactHistory(s.messages) }))
    );
  }

  newSession() {
    if (this.agent) return;
    const cur = this.active();
    if (!cur.ui.length) return; // 当前已经是空白对话
    const s = this.blankSession();
    this.sessions.unshift(s);
    this.activeId = s.id;
    this.persist();
    this.notify(true);
  }

  switchSession(id: string) {
    if (this.agent || id === this.activeId || !this.sessions.some((s) => s.id === id)) return;
    this.activeId = id;
    this.notify(true);
  }

  deleteSession(id: string) {
    if (this.agent && id === this.activeId) return;
    this.sessions = this.sessions.filter((s) => s.id !== id);
    if (!this.sessions.length) this.sessions.push(this.blankSession());
    if (!this.sessions.some((s) => s.id === this.activeId)) this.activeId = this.sessions[0].id;
    this.persist();
    this.notify(true);
  }

  /** 从游戏右键菜单等入口预填输入框 */
  prefill(text: string) {
    this.draft = text;
    this.inputGen++;
    this.notify(true);
  }

  // ─────────── 发送与流式事件 ───────────

  abort() {
    this.agent?.abort();
  }

  async send(rawText: string) {
    const text = rawText.trim();
    if (!text || this.agent) return;
    this.settings = readSettings(this.ctx);
    const session = this.active();
    this.draft = '';
    this.inputGen++;

    const now = Date.now();
    session.ui.push({ id: newId('u'), role: 'user', blocks: [{ kind: 'text', text }], at: now });
    const reply: UIMessage = { id: newId('a'), role: 'assistant', blocks: [], streaming: true, at: now };
    session.ui.push(reply);
    if (session.title === '新对话') session.title = clip(text.replace(/\s+/g, ' '), 30).split('\n')[0];
    session.updatedAt = now;

    const built = buildModel(this.settings);
    if (!built) {
      reply.streaming = false;
      reply.error = '还没有配置 AI 服务：请在「设置 → AI」填写 API Key，或在本插件的设置里单独配置。';
      this.persist();
      this.notify(true);
      return;
    }

    const agent = new Agent({
      initialState: {
        systemPrompt: systemPrompt(this.ctx, this.settings),
        model: built.model,
        tools: this.tools as never,
        messages: session.messages as never,
      },
      streamFn: built.models.streamSimple.bind(built.models),
      identity: null as never,
    });
    this.agent = agent;
    this.notify(true);

    // contentIndex → 当前回复里的块下标；每条新的模型消息（工具循环的下一轮）重新计数
    let blockOf = new Map<number, number>();
    const toolBlock = (id: string) =>
      reply.blocks.find((b): b is Extract<UIBlock, { kind: 'tool' }> => b.kind === 'tool' && b.call.id === id);
    const toolOf = (name: string) => this.tools.find((t) => t.name === name);
    const labelOf = (name: string) => toolOf(name)?.label || name;
    const iconOf = (name: string) => toolOf(name)?.icon;

    const unsubscribe = agent.subscribe((event: any) => {
      switch (event.type) {
        case 'message_start':
          if (event.message?.role === 'assistant') blockOf = new Map();
          break;
        case 'message_update': {
          const e = event.assistantMessageEvent;
          const idx = e.contentIndex;
          if (e.type === 'text_start' || e.type === 'thinking_start') {
            blockOf.set(idx, reply.blocks.push({ kind: e.type === 'text_start' ? 'text' : 'thinking', text: '' }) - 1);
          } else if (e.type === 'text_delta' || e.type === 'thinking_delta') {
            const b = reply.blocks[blockOf.get(idx) ?? -1];
            if (b && (b.kind === 'text' || b.kind === 'thinking')) b.text += e.delta;
          } else if (e.type === 'toolcall_start') {
            const part = e.partial?.content?.[idx] || {};
            const call: UIToolCall = {
              id: part.id || `pending-${idx}`,
              name: part.name || '',
              label: labelOf(part.name || ''),
              icon: iconOf(part.name || ''),
              args: '',
              status: 'streaming',
            };
            blockOf.set(idx, reply.blocks.push({ kind: 'tool', call }) - 1);
          } else if (e.type === 'toolcall_delta') {
            const b = reply.blocks[blockOf.get(idx) ?? -1];
            if (b?.kind === 'tool') {
              b.call.args += e.delta;
              const part = e.partial?.content?.[idx];
              if (part?.name && !b.call.name) {
                b.call.name = part.name;
                b.call.label = labelOf(part.name);
                b.call.icon = iconOf(part.name);
              }
            }
          } else if (e.type === 'toolcall_end') {
            const b = reply.blocks[blockOf.get(idx) ?? -1];
            if (b?.kind === 'tool') {
              b.call.id = e.toolCall.id;
              b.call.name = e.toolCall.name;
              b.call.label = labelOf(e.toolCall.name);
              b.call.icon = iconOf(e.toolCall.name);
              b.call.args = JSON.stringify(e.toolCall.arguments ?? {}, null, 2);
              b.call.status = 'running';
            }
          }
          break;
        }
        case 'message_end': {
          const m = event.message;
          if (m?.role === 'assistant' && (m.stopReason === 'error' || m.stopReason === 'aborted')) {
            reply.error = m.stopReason === 'aborted' ? '已停止' : m.errorMessage || '请求失败';
          }
          break;
        }
        case 'tool_execution_start': {
          const b = toolBlock(event.toolCallId);
          if (b) b.call.status = 'running';
          break;
        }
        case 'tool_execution_end': {
          const b = toolBlock(event.toolCallId);
          if (b) {
            b.call.status = event.isError ? 'error' : 'done';
            b.call.result = clip(contentText(event.result?.content) || (event.isError ? '工具执行失败' : '（无输出）'), 4000);
            b.call.durationMs = event.durationMs;
          }
          break;
        }
      }
      this.notify(event.type === 'tool_execution_end' || event.type === 'message_end');
    });

    try {
      await agent.prompt(text);
    } catch (err: any) {
      reply.error = err?.message || String(err);
    } finally {
      unsubscribe();
      for (const b of reply.blocks) {
        if (b.kind === 'tool' && (b.call.status === 'running' || b.call.status === 'streaming')) {
          b.call.status = 'error';
          b.call.result = b.call.result || '已中止';
        }
      }
      if (!reply.error && agent.state.errorMessage) reply.error = agent.state.errorMessage;
      reply.streaming = false;
      session.messages = agent.state.messages as unknown[];
      session.updatedAt = Date.now();
      this.agent = null;
      this.persist();
      this.notify(true);
    }
  }

  /** 重新发送最后一条用户消息（上次出错或被停止时） */
  retry() {
    if (this.agent) return;
    const session = this.active();
    const lastUser = [...session.ui].reverse().find((m) => m.role === 'user');
    if (!lastUser) return;
    const text = lastUser.blocks.map((b) => (b.kind === 'text' ? b.text : '')).join('');
    // 去掉失败的这一轮（界面消息与模型历史都回退到发送前）
    const at = session.ui.lastIndexOf(lastUser);
    session.ui = session.ui.slice(0, at);
    const histAt = (session.messages as any[]).map((m) => m?.role).lastIndexOf('user');
    if (histAt >= 0) session.messages = session.messages.slice(0, histAt);
    void this.send(text);
  }
}
