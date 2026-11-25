// 交给 AI 的工具：查询与整理游戏库、刮削网页、按元数据入库、执行命令。
// 工具抛出的异常会被 pi 当作 isError 的工具结果交回模型，所以这里出错直接 throw。
import type { Context, Game, GamePatch } from 'lunabox';
import type { AgentSettings } from './model';

export interface ToolResult {
  content: Array<{ type: 'text'; text: string }>;
  details: unknown;
}

export interface LibraryTool {
  name: string;
  /** 界面上显示的工具名与图标（宿主内置的 MDI 图标） */
  label: string;
  icon: string;
  description: string;
  parameters: Record<string, unknown>;
  execute: (toolCallId: string, params: any, signal?: AbortSignal) => Promise<ToolResult>;
}

const MAX_RESULT_CHARS = 20000;

export function clip(text: string, max: number): string {
  if (text.length <= max) return text;
  return text.slice(0, max) + `\n…（已截断，共 ${text.length} 字符）`;
}

function ok(data: unknown): ToolResult {
  const text = typeof data === 'string' ? data : JSON.stringify(data, null, 1);
  return { content: [{ type: 'text', text: clip(text ?? 'null', MAX_RESULT_CHARS) }], details: data };
}

const STATUS_VALUES = ['not_started', 'want_to_play', 'playing', 'completed', 'on_hold', 'dropped'];
export const STATUS_LABELS: Record<string, string> = {
  not_started: '未开始',
  want_to_play: '想玩',
  playing: '游玩中',
  completed: '已通关',
  on_hold: '搁置',
  dropped: '弃坑',
};

/** 发给模型的游戏摘要：去掉封面路径、时间戳等噪音，节省上下文 */
function brief(g: Game) {
  return {
    id: g.id,
    name: g.name,
    aliases: g.aliases?.length ? g.aliases : undefined,
    company: g.company || undefined,
    status: g.status,
    rating: g.rating || undefined,
    release_date: g.release_date || undefined,
    path: g.path || undefined,
    source: g.source_type ? `${g.source_type}:${g.source_id}` : undefined,
    last_played_at: g.last_played_at || undefined,
  };
}

const str = (description: string) => ({ type: 'string', description });
const num = (description: string) => ({ type: 'number', description });
const strArray = (description: string) => ({ type: 'array', items: { type: 'string' }, description });

const GAME_FIELDS = {
  name: str('游戏名称'),
  aliases: strArray('别名列表'),
  company: str('开发商 / 品牌'),
  summary: str('简介'),
  rating: num('评分，10 分制'),
  release_date: str('发售日期，如 2024-05-31'),
  status: { type: 'string', enum: STATUS_VALUES, description: '游玩状态' },
  path: str('启动程序的完整路径'),
  game_directory: str('游戏根目录'),
  save_path: str('存档目录'),
  process_name: str('实际游戏进程名（启动器与游戏进程不同时）'),
  launch_mode: { type: 'string', enum: ['normal', 'admin', 'steam', 'compatibility'], description: '启动方式' },
  use_locale_emulator: { type: 'boolean', description: '是否用 Locale Emulator 转区启动' },
  use_magpie: { type: 'boolean', description: '是否用 Magpie 缩放' },
  is_nsfw: { type: 'boolean', description: '是否为 NSFW' },
  cover_url: str('封面图片地址（http/https）'),
};

/** 元数据结果的字段在 Go 侧没有 json tag，大小写两种都兼容 */
function pickMeta(r: any) {
  const game = r?.Game ?? r?.game ?? {};
  const tags = (r?.Tags ?? r?.tags ?? []).map((t: any) => t?.Name ?? t?.name).filter(Boolean);
  return { source: r?.Source ?? r?.source ?? game.source_type, game, tags };
}

export function createTools(ctx: Context, settings: () => AgentSettings): LibraryTool[] {
  async function confirm(title: string, message: string, opts: { always?: boolean; danger?: boolean; confirmText?: string } = {}) {
    if (!opts.always && !settings().confirmWrites) return;
    const allowed = await ctx.ui.confirm({
      title,
      message,
      danger: opts.danger,
      confirmText: opts.confirmText || '允许',
      cancelText: '拒绝',
    });
    if (!allowed) throw new Error('用户拒绝了这个操作。不要重试，请询问用户接下来怎么做。');
  }

  async function getGame(id: string) {
    const game = await ctx.games.get(id);
    if (!game || !game.id) throw new Error(`找不到游戏 ${id}`);
    return game;
  }

  const describePatch = (patch: Record<string, unknown>) =>
    Object.entries(patch)
      .map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
      .join('\n');

  return [
    {
      name: 'search_games',
      label: '搜索游戏库',
      icon: 'magnify',
      description: '在本地游戏库中按关键字、状态筛选游戏，返回精简信息（含 id）。不传 query 时按最近游玩排序列出。',
      parameters: {
        type: 'object',
        properties: {
          query: str('名称 / 别名 / 开发商关键字'),
          status: { type: 'string', enum: STATUS_VALUES, description: '只看某个游玩状态' },
          tag: str('只看带有该标签的游戏'),
          limit: num('返回条数，默认 20，最多 100'),
          offset: num('分页偏移'),
        },
      },
      async execute(_id, p) {
        const limit = Math.min(Math.max(Number(p.limit) || 20, 1), 100);
        const res = await ctx.games.list({
          search_query: p.query || '',
          status: p.status || undefined,
          tags: p.tag ? [p.tag] : undefined,
          limit,
          offset: Number(p.offset) || 0,
        });
        return ok({ total: res.total, games: (res.games || []).map(brief) });
      },
    },
    {
      name: 'get_game',
      label: '查看游戏详情',
      icon: 'gamepad-variant-outline',
      description: '读取一个游戏的完整信息：基本字段、标签、所在收藏夹、游玩记录汇总、进度与评价。',
      parameters: { type: 'object', properties: { id: str('游戏 id') }, required: ['id'] },
      async execute(_id, p) {
        const game = await getGame(p.id);
        const [tags, categories, sessions, progress, review] = await Promise.all([
          ctx.games.tags.list(game.id).catch(() => []),
          ctx.categories.listByGame(game.id).catch(() => []),
          ctx.games.sessions.list(game.id).catch(() => []),
          ctx.games.progress.get(game.id).catch(() => null),
          ctx.games.review.get(game.id).catch(() => null),
        ]);
        const totalSeconds = sessions.reduce((n, s) => n + (s.duration || 0), 0);
        return ok({
          ...game,
          cover_url: undefined,
          tags: tags.map((t) => (t.source === 'user' ? t.name : `${t.name}（${t.source}）`)),
          categories: categories.map((c) => ({ id: c.id, name: c.name })),
          play: { sessions: sessions.length, total_hours: Math.round((totalSeconds / 3600) * 10) / 10 },
          progress,
          review,
        });
      },
    },
    {
      name: 'update_game',
      label: '修改游戏',
      icon: 'pencil-outline',
      description: '修改一个游戏的字段，只传需要修改的字段。',
      parameters: {
        type: 'object',
        properties: { id: str('游戏 id'), patch: { type: 'object', properties: GAME_FIELDS, description: '要修改的字段' } },
        required: ['id', 'patch'],
      },
      async execute(_id, p) {
        const game = await getGame(p.id);
        const patch = (p.patch || {}) as GamePatch;
        if (!Object.keys(patch).length) throw new Error('patch 为空');
        await confirm(`修改《${game.name}》？`, describePatch(patch as Record<string, unknown>));
        return ok(brief(await ctx.games.update(game.id, patch)));
      },
    },
    {
      name: 'create_game',
      label: '添加游戏',
      icon: 'plus-circle-outline',
      description:
        '手动把一个游戏加入游戏库（例如根据 web_fetch 从官网整理出的信息）。能在元数据源中找到时优先用 add_game_from_metadata。',
      parameters: {
        type: 'object',
        properties: {
          ...GAME_FIELDS,
          tags: strArray('用户标签'),
          category_ids: strArray('加入的收藏夹 id'),
        },
        required: ['name'],
      },
      async execute(_id, p) {
        const { cover_url, ...input } = p;
        await confirm(`添加《${p.name}》到游戏库？`, describePatch(p));
        const game = await ctx.games.create(input);
        let cover = '';
        if (typeof cover_url === 'string' && /^https?:\/\//.test(cover_url)) {
          cover = await ctx.games.setCoverFromURL(game.id, cover_url).catch((e) => `封面下载失败：${e?.message || e}`);
        }
        return ok({ created: brief(game), cover: cover || undefined });
      },
    },
    {
      name: 'delete_games',
      label: '删除游戏',
      icon: 'delete-outline',
      description: '从游戏库删除游戏（只删除记录，不删除硬盘上的文件）。会弹窗让用户确认。',
      parameters: { type: 'object', properties: { ids: strArray('游戏 id 列表') }, required: ['ids'] },
      async execute(_id, p) {
        const ids: string[] = (p.ids || []).filter(Boolean);
        if (!ids.length) throw new Error('ids 为空');
        const names = await Promise.all(ids.map((id) => ctx.games.get(id).then((g) => g.name, () => id)));
        await confirm(`删除 ${ids.length} 个游戏？`, names.map((n) => `· ${n}`).join('\n') + '\n\n只删除游戏库记录与游玩数据，不会删除硬盘上的游戏文件。', {
          always: true,
          danger: true,
          confirmText: '确认删除',
        });
        return ok(await ctx.games.batchDelete(ids));
      },
    },
    {
      name: 'set_game_tags',
      label: '设置标签',
      icon: 'tag-edit-outline',
      description: '把一个游戏的用户标签设为给定列表（覆盖原有用户标签，刮削来的标签不受影响）。',
      parameters: {
        type: 'object',
        properties: { id: str('游戏 id'), tags: strArray('完整的用户标签列表') },
        required: ['id', 'tags'],
      },
      async execute(_id, p) {
        const game = await getGame(p.id);
        await confirm(`设置《${game.name}》的标签？`, (p.tags || []).join('、') || '（清空用户标签）');
        const tags = await ctx.games.setTags(game.id, p.tags || []);
        return ok(tags.filter((t) => t.source === 'user').map((t) => t.name));
      },
    },
    {
      name: 'list_categories',
      label: '列出收藏夹',
      icon: 'folder-multiple',
      description: '列出所有收藏夹（含 id 与游戏数量）。',
      parameters: { type: 'object', properties: {} },
      async execute() {
        const cats = await ctx.categories.list();
        return ok(cats.map((c) => ({ id: c.id, name: c.name, emoji: c.emoji, games: c.game_count, system: c.is_system || undefined })));
      },
    },
    {
      name: 'organize_category',
      label: '整理收藏夹',
      icon: 'folder-plus-outline',
      description: '新建收藏夹，或把游戏加入 / 移出收藏夹。action=create 时传 name；add / remove 时传 category_id 与 game_ids。',
      parameters: {
        type: 'object',
        properties: {
          action: { type: 'string', enum: ['create', 'add', 'remove'] },
          name: str('新收藏夹名称（create）'),
          emoji: str('新收藏夹的 emoji（create，可选）'),
          category_id: str('收藏夹 id（add / remove）'),
          game_ids: strArray('游戏 id 列表（add / remove）'),
        },
        required: ['action'],
      },
      async execute(_id, p) {
        if (p.action === 'create') {
          if (!p.name) throw new Error('缺少 name');
          await confirm('新建收藏夹？', `${p.emoji || ''} ${p.name}`.trim());
          return ok(await ctx.categories.create({ name: p.name, emoji: p.emoji }));
        }
        const ids: string[] = p.game_ids || [];
        if (!p.category_id || !ids.length) throw new Error('缺少 category_id 或 game_ids');
        const cat = await ctx.categories.get(p.category_id);
        const verb = p.action === 'remove' ? '移出' : '加入';
        await confirm(`把 ${ids.length} 个游戏${verb}「${cat.name}」？`, ids.join('\n'));
        if (p.action === 'remove') await ctx.categories.removeGames(cat.id, ids);
        else await ctx.categories.addGames(cat.id, ids);
        return ok({ category: cat.name, [p.action === 'remove' ? 'removed' : 'added']: ids.length });
      },
    },
    {
      name: 'search_metadata',
      label: '搜索元数据',
      icon: 'database-search',
      description: '按游戏名在已配置的元数据源（Bangumi、VNDB 等）中搜索，返回候选条目（source + source_id）。',
      parameters: { type: 'object', properties: { name: str('游戏名') }, required: ['name'] },
      async execute(_id, p) {
        const results = (await ctx.metadata.search(p.name)) as unknown[];
        return ok(
          (results || []).slice(0, 10).map((r) => {
            const m = pickMeta(r);
            return {
              source: m.source,
              source_id: m.game.source_id,
              name: m.game.name,
              company: m.game.company || undefined,
              release_date: m.game.release_date || undefined,
              summary: clip(String(m.game.summary || ''), 200) || undefined,
            };
          })
        );
      },
    },
    {
      name: 'add_game_from_metadata',
      label: '按元数据入库',
      icon: 'database-plus-outline',
      description: '用元数据源的条目（search_metadata 得到的 source + source_id）创建游戏，自动带上简介、封面与关联。',
      parameters: {
        type: 'object',
        properties: {
          source: str('元数据源，如 bangumi、vndb'),
          source_id: str('条目 id'),
          path: str('本地启动程序路径（可选）'),
          status: { type: 'string', enum: STATUS_VALUES, description: '游玩状态（可选）' },
        },
        required: ['source', 'source_id'],
      },
      async execute(_id, p) {
        const m = pickMeta(await ctx.metadata.fetch(p.source, p.source_id));
        if (!m.game?.name) throw new Error('元数据源没有返回游戏信息');
        const existing = await ctx.games.findBySource(p.source, p.source_id);
        if (existing) return ok({ already_in_library: brief(existing) });
        await confirm(`添加《${m.game.name}》到游戏库？`, `来源：${p.source} / ${p.source_id}\n开发商：${m.game.company || '—'}`);
        const fields: Record<string, unknown> = {};
        for (const k of ['name', 'aliases', 'company', 'summary', 'rating', 'release_date']) {
          if (m.game[k] !== undefined && m.game[k] !== '') fields[k] = m.game[k];
        }
        const game = await ctx.games.create({
          ...(fields as GamePatch),
          name: String(m.game.name),
          path: p.path || undefined,
          status: p.status || undefined,
          source_type: p.source,
          source_id: p.source_id,
          metadata_sources: [{ source: p.source, source_id: p.source_id }],
        });
        if (/^https?:\/\//.test(String(m.game.cover_url || ''))) {
          await ctx.games.setCoverFromURL(game.id, m.game.cover_url).catch(() => undefined);
        }
        return ok({ created: brief(game), tags_from_source: m.tags.slice(0, 15) });
      },
    },
    {
      name: 'web_fetch',
      label: '读取网页',
      icon: 'web',
      description:
        '抓取网页并提取标题、描述、封面候选图与正文文本，用于从游戏官网或资料站整理信息。可用 CSS selector 只取页面的一部分。',
      parameters: {
        type: 'object',
        properties: {
          url: str('http/https 网址'),
          selector: str('只提取匹配该 CSS 选择器的部分（可选）'),
          max_chars: num('正文最多返回多少字符，默认 12000'),
        },
        required: ['url'],
      },
      async execute(_id, p, signal) {
        const url = String(p.url || '');
        if (!/^https?:\/\//i.test(url)) throw new Error('只支持 http/https 网址');
        const res = await fetch(url, {
          headers: { Accept: 'text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8' },
          signal,
        });
        const type = res.headers.get('content-type') || '';
        const body = await res.text();
        const maxChars = Math.min(Math.max(Number(p.max_chars) || 12000, 500), 60000);
        if (!/html/i.test(type) && !/^\s*</.test(body)) {
          return ok({ url: res.url || url, status: res.status, content_type: type, text: clip(body, maxChars) });
        }
        const cleaned = body
          .replace(/<(script|style|noscript|svg|template|iframe)\b[\s\S]*?<\/\1>/gi, ' ')
          .replace(/<!--[\s\S]*?-->/g, ' ')
          .replace(/<(br|hr)\b[^>]*>/gi, '\n')
          .replace(/<\/(p|div|li|h[1-6]|tr|section|article|dd|dt|table|ul|ol)>/gi, '$&\n');
        const doc = ctx.html.parse(cleaned);
        const meta = (sel: string) => doc.select(sel).first().attr('content') || '';
        const abs = (src: string) => {
          try {
            return new URL(src, res.url || url).href;
          } catch {
            return src;
          }
        };
        const root = p.selector ? doc.select(p.selector) : doc.select('body');
        const text = (root.length() ? root.text() : doc.text())
          .replace(/[ \t\f\v ]+/g, ' ')
          .replace(/\s*\n\s*/g, '\n')
          .replace(/\n{3,}/g, '\n\n')
          .trim();
        const images = doc
          .select('img')
          .map((el) => el.attr('src') || el.attr('data-src') || '')
          .filter((s) => s && !s.startsWith('data:'))
          .slice(0, 15)
          .map(abs);
        const ogImage = meta('meta[property="og:image"]');
        return ok({
          url: res.url || url,
          status: res.status,
          title: doc.select('title').first().text().trim(),
          description: meta('meta[name="description"]') || meta('meta[property="og:description"]') || undefined,
          og_image: ogImage ? abs(ogImage) : undefined,
          images: images.length ? images : undefined,
          text: clip(text, maxChars),
        });
      },
    },
    {
      name: 'run_command',
      label: '执行命令',
      icon: 'console-line',
      description:
        `在用户电脑上执行一条命令（${ctx.process.platform === 'windows' ? 'Windows cmd.exe' : '/bin/sh'}），返回退出码与输出。每次执行前都会弹窗让用户确认，不要用它做破坏性操作。`,
      parameters: {
        type: 'object',
        properties: {
          command: str('完整的命令行'),
          cwd: str('工作目录（可选）'),
          reason: str('向用户说明为什么要执行这条命令'),
        },
        required: ['command', 'reason'],
      },
      async execute(_id, p) {
        await confirm('允许 AI 执行命令？', `${p.reason || ''}\n\n${p.command}${p.cwd ? `\n\n工作目录：${p.cwd}` : ''}`, {
          always: true,
          danger: true,
          confirmText: '执行',
        });
        const r = await ctx.process.exec(p.command, { shell: true, cwd: p.cwd || undefined, timeout: 120000 });
        return ok({
          code: r.code,
          timed_out: r.timedOut || undefined,
          stdout: clip(r.stdout, 12000),
          stderr: r.stderr ? clip(r.stderr, 4000) : undefined,
        });
      },
    },
  ];
}
