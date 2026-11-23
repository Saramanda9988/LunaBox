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
export const HeroBanner = __makeFactory('HeroBanner');
export const NativeSlot = __makeFactory('NativeSlot');

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
