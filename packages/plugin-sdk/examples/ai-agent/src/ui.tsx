// 对话界面：侧边栏整页（左侧会话列表 + 对话）与右侧抽屉（只有对话）共用 ChatView。
import {
  Badge,
  Box,
  Button,
  CodeBlock,
  Collapsible,
  Column,
  Divider,
  Empty,
  Icon,
  IconButton,
  Markdown,
  Row,
  ScrollArea,
  Select,
  Text,
  Textarea,
  useEffect,
  useState,
} from 'lunabox';
import type { Context, VNode } from 'lunabox';
import type { ChatSnapshot, ChatStore, UIBlock, UIMessage, UIToolCall } from './chat';
import { modelLabel } from './model';
import { clip } from './tools';

/** 一次最多渲染多少条消息 */
const PAGE = 24;
/** 描述树最多 2000 个节点：按估算的节点数给消息列表留出预算，超出的更早消息不渲染 */
const NODE_BUDGET = 1400;
/** 一条回复里最多展开显示多少张工具卡片，更早的折叠成一行 */
const MAX_TOOL_CARDS = 30;
const TOOL_CARD_COST = 9;

function messageCost(m: UIMessage): number {
  let tools = 0;
  let other = 4;
  for (const b of m.blocks) {
    if (b.kind === 'tool') tools++;
    else other += 2;
  }
  return other + Math.min(tools, MAX_TOOL_CARDS) * TOOL_CARD_COST;
}
/** Markdown 单个字符串上限 64 KiB，留余量 */
const MAX_TEXT = 60000;

const SUGGESTIONS = [
  '列出我正在玩和最近玩过的游戏',
  '帮我把「想玩」的游戏按题材整理到收藏夹，先给我看方案',
  '帮我给没有标签的游戏补上标签',
  '根据官网把一个新游戏加入游戏库：',
];

function useChat(store: ChatStore): ChatSnapshot {
  const [, setTick] = useState(0);
  useEffect(() => store.subscribe(() => setTick((n) => n + 1)), []);
  return store.snapshot();
}

function formatTime(ts: number): string {
  const d = new Date(ts);
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  if (d.toDateString() === now.toDateString()) return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

/** 工具卡片标题里的参数摘要：取第一个字符串参数 */
function argsSummary(call: UIToolCall): string {
  try {
    const args = JSON.parse(call.args || '{}');
    for (const key of ['query', 'name', 'url', 'command', 'id', 'action']) {
      if (typeof args[key] === 'string' && args[key]) return clip(args[key], 40).split('\n')[0];
    }
    if (Array.isArray(args.ids)) return `${args.ids.length} 项`;
  } catch {
    /* 参数还在生成 */
  }
  return '';
}

function ToolCard({ call, k }: { call: UIToolCall; k: string }) {
  const summary = argsSummary(call);
  const status = call.status === 'error' ? 'error' : call.status === 'done' ? 'done' : 'running';
  const title = `${call.label || call.name || '工具'}${summary ? ` · ${summary}` : ''}`;
  return (
    <Collapsible k={k} title={title} icon={call.icon || 'toolbox-outline'} status={status}>
      <Column gap="sm" width="full">
        <Row gap="sm" align="center">
          <Text text="参数" size="xs" weight="semibold" tone="muted" />
          <Text text={call.name} size="xs" tone="faint" />
          {call.durationMs ? <Text text={`${(call.durationMs / 1000).toFixed(1)} 秒`} size="xs" tone="faint" /> : null}
        </Row>
        <CodeBlock k={`${k}-args`} code={call.args || (call.status === 'streaming' ? '…' : '{}')} language="json" />
        <Text text={call.status === 'error' ? '错误' : '结果'} size="xs" weight="semibold" tone={call.status === 'error' ? 'danger' : 'muted'} />
        {call.result !== undefined ? (
          <CodeBlock k={`${k}-result`} code={call.result} copyable={true} />
        ) : (
          <Text text={call.status === 'streaming' ? '正在生成参数…' : '执行中…'} size="sm" tone="muted" />
        )}
      </Column>
    </Collapsible>
  );
}

function AssistantBlocks({ ctx, msg }: { ctx: Context; msg: UIMessage }) {
  const toolIdx = msg.blocks.map((b, i) => (b.kind === 'tool' ? i : -1)).filter((i) => i >= 0);
  const foldedTools = new Set(toolIdx.slice(0, Math.max(0, toolIdx.length - MAX_TOOL_CARDS)));
  const nodes: VNode[] = msg.blocks.map((b: UIBlock, i: number) => {
    const k = `${msg.id}-${i}`;
    if (b.kind === 'tool') {
      if (!foldedTools.has(i)) return <ToolCard k={k} call={b.call} />;
      // 折叠的工具调用只在第一张的位置显示一行汇总
      if (i !== toolIdx[0]) return null;
      return (
        <Row k={`${msg.id}-folded`} gap="sm" align="center">
          <Icon name="toolbox-outline" size={16} tone="muted" />
          <Text text={`前面还有 ${foldedTools.size} 次工具调用已折叠`} size="xs" tone="muted" />
        </Row>
      );
    }
    if (b.kind === 'thinking') {
      if (!b.text.trim()) return null;
      return (
        <Collapsible k={k} title="思考过程" icon="information-outline">
          <Text text={clip(b.text, MAX_TEXT)} size="sm" tone="muted" selectable={true} />
        </Collapsible>
      );
    }
    if (!b.text) return null;
    // 只有正在生成的最后一段文字按流式显示
    const streaming = !!msg.streaming && i === msg.blocks.length - 1;
    return <Markdown k={k} content={clip(b.text, MAX_TEXT)} streaming={streaming} />;
  });
  const empty = !msg.blocks.some((b) => b.kind === 'tool' || b.text.trim());
  return (
    <Column gap="sm" width="full">
      {nodes}
      {msg.streaming && empty ? (
        <Row gap="sm" align="center">
          <Icon name="dots-horizontal" size={18} tone="muted" />
          <Text text="正在思考…" size="sm" tone="muted" />
        </Row>
      ) : null}
    </Column>
  );
}

function MessageView({ ctx, store, msg, last, running }: { ctx: Context; store: ChatStore; msg: UIMessage; last: boolean; running: boolean }) {
  if (msg.role === 'user') {
    const text = msg.blocks.map((b) => (b.kind === 'text' ? b.text : '')).join('');
    return (
      <Row k={msg.id} justify="end" width="full">
        <Box maxWidth={560} surface="chip" radius="lg" paddingX="md" paddingY="sm">
          <Text text={clip(text, MAX_TEXT)} size="sm" tone="strong" selectable={true} />
        </Box>
      </Row>
    );
  }
  return (
    <Row k={msg.id} gap="md" align="start" width="full">
      <Box width={28} height={28} radius="full" surface="chip" align="center" justify="center" shrink={0}>
        <Icon name="robot-happy" size={18} tone="accent" />
      </Box>
      <Column grow={1} basis={0} minWidth={0} gap="sm">
        <AssistantBlocks ctx={ctx} msg={msg} />
        {msg.error ? (
          <Row gap="sm" align="center" wrap={true}>
            <Icon name="alert-circle-outline" size={16} tone="danger" />
            <Text text={clip(msg.error, 2000)} size="sm" tone="danger" selectable={true} />
            {last && !running ? <Button k={`${msg.id}-retry`} label="重试" icon="refresh" size="sm" variant="ghost" onPress={() => store.retry()} /> : null}
          </Row>
        ) : null}
      </Column>
    </Row>
  );
}

function Welcome({ ctx, store, configured }: { ctx: Context; store: ChatStore; configured: boolean }) {
  if (!configured) {
    return (
      <Column gap="md" align="center" paddingY="xl" width="full">
        <Empty icon="robot-happy" title="还没有配置 AI 服务" description="在「设置 → AI」填写 API Key，或在本插件的设置里单独配置 OpenAI 兼容 / Anthropic 接口。" />
        <Row gap="sm">
          <Button k="go-settings" label="打开 AI 设置" icon="cog-outline" variant="primary" onPress={() => ctx.ui.navigate('/settings')} />
          <Button k="go-plugin" label="插件设置" icon="application-cog" onPress={() => ctx.ui.navigate('/plugins/example-ai-agent')} />
        </Row>
      </Column>
    );
  }
  return (
    <Column gap="lg" align="center" paddingY="xl" width="full">
      <Column gap="xs" align="center">
        <Icon name="robot-happy" size={40} tone="accent" />
        <Text text="有什么可以帮你整理的？" size="xl" weight="semibold" tone="strong" />
        <Text text="我可以查询和整理游戏库、从元数据源或官网获取信息入库，修改前会先征求你的同意。" size="sm" tone="muted" />
      </Column>
      <Column gap="sm" width="full" maxWidth={520}>
        {SUGGESTIONS.map((s, i) => (
          <Box
            k={`suggest-${i}`}
            surface="card"
            border={true}
            radius="md"
            paddingX="md"
            paddingY="sm"
            hoverBackground="accentSoft"
            label={s}
            onPress={() => (s.endsWith('：') ? store.prefill(s) : void store.send(s))}
          >
            <Row gap="sm" align="center">
              <Icon name="arrow-right" size={16} tone="muted" />
              <Text text={s} size="sm" />
            </Row>
          </Box>
        ))}
      </Column>
    </Column>
  );
}

function Composer({ store, snap }: { store: ChatStore; snap: ChatSnapshot }) {
  // 输入内容由宿主保存（见插件系统 §9.7）：value 只在预填或发送后变化，打字时不回写，避免覆盖正在输入的文字；
  // 发送或预填时用新的 k 重建输入框，清掉宿主侧的输入状态
  const [text, setText] = useState(snap.draft);
  useEffect(() => setText(snap.draft), [snap.inputGen]);
  const send = (value?: string) => {
    const v = (value ?? text).trim();
    if (!v || snap.running) return;
    void store.send(v);
  };
  return (
    <Column gap="xs" width="full">
      <Row gap="sm" align="end" width="full">
        <Box grow={1} basis={0} minWidth={0}>
          <Textarea
            k={`composer-${snap.inputGen}`}
            value={snap.draft}
            rows={2}
            autoResize={true}
            placeholder={snap.running ? 'AI 正在回复…' : '给 AI 发消息，Ctrl / ⌘ + Enter 发送'}
            onChange={setText}
            onSubmit={(v) => send(v)}
          />
        </Box>
        {snap.running ? (
          <Button k="stop" label="停止" icon="stop" variant="danger" onPress={() => store.abort()} />
        ) : (
          <Button k="send" label="发送" icon="arrow-up" variant="primary" disabled={!text.trim()} onPress={() => send()} />
        )}
      </Row>
      <Text text={`${snap.settings.protocol === 'anthropic' ? 'Anthropic' : 'OpenAI 兼容'} · ${modelLabel(snap.settings)}`} size="xs" tone="faint" />
    </Column>
  );
}

export function ChatView({ ctx, store, compact }: { ctx: Context; store: ChatStore; compact?: boolean }) {
  const snap = useChat(store);
  const [visible, setVisible] = useState(PAGE);
  useEffect(() => setVisible(PAGE), [snap.activeId]);

  const configured = !!snap.settings.apiKey;
  const msgs = snap.messages;
  // 从最新的消息往前取，直到条数或节点预算用完
  let start = msgs.length;
  let cost = 0;
  while (start > 0 && msgs.length - start < visible) {
    const next = cost + messageCost(msgs[start - 1]);
    if (next > NODE_BUDGET && start < msgs.length) break;
    cost = next;
    start--;
  }
  const shown = msgs.slice(start);
  const hidden = start;
  const budgetFull = hidden > 0 && msgs.length - start < visible;

  return (
    <Column gap="sm" height="full" width="full" padding={compact ? 'sm' : 'none'}>
      <Row justify="between" align="center" width="full" gap="sm">
        {compact ? (
          <Box grow={1} basis={0} minWidth={0}>
            <Select
              k="session-select"
              value={snap.activeId}
              disabled={snap.running}
              options={snap.sessions.map((s) => ({ value: s.id, label: s.title }))}
              onChange={(id) => store.switchSession(id)}
            />
          </Box>
        ) : (
          <Row gap="sm" align="center" minWidth={0}>
            <Text text={snap.sessions.find((s) => s.id === snap.activeId)?.title || '新对话'} size="lg" weight="semibold" tone="strong" maxLines={1} />
            {snap.running ? <Badge label="回复中" tone="brand" /> : null}
          </Row>
        )}
        <Row gap="xs" align="center" shrink={0}>
          <Badge label={modelLabel(snap.settings)} tone={configured ? 'success' : 'warning'} />
          <IconButton k="new-chat" icon="plus" label="新对话" size="sm" disabled={snap.running} onPress={() => store.newSession()} />
        </Row>
      </Row>
      <Divider />
      <ScrollArea k={`chat-${snap.activeId}`} grow={1} autoScrollBottom={true} gap="lg" padding="sm">
        {hidden > 0 && budgetFull ? (
          <Row justify="center" width="full">
            <Text text={`更早的 ${hidden} 条消息未显示（对话较长），可以开一个新对话继续`} size="xs" tone="faint" />
          </Row>
        ) : hidden > 0 ? (
          <Row justify="center" width="full">
            <Button k="more" label={`显示更早的 ${Math.min(hidden, PAGE)} 条消息`} size="sm" variant="ghost" onPress={() => setVisible(visible + PAGE)} />
          </Row>
        ) : null}
        {msgs.length === 0 ? <Welcome ctx={ctx} store={store} configured={configured} /> : null}
        {shown.map((m, i) => (
          <MessageView ctx={ctx} store={store} msg={m} last={i === shown.length - 1} running={snap.running} />
        ))}
      </ScrollArea>
      <Composer store={store} snap={snap} />
    </Column>
  );
}

function SessionList({ store }: { store: ChatStore }) {
  const snap = useChat(store);
  return (
    <Column width={220} shrink={0} height="full" gap="sm">
      <Button k="new-session" label="新对话" icon="plus" variant="secondary" disabled={snap.running} onPress={() => store.newSession()} />
      <ScrollArea k="sessions" grow={1} gap="xs">
        {snap.sessions.map((s) => {
          const active = s.id === snap.activeId;
          return (
            <Row
              k={`session-${s.id}`}
              gap="xs"
              align="center"
              width="full"
              radius="md"
              paddingX="sm"
              paddingY="xs"
              background={active ? 'accentSoft' : undefined}
              hoverBackground={active ? undefined : 'chipBg'}
              label={s.title}
              onPress={() => store.switchSession(s.id)}
            >
              <Column grow={1} basis={0} minWidth={0} gap={2}>
                <Text text={s.title} size="sm" weight={active ? 'semibold' : 'normal'} tone={active ? 'accent' : 'default'} maxLines={1} />
                <Text text={formatTime(s.updatedAt)} size="xs" tone="faint" />
              </Column>
              {active && snap.running ? null : (
                <IconButton k={`del-${s.id}`} icon="delete-outline" label="删除对话" size="sm" onPress={() => store.deleteSession(s.id)} />
              )}
            </Row>
          );
        })}
      </ScrollArea>
    </Column>
  );
}

export function createChatPage(ctx: Context, store: ChatStore) {
  return function ChatPage() {
    return (
      <Row gap="lg" height="full" width="full" align="stretch">
        <SessionList store={store} />
        <Box width={1} background="divider" />
        <Column grow={1} basis={0} minWidth={0} height="full">
          <ChatView ctx={ctx} store={store} />
        </Column>
      </Row>
    );
  };
}

export function createChatDrawer(ctx: Context, store: ChatStore) {
  return function ChatDrawer() {
    return <ChatView ctx={ctx} store={store} compact={true} />;
  };
}
