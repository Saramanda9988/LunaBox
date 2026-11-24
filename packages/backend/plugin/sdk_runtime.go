package plugin

import (
	"sync"

	"github.com/MoeForge/moejs"
)

// lunaboxRuntimeJS 是虚拟模块 "lunabox" 的内嵌 ESM 运行时代码。
// 它提供 JSX 构造、精简 hooks（useState / useEffect / useRef / useMemo）、
// 全部白名单组件工厂、微任务批量渲染调度，以及高阶辅助工具（createAIChatStore / createPagination / defineThemePreset）。
const lunaboxRuntimeJS = `
export const Fragment = Symbol.for('lunabox.Fragment');

const __instances = new Map();
let __currentInst = null;
let __hookIndex = 0;
let __handlerSeq = 0;

function __isEqualDeps(a, b) {
  if (!a || !b || a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (!Object.is(a[i], b[i])) return false;
  }
  return true;
}

function __scheduleRender(inst) {
  if (!inst || inst.disposed || inst.scheduled) return;
  inst.scheduled = true;
  queueMicrotask(() => {
    inst.scheduled = false;
    if (!inst.disposed) __renderInstance(inst);
  });
}

function __flattenChildren(raw, out) {
  if (raw == null || raw === false || raw === true) return;
  if (Array.isArray(raw)) {
    for (let i = 0; i < raw.length; i++) __flattenChildren(raw[i], out);
    return;
  }
  if (typeof raw === 'string' || typeof raw === 'number') {
    out.push({ t: 'Text', p: { text: String(raw) } });
    return;
  }
  if (typeof raw === 'object') {
    if (raw.__fragment) {
      __flattenChildren(raw.children, out);
      return;
    }
    if (typeof raw.type === 'function') {
      const rendered = raw.type(raw.props || {});
      __flattenChildren(rendered, out);
      return;
    }
    out.push(raw);
  }
}

function __serializeNode(vnode, handlers) {
  if (vnode == null || vnode === false || vnode === true) return null;
  if (typeof vnode === 'string' || typeof vnode === 'number') {
    return { t: 'Text', p: { text: String(vnode) } };
  }
  if (typeof vnode.type === 'function') {
    const rendered = vnode.type(vnode.props || {});
    return __serializeNode(rendered, handlers);
  }
  if (vnode.__fragment) {
    const kids = [];
    __flattenChildren(vnode.children, kids);
    const serialized = [];
    for (let i = 0; i < kids.length; i++) {
      const n = __serializeNode(kids[i], handlers);
      if (n) serialized.push(n);
    }
    return { t: 'Column', c: serialized };
  }
  if (typeof vnode.t === 'string') {
    const node = { t: vnode.t };
    if (vnode.k != null && String(vnode.k) !== '') node.k = String(vnode.k);
    if (vnode.p && typeof vnode.p === 'object') {
      const props = {};
      let hasProps = false;
      const evs = vnode.on ? Object.assign({}, vnode.on) : {};
      let hasEvs = Object.keys(evs).length > 0;
      for (const [key, val] of Object.entries(vnode.p)) {
        if (val === undefined) continue;
        if (key === 'k' || key === 'key') {
          if (!node.k && val != null) node.k = String(val);
          continue;
        }
        if (key === 'children') continue;
        if (typeof val === 'function' && key.length > 2 && key.startsWith('on') && key[2] >= 'A' && key[2] <= 'Z') {
          const evName = key[2].toLowerCase() + key.slice(3);
          const hid = 'h' + (++__handlerSeq);
          handlers.set(hid, val);
          evs[evName] = hid;
          hasEvs = true;
          continue;
        }
        if (typeof val === 'function') continue;
        props[key] = val;
        hasProps = true;
      }
      if (hasProps) node.p = props;
      if (hasEvs) node.on = evs;
    } else if (vnode.on && Object.keys(vnode.on).length > 0) {
      node.on = vnode.on;
    }
    const rawKids = vnode.c || (vnode.p ? vnode.p.children : null);
    if (rawKids != null) {
      const flat = [];
      __flattenChildren(rawKids, flat);
      if (flat.length > 0) {
        const serialized = [];
        for (let i = 0; i < flat.length; i++) {
          const n = __serializeNode(flat[i], handlers);
          if (n) serialized.push(n);
        }
        if (serialized.length > 0) node.c = serialized;
      }
    }
    return node;
  }
  return null;
}

function __renderInstance(inst) {
  if (!inst || inst.disposed) return;
  const prevInst = __currentInst;
  const prevIdx = __hookIndex;
  __currentInst = inst;
  __hookIndex = 0;
  inst.pendingEffects = [];

  try {
    const vnode = inst.renderFn(inst.props || {});
    const handlers = new Map();
    const tree = __serializeNode(vnode, handlers);
    inst.handlers = handlers;
    inst.version++;
    if (typeof globalThis.__lunabox_commitTree === 'function') {
      globalThis.__lunabox_commitTree(inst.id, inst.version, JSON.stringify(tree), '');
    }
    const effects = inst.pendingEffects;
    inst.pendingEffects = [];
    for (let i = 0; i < effects.length; i++) {
      const eff = effects[i];
      if (typeof eff.cleanup === 'function') {
        try { eff.cleanup(); } catch (e) { console.error(e); }
        eff.cleanup = null;
      }
      try {
        const res = eff.setup();
        if (typeof res === 'function') eff.cleanup = res;
      } catch (e) {
        console.error(e);
      }
    }
  } catch (err) {
    const msg = err && err.stack ? String(err.stack) : String(err);
    if (typeof globalThis.__lunabox_commitTree === 'function') {
      globalThis.__lunabox_commitTree(inst.id, inst.version, 'null', msg);
    }
  } finally {
    __currentInst = prevInst;
    __hookIndex = prevIdx;
  }
}

export function useState(initial) {
  const inst = __currentInst;
  if (!inst) throw new Error('useState must be called inside a component render');
  const idx = __hookIndex++;
  if (idx >= inst.hooks.length) {
    inst.hooks.push({ value: typeof initial === 'function' ? initial() : initial });
  }
  const slot = inst.hooks[idx];
  const setState = (next) => {
    const resolved = typeof next === 'function' ? next(slot.value) : next;
    if (!Object.is(resolved, slot.value)) {
      slot.value = resolved;
      __scheduleRender(inst);
    }
  };
  return [slot.value, setState];
}

export function useRef(initial) {
  const inst = __currentInst;
  if (!inst) throw new Error('useRef must be called inside a component render');
  const idx = __hookIndex++;
  if (idx >= inst.hooks.length) {
    inst.hooks.push({ current: initial });
  }
  return inst.hooks[idx];
}

export function useMemo(factory, deps) {
  const inst = __currentInst;
  if (!inst) throw new Error('useMemo must be called inside a component render');
  const idx = __hookIndex++;
  const prev = inst.hooks[idx];
  if (prev && __isEqualDeps(prev.deps, deps)) {
    return prev.value;
  }
  const value = factory();
  inst.hooks[idx] = { value, deps };
  return value;
}

export function useEffect(setup, deps) {
  const inst = __currentInst;
  if (!inst) throw new Error('useEffect must be called inside a component render');
  const idx = __hookIndex++;
  const prev = inst.hooks[idx];
  if (!prev) {
    const entry = { setup, deps, cleanup: null };
    inst.hooks.push(entry);
    inst.pendingEffects.push(entry);
  } else if (deps === undefined || !__isEqualDeps(prev.deps, deps)) {
    prev.setup = setup;
    prev.deps = deps;
    inst.pendingEffects.push(prev);
  }
}

export function h(type, props, ...children) {
  const p = props ? Object.assign({}, props) : {};
  if (children.length > 0) {
    p.children = children.length === 1 ? children[0] : children;
  }
  if (type === Fragment) {
    return { __fragment: true, children: p.children };
  }
  if (typeof type === 'function') {
    return { type, props: p };
  }
  const k = p.k != null ? String(p.k) : (p.key != null ? String(p.key) : undefined);
  return { t: String(type), k, p, c: p.children };
}

export function jsx(type, props, key) {
  const p = props ? Object.assign({}, props) : {};
  if (key !== undefined && p.k === undefined) p.k = String(key);
  if (type === Fragment) {
    return { __fragment: true, children: p.children };
  }
  if (typeof type === 'function') {
    return { type, props: p };
  }
  const k = p.k != null ? String(p.k) : (p.key != null ? String(p.key) : undefined);
  return { t: String(type), k, p, c: p.children };
}

export const jsxs = jsx;
export const jsxDEV = jsx;

function __makeFactory(name) {
  return function Component(props) {
    return jsx(name, props || {});
  };
}

export const Column = __makeFactory('Column');
export const Row = __makeFactory('Row');
export const Box = __makeFactory('Box');
export const Stack = __makeFactory('Stack');
export const ScrollArea = __makeFactory('ScrollArea');
export const Grid = __makeFactory('Grid');
export const Text = __makeFactory('Text');
export const Markdown = __makeFactory('Markdown');
export const CodeBlock = __makeFactory('CodeBlock');
export const Button = __makeFactory('Button');
export const IconButton = __makeFactory('IconButton');
export const Input = __makeFactory('Input');
export const Textarea = __makeFactory('Textarea');
export const Switch = __makeFactory('Switch');
export const Checkbox = __makeFactory('Checkbox');
export const Select = __makeFactory('Select');
export const NumberInput = __makeFactory('NumberInput');
export const Tabs = __makeFactory('Tabs');
export const Badge = __makeFactory('Badge');
export const Card = __makeFactory('Card');
export const Collapsible = __makeFactory('Collapsible');
export const Progress = __makeFactory('Progress');
export const Image = __makeFactory('Image');
export const Avatar = __makeFactory('Avatar');
export const Empty = __makeFactory('Empty');
export const Skeleton = __makeFactory('Skeleton');
export const Tooltip = __makeFactory('Tooltip');
export const Table = __makeFactory('Table');
export const Divider = __makeFactory('Divider');
export const Spacer = __makeFactory('Spacer');
export const Icon = __makeFactory('Icon');
export const NativeSlot = __makeFactory('NativeSlot');

// HeroBanner 是基于底层原语（Stack / Box / Column / Row / Image / Text / Icon / NativeSlot）组合而成的 SDK 模板组件，
// 宿主 Go 渲染器不感知 HeroBanner，任何插件也可以直接使用底层原语自由定制自己的头图布局。
export function HeroBanner(props) {
  const p = props || {};
  const key = p.k != null ? String(p.k) : 'hero';
  const rawH = Number(p.height);
  const bannerH = Number.isFinite(rawH) && rawH >= 280 ? Math.min(760, rawH) : 560;
  const rawMaxW = Number(p.contentMaxWidth);
  const contentMaxW = Number.isFinite(rawMaxW) && rawMaxW > 0 ? rawMaxW : 1200;
  const rawCoverW = Number(p.coverWidth);
  const coverW = Number.isFinite(rawCoverW) && rawCoverW > 0 ? rawCoverW : 196;
  const rawToolbarW = Number(p.toolbarMaxWidth);
  const toolbarMaxW = Number.isFinite(rawToolbarW) && rawToolbarW > 0
    ? rawToolbarW
    : Math.max(360, contentMaxW - coverW - 40);

  let bannerSrc = p.banner ? String(p.banner).trim() : '';
  let fallbackBannerSrc = p.fallbackBanner ? String(p.fallbackBanner).trim() : '';
  if (!bannerSrc && fallbackBannerSrc) {
    bannerSrc = fallbackBannerSrc;
    fallbackBannerSrc = '';
  }
  const logoSrc = p.logo ? String(p.logo).trim() : '';
  const title = p.title ? String(p.title) : '';
  const subtitle = p.subtitle ? String(p.subtitle) : '';
  const blurVal = typeof p.blur === 'number' ? p.blur : (p.blur ? 20 : 0);
  const backSlot = p.backSlot ? String(p.backSlot) : '';
  const coverSlot = p.coverSlot ? String(p.coverSlot) : '';
  const toolbarSlot = p.toolbarSlot ? String(p.toolbarSlot) : '';
  const onPrev = typeof p.onPrev === 'function' ? p.onPrev : undefined;
  const onNext = typeof p.onNext === 'function' ? p.onNext : undefined;

  const [loadedLogo, setLoadedLogo] = useState('');
  const hasLogo = Boolean(logoSrc && loadedLogo === logoSrc);

  const pager = p.pager && typeof p.pager === 'object' ? p.pager : null;
  const pagerTotal = pager ? Math.max(0, Number(pager.total) || 0) : 0;
  const pagerCurrent = pager ? Math.max(1, Math.min(pagerTotal || 1, Number(pager.current) || 1)) : 1;
  const hasPager = Boolean((onPrev || onNext) && (pagerTotal === 0 || pagerTotal > 1));

  const metaItems = [];
  if (Array.isArray(p.meta)) {
    for (let i = 0; i < p.meta.length; i++) {
      const item = p.meta[i];
      if (item && typeof item === 'object') {
        const text = item.text != null ? String(item.text).trim() : '';
        if (text) metaItems.push({ icon: item.icon ? String(item.icon) : '', text });
      } else if (item != null) {
        const text = String(item).trim();
        if (text) metaItems.push({ icon: '', text });
      }
    }
  } else if (typeof p.meta === 'string' && p.meta.trim()) {
    metaItems.push({ icon: '', text: p.meta.trim() });
  }

  const rating = p.rating && typeof p.rating === 'object' ? p.rating : null;
  const ratingScore = rating ? Number(rating.score) || 0 : 0;
  const ratingTotal = rating ? Number(rating.total) || 0 : 0;
  const ratingPlayed = rating ? Number(rating.played) || 0 : 0;

  // safeTop：宿主隐藏顶栏时窗口顶部被拖动区 / 窗口按钮占用的高度，左上角返回胶囊整体下移避让
  const rawSafeTop = Number(p.safeTop);
  const safeTop = Number.isFinite(rawSafeTop) && rawSafeTop > 0 ? rawSafeTop : 0;
  const padTop = 20 + safeTop;
  const padX = 32;
  const padBottom = 28;
  const innerMinH = Math.max(0, bannerH - padTop - padBottom);

  const leftChildren = [];
  if (logoSrc) {
    leftChildren.push(
      jsx(Image, {
        k: key + '-logo',
        src: logoSrc,
        maxWidth: 460,
        maxHeight: 132,
        fit: 'contain',
        transparent: true,
        hideWhenEmpty: true,
        onLoad: () => setLoadedLogo(logoSrc),
        onError: () => { if (loadedLogo === logoSrc) setLoadedLogo(''); },
      })
    );
  }

  if (hasLogo) {
    const subLine = subtitle && subtitle !== title ? (title ? title + '  ·  ' + subtitle : subtitle) : title;
    if (subLine) {
      leftChildren.push(
        jsx(Text, { text: subLine, size: 'base', weight: 'medium', color: 'white@0.86', selectable: true })
      );
    }
  } else {
    if (title) {
      leftChildren.push(
        jsx(Text, { text: title, size: 34, lineHeight: 42, weight: 'bold', tone: 'light', selectable: true })
      );
    }
    if (subtitle && subtitle !== title) {
      leftChildren.push(
        jsx(Text, { text: subtitle, size: 'sm', weight: 'normal', color: 'white@0.80', selectable: true })
      );
    }
  }

  if (metaItems.length > 0) {
    leftChildren.push(
      jsx(Row, {
        wrap: true,
        gap: 16,
        align: 'center',
        children: metaItems.map((item, idx) =>
          jsx(Row, {
            k: key + '-meta-' + idx,
            gap: 5,
            align: 'center',
            children: [
              item.icon ? jsx(Icon, { name: item.icon, fallback: '', size: 14, color: 'white@0.72' }) : null,
              jsx(Text, { text: item.text, size: 'xs', weight: 'normal', color: 'white@0.90', wrap: false }),
            ],
          })
        ),
      })
    );
  }

  if (ratingScore > 0) {
    const stars = ratingScore / 2;
    const starNodes = [];
    for (let i = 1; i <= 5; i++) {
      let iconName = 'star-outline';
      let col = 'white@0.35';
      if (stars >= i - 0.25) {
        iconName = 'star';
        col = 'warning400';
      } else if (stars >= i - 0.75) {
        iconName = 'star-half-full';
        col = 'warning400';
      }
      starNodes.push(jsx(Icon, { k: key + '-star-' + i, name: iconName, fallback: 'star-outline', size: 16, color: col }));
    }
    const statParts = [];
    if (ratingTotal > 0) statParts.push(ratingTotal + ' 人评分');
    if (ratingPlayed > 0) statParts.push(ratingPlayed + ' 人玩过');

    leftChildren.push(
      jsx(Row, {
        k: key + '-rating',
        gap: 12,
        align: 'center',
        marginY: 2,
        children: [
          jsx(Text, { text: ratingScore.toFixed(1), size: 34, lineHeight: 38, weight: 'bold', tone: 'light', wrap: false }),
          jsx(Column, {
            gap: 2,
            children: [
              jsx(Row, { gap: 2, align: 'center', children: starNodes }),
              statParts.length > 0
                ? jsx(Text, { text: statParts.join('   '), size: 'xs', weight: 'normal', color: 'white@0.75', wrap: false })
                : null,
            ],
          }),
        ],
      })
    );
  }

  if (toolbarSlot) {
    leftChildren.push(
      jsx(Box, {
        k: key + '-toolbar-inline',
        width: 'full',
        marginTop: 6,
        children: jsx(NativeSlot, { name: toolbarSlot, maxWidth: toolbarMaxW }),
      })
    );
  }

  let rightCol = null;
  if (coverSlot || hasPager) {
    const rightChildren = [];
    if (coverSlot) {
      rightChildren.push(
        jsx(Box, {
          k: key + '-cover-frame',
          shrink: 0,
          radius: 10,
          clip: true,
          shadow: { x: 0, y: 14, blur: 32, spread: 0, color: 'black@0.45' },
          border: true,
          borderColor: 'white@0.18',
          children: jsx(NativeSlot, { name: coverSlot, width: coverW }),
        })
      );
    }
    if (hasPager) {
      rightChildren.push(
        jsx(Row, {
          k: key + '-pager',
          shrink: 0,
          align: 'center',
          gap: 2,
          padding: 3,
          radius: 'full',
          background: 'black@0.42',
          border: true,
          borderColor: 'white@0.16',
          children: [
            jsx(Box, {
              k: key + '-pager-prev',
              width: 28,
              height: 28,
              radius: 'full',
              align: 'center',
              justify: 'center',
              hoverBackground: 'white@0.18',
              label: '上一张头图',
              disabled: !onPrev,
              onPress: onPrev,
              children: jsx(Icon, { name: 'chevron-left', fallback: '', size: 18, color: onPrev ? 'white@0.92' : 'white@0.32' }),
            }),
            pagerTotal > 0
              ? jsx(Text, {
                  text: pagerCurrent + ' / ' + pagerTotal,
                  size: 'xs',
                  weight: 'medium',
                  color: 'white@0.84',
                  wrap: false,
                  marginX: 6,
                })
              : null,
            jsx(Box, {
              k: key + '-pager-next',
              width: 28,
              height: 28,
              radius: 'full',
              align: 'center',
              justify: 'center',
              hoverBackground: 'white@0.18',
              label: '下一张头图',
              disabled: !onNext,
              onPress: onNext,
              children: jsx(Icon, { name: 'chevron-right', fallback: '', size: 18, color: onNext ? 'white@0.92' : 'white@0.32' }),
            }),
          ],
        })
      );
    }
    rightCol = jsx(Column, {
      k: key + '-right-col',
      shrink: 0,
      align: 'end',
      gap: 12,
      children: rightChildren,
    });
  }

  const extraRow = p.children != null
    ? jsx(Row, {
        k: key + '-extra',
        width: 'full',
        wrap: true,
        gap: 10,
        align: 'center',
        marginTop: 4,
        children: p.children,
      })
    : null;

  return jsx(Stack, {
    k: key,
    width: 'full',
    minHeight: bannerH,
    clip: true,
    label: p.label,
    gradient: {
      angle: 135,
      stops: [
        { color: { light: 'brand700', dark: 'brand800' }, at: 0 },
        { color: 'fadeBg', at: 1 },
      ],
    },
    children: [
      bannerSrc
        ? jsx(Image, {
            k: key + '-bg',
            src: bannerSrc,
            fallbackSrc: fallbackBannerSrc,
            position: 'absolute',
            inset: 0,
            zIndex: -3,
            pointerEvents: 'none',
            fit: 'cover',
            transparent: true,
            blur: blurVal,
          })
        : null,
      jsx(Box, {
        k: key + '-scrim-h',
        position: 'absolute',
        inset: 0,
        zIndex: -2,
        pointerEvents: 'none',
        gradient: {
          angle: 90,
          stops: [
            { color: 'scrim@0.84', at: 0 },
            { color: 'scrim@0.56', at: 0.42 },
            { color: 'scrim@0.18', at: 0.76 },
            { color: 'scrim@0.06', at: 1 },
          ],
        },
      }),
      jsx(Box, {
        k: key + '-scrim-v',
        position: 'absolute',
        inset: 0,
        zIndex: -1,
        pointerEvents: 'none',
        gradient: {
          angle: 180,
          stops: [
            { color: 'black@0.24', at: 0 },
            { color: 'black@0.04', at: 0.24 },
            { color: 'black@0.12', at: 0.48 },
            { color: { light: 'scrim@0.56', dark: 'fadeBg@0.56' }, at: 0.72 },
            { color: { light: 'scrim@0.84', dark: 'fadeBg@0.92' }, at: 0.91 },
            { color: 'fadeBg', at: 1 },
          ],
        },
      }),
      jsx(Column, {
        k: key + '-fg',
        width: 'full',
        minHeight: bannerH,
        paddingTop: padTop,
        paddingRight: padX,
        paddingBottom: padBottom,
        paddingLeft: padX,
        children: jsx(Column, {
          k: key + '-inner',
          width: 'full',
          maxWidth: contentMaxW,
          centerX: true,
          minHeight: innerMinH,
          gap: 28,
          justify: backSlot ? 'space-between' : 'end',
          children: [
            backSlot
              ? jsx(Box, {
                  k: key + '-back-pill',
                  alignSelf: 'start',
                  paddingTop: 4,
                  paddingRight: 12,
                  paddingBottom: 4,
                  paddingLeft: 8,
                  radius: 'full',
                  background: { light: 'white@0.78', dark: 'black@0.36' },
                  border: true,
                  borderColor: { light: 'white@0.45', dark: 'white@0.14' },
                  children: jsx(NativeSlot, { name: backSlot }),
                })
              : null,
            jsx(Column, {
              width: 'full',
              gap: 16,
              children: [
                jsx(Row, {
                  width: 'full',
                  gap: 32,
                  align: 'end',
                  children: [
                    jsx(Column, {
                      grow: 1,
                      basis: 0,
                      minWidth: 0,
                      gap: 10,
                      children: leftChildren,
                    }),
                    rightCol,
                  ],
                }),
                extraRow,
              ],
            }),
          ],
        }),
      }),
    ],
  });
}

export function defineThemePreset(preset) {
  return preset;
}

export function createPagination(fetcher, options) {
  const pageSize = (options && options.pageSize) || 24;
  return function usePagination(queryKey) {
    const [items, setItems] = useState([]);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [error, setError] = useState(null);

    const load = async (targetPage, reset) => {
      setLoading(true);
      setError(null);
      try {
        const res = await fetcher({ page: targetPage, pageSize, query: queryKey });
        const list = Array.isArray(res) ? res : (res && Array.isArray(res.items) ? res.items : []);
        const more = Array.isArray(res) ? list.length >= pageSize : Boolean(res && res.hasMore);
        setItems((prev) => (reset ? list : prev.concat(list)));
        setPage(targetPage);
        setHasMore(more);
      } catch (e) {
        setError(e && e.message ? e.message : String(e));
      } finally {
        setLoading(false);
      }
    };

    useEffect(() => {
      load(1, true);
    }, [queryKey]);

    return {
      items,
      page,
      loading,
      hasMore,
      error,
      loadMore: () => { if (!loading && hasMore) return load(page + 1, false); },
      refresh: () => load(1, true),
    };
  };
}

export function createAIChatStore(ctx, options) {
  const opts = options || {};
  let messages = [];
  let streaming = false;
  let abortFn = null;
  const listeners = new Set();
  const notify = () => { for (const fn of listeners) fn(); };

  return {
    getState() { return { messages: messages.slice(), streaming }; },
    subscribe(fn) { listeners.add(fn); return () => listeners.delete(fn); },
    clear() {
      if (abortFn) { abortFn(); abortFn = null; }
      messages = [];
      streaming = false;
      notify();
    },
    abort() {
      if (abortFn) { abortFn(); abortFn = null; }
      streaming = false;
      notify();
    },
    send(content) {
      if (!content || streaming) return;
      messages = messages.concat([
        { role: 'user', content: String(content) },
        { role: 'assistant', content: '', toolCalls: [] },
      ]);
      streaming = true;
      notify();
      const req = {
        system: opts.system || '',
        messages: messages.slice(0, -1).map((m) => ({ role: m.role, content: m.content })),
        tools: opts.tools || [],
        model: opts.model,
        baseUrl: opts.baseUrl,
        apiKey: opts.apiKey,
      };
      abortFn = ctx.ai.stream(req, {
        onDelta(text) {
          const last = messages[messages.length - 1];
          if (last && last.role === 'assistant') {
            last.content += text;
            notify();
          }
        },
        async onToolCall(call) {
          const last = messages[messages.length - 1];
          const tc = Object.assign({ status: 'running' }, call);
          if (last && last.role === 'assistant') {
            last.toolCalls = (last.toolCalls || []).concat([tc]);
            notify();
          }
          let result = '';
          try {
            if (typeof opts.onToolCall === 'function') {
              const out = await opts.onToolCall(call);
              result = typeof out === 'string' ? out : JSON.stringify(out);
            }
            tc.status = 'done';
            tc.result = result;
          } catch (err) {
            tc.status = 'error';
            tc.result = err && err.message ? err.message : String(err);
            result = tc.result;
          }
          notify();
          return result;
        },
        onDone() {
          streaming = false;
          abortFn = null;
          notify();
        },
        onError(err) {
          streaming = false;
          abortFn = null;
          if (typeof opts.onError === 'function') opts.onError(err);
          notify();
        },
      });
    },
  };
}

// 供宿主 Go 运行时调用的内部桥接函数
globalThis.__lunabox_ui_mount = function(instanceId, renderFn, propsJson) {
  if (__instances.has(instanceId)) {
    globalThis.__lunabox_ui_unmount(instanceId);
  }
  const props = propsJson ? JSON.parse(propsJson) : {};
  const inst = {
    id: instanceId,
    renderFn,
    props,
    hooks: [],
    pendingEffects: [],
    handlers: new Map(),
    version: 0,
    scheduled: false,
    disposed: false,
  };
  __instances.set(instanceId, inst);
  __renderInstance(inst);
};

// props 变化时原地重渲染，保留 hook 状态；实例不存在（例如插件重载后）时按新实例挂载。
globalThis.__lunabox_ui_update = function(instanceId, renderFn, propsJson) {
  const inst = __instances.get(instanceId);
  if (!inst || inst.disposed) {
    globalThis.__lunabox_ui_mount(instanceId, renderFn, propsJson);
    return;
  }
  inst.props = propsJson ? JSON.parse(propsJson) : {};
  __renderInstance(inst);
};

globalThis.__lunabox_ui_unmount = function(instanceId) {
  const inst = __instances.get(instanceId);
  if (!inst) return;
  inst.disposed = true;
  __instances.delete(instanceId);
  for (let i = inst.hooks.length - 1; i >= 0; i--) {
    const h = inst.hooks[i];
    if (h && typeof h.cleanup === 'function') {
      try { h.cleanup(); } catch (e) { console.error(e); }
    }
  }
  inst.handlers.clear();
};

globalThis.__lunabox_ui_dispatch = function(instanceId, version, handlerId, payloadJson) {
  const inst = __instances.get(instanceId);
  if (!inst || inst.disposed) return;
  if (version && inst.version !== version) return;
  const fn = inst.handlers.get(handlerId);
  if (typeof fn !== 'function') return;
  const payload = payloadJson ? JSON.parse(payloadJson) : undefined;
  try {
    const res = payload !== undefined ? fn(payload) : fn();
    if (res && typeof res.catch === 'function') {
      res.catch((err) => console.error(err));
    }
  } catch (err) {
    console.error(err);
  }
};
`

var (
	sdkModuleOnce sync.Once
	sdkModule     *moejs.Module
	sdkModuleErr  error
)

func getLunaboxSDKModule() (*moejs.Module, error) {
	sdkModuleOnce.Do(func() {
		sdkModule, sdkModuleErr = moejs.Compile("lunabox", lunaboxRuntimeJS)
	})
	return sdkModule, sdkModuleErr
}
