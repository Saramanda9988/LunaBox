// AI 游戏库助手：类 Claude / Codex 桌面端的对话工作台。
// 模型适配与 agent 循环由打包进来的 pi（@earendil-works/pi-ai + pi-agent-core）负责，宿主只提供 fetch、ctx.* 等基础能力。
import type { Context, Game } from 'lunabox';
import { ChatStore, type ChatSnapshot } from './chat';
import { CONFIG_SCHEMA } from './model';
import { createChatDrawer, createChatPage } from './ui';

const DRAWER_ID = 'ai-agent-drawer';

let current: ChatStore | null = null;

export function apply(ctx: Context) {
  ctx.config.define(CONFIG_SCHEMA);
  const store = new ChatStore(ctx);
  current = store;
  ctx.config.onChange(() => store.reloadSettings());
  ctx.effect(() => () => {
    store.abort();
    if (current === store) current = null;
  });

  ctx.ui.slot('sidebar.page', {
    id: 'ai-agent',
    title: 'AI 助手',
    icon: 'robot-happy',
    render: createChatPage(ctx, store),
  });

  ctx.ui.slot('workspace.drawer', {
    id: DRAWER_ID,
    title: 'AI 助手',
    icon: 'robot-happy',
    defaultWidth: 440,
    render: createChatDrawer(ctx, store),
  });

  ctx.ui.slot('topbar.item', {
    id: 'ai-agent-toggle',
    title: 'AI 助手',
    icon: 'robot-happy',
    onPress: () => ctx.ui.openDrawer(DRAWER_ID),
  });

  ctx.ui.slot('library.toolbar', {
    id: 'ai-agent-organize',
    title: '让 AI 整理选中的游戏',
    icon: 'robot-happy',
    mode: 'batch',
    onPress: (payload?: { selectedGameIds?: string[] }) => {
      const ids = payload?.selectedGameIds || [];
      if (!ids.length) return;
      ctx.ui.openDrawer(DRAWER_ID);
      store.prefill(`请帮我看看这 ${ids.length} 个游戏（id：${ids.join(', ')}），给出整理建议：`);
    },
  });

  ctx.ui.slot('game.actions', {
    id: 'ai-agent-ask',
    title: '问问 AI',
    icon: 'robot-happy',
    run: (game: Game) => {
      ctx.ui.openDrawer(DRAWER_ID);
      store.prefill(`关于《${game.name}》（id：${game.id}）：`);
    },
  });
}

/**
 * 发送一条消息并等待这一轮结束，返回对话快照。供自动化测试与调试使用（插件运行时里调用）。
 */
export async function sendForTest(text: string): Promise<ChatSnapshot | null> {
  if (!current) return null;
  await current.send(text);
  return current.snapshot();
}
