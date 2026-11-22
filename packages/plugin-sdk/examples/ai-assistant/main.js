import {
  useState, useEffect, createAIChatStore,
  Column, Row, ScrollArea, Markdown, Collapsible, Textarea, Button, Text, Badge, Divider,
} from 'lunabox';

export function apply(ctx) {
  const tools = [
    {
      type: 'function',
      function: {
        name: 'list_games',
        description: '列出本地游戏库中的游戏（支持关键字搜索）',
        parameters: {
          type: 'object',
          properties: { query: { type: 'string' }, limit: { type: 'number' } },
        },
      },
    },
    {
      type: 'function',
      function: {
        name: 'list_categories',
        description: '列出本地所有收藏夹',
        parameters: { type: 'object', properties: {} },
      },
    },
  ];

  const store = createAIChatStore(ctx, {
    system: '你是 LunaBox 游戏库助手，可以帮助用户查询和整理游戏库。',
    tools,
    async onToolCall(call) {
      if (call.name === 'list_games') {
        return await ctx.games.list({ query: call.arguments.query || '', limit: call.arguments.limit || 20 });
      }
      if (call.name === 'list_categories') {
        return await ctx.categories.list();
      }
      return { error: 'unknown tool' };
    },
    onError: (err) => ctx.ui.toast(err.message, 'error'),
  });

  function AIWorkspace() {
    const [, setTick] = useState(0);
    const [input, setInput] = useState('');
    useEffect(() => store.subscribe(() => setTick((n) => n + 1)), []);

    const { messages, streaming } = store.getState();
    const cfg = ctx.ai.getConfig();

    const send = () => {
      if (!input.trim() || streaming) return;
      const text = input.trim();
      setInput('');
      store.send(text);
    };

    return Column({ gap: 'md', height: 'full', children: [
      Row({ justify: 'between', align: 'center', children: [
        Row({ gap: 'sm', align: 'center', children: [
          Text({ text: 'AI 游戏库工作台', size: 'lg', weight: 'bold' }),
          Badge({ label: cfg.configured ? (cfg.model || '已配置') : '未配置模型', tone: cfg.configured ? 'success' : 'warning' }),
        ]}),
        Button({ k: 'clear-chat', label: '清空对话', variant: 'ghost', size: 'sm', onPress: () => store.clear() }),
      ]}),
      Divider(),
      ScrollArea({
        k: 'chat-scroll',
        grow: 1,
        autoScrollBottom: true,
        gap: 'md',
        children: messages.map((m, idx) =>
          Column({ k: 'msg-' + idx, gap: 'xs', children: [
            Text({ text: m.role === 'user' ? '你' : 'AI 助手', size: 'xs', weight: 'semibold', tone: 'muted' }),
            ...(m.toolCalls || []).map((tc, tIdx) =>
              Collapsible({
                k: `tc-${idx}-${tIdx}`,
                title: `调用工具: ${tc.name}`,
                status: tc.status,
                children: [Text({ text: tc.result || '执行中...', size: 'xs', tone: 'muted' })],
              })
            ),
            Markdown({ content: m.content || (streaming ? '...' : '') }),
          ]})
        ),
      }),
      Row({ gap: 'sm', align: 'end', children: [
        Textarea({ k: 'ai-input', value: input, placeholder: '输入需求（Cmd/Ctrl+Enter 发送）...', rows: 2, autoResize: true, onChange: setInput, onSubmit: send }),
        streaming
          ? Button({ k: 'stop-btn', label: '停止', variant: 'danger', onPress: () => store.abort() })
          : Button({ k: 'send-btn', label: '发送', variant: 'primary', onPress: send }),
      ]}),
    ]});
  }

  ctx.ui.slot('sidebar.page', {
    id: 'ai-workbench',
    title: 'AI 助手',
    icon: 'robot-happy',
    render: AIWorkspace,
  });

  ctx.ui.slot('workspace.drawer', {
    id: 'ai-drawer',
    title: 'AI 伴随助手',
    icon: 'robot-happy',
    defaultWidth: 400,
    render: AIWorkspace,
  });

  ctx.ui.slot('topbar.item', {
    id: 'open-ai-drawer',
    title: '打开 AI 助手抽屉',
    icon: 'robot-happy',
    onPress: () => ctx.ui.openDrawer('ai-drawer'),
  });

  ctx.ui.slot('library.toolbar', {
    id: 'ai-organize',
    title: 'AI 整理选中游戏',
    icon: 'robot-happy',
    mode: 'batch',
    onPress: ({ selectedGameIds }) => {
      ctx.ui.openDrawer('ai-drawer', { selectedGameIds });
      store.send(`请帮我分析并整理这 ${selectedGameIds.length} 款游戏（ID: ${selectedGameIds.join(', ')}）的分类与标签建议。`);
    },
  });
}
