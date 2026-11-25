package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/MoeForge/moejs"
)

// webCompatPrelude 补齐 moejs 尚未提供、但常见 npm 库（如 pi 的 pi-ai / pi-agent-core、官方 OpenAI / Anthropic SDK）
// 在加载或请求时会用到的全局对象。只补“存在即可”的部分，不追求完整实现：
//   - FinalizationRegistry：空实现（不会回调；只用于缓存清理的库照常工作）。
//   - crypto.getRandomValues / crypto.randomUUID：由 Go crypto/rand 提供随机数。
//   - Blob / File / FormData：可构造、可读写字段，供库做 instanceof 判断与简单拼装；
//     fetch 暂不支持把 FormData / Blob 作为请求体发出 multipart 上传。
const webCompatPrelude = `(function (g) {
  var randomHex = g.__lunabox_random_hex;
  delete g.__lunabox_random_hex;

  if (typeof g.FinalizationRegistry === 'undefined') {
    g.FinalizationRegistry = class FinalizationRegistry {
      constructor(callback) {
        if (typeof callback !== 'function') throw new TypeError('FinalizationRegistry: callback must be a function');
      }
      register() {}
      unregister() { return false; }
    };
  }

  if (typeof g.crypto === 'undefined') {
    var getRandomValues = function (arr) {
      if (!ArrayBuffer.isView(arr) || arr instanceof Float32Array || arr instanceof Float64Array || arr instanceof DataView) {
        throw new TypeError('crypto.getRandomValues: expected an integer TypedArray');
      }
      if (arr.byteLength > 65536) {
        throw new DOMException('crypto.getRandomValues: at most 65536 bytes', 'QuotaExceededError');
      }
      var bytes = new Uint8Array(arr.buffer, arr.byteOffset, arr.byteLength);
      var hex = randomHex(bytes.length);
      for (var i = 0; i < bytes.length; i++) bytes[i] = parseInt(hex.substr(i * 2, 2), 16);
      return arr;
    };
    var randomUUID = function () {
      var b = getRandomValues(new Uint8Array(16));
      b[6] = (b[6] & 0x0f) | 0x40;
      b[8] = (b[8] & 0x3f) | 0x80;
      var h = '';
      for (var i = 0; i < 16; i++) h += (b[i] < 16 ? '0' : '') + b[i].toString(16);
      return h.slice(0, 8) + '-' + h.slice(8, 12) + '-' + h.slice(12, 16) + '-' + h.slice(16, 20) + '-' + h.slice(20);
    };
    g.crypto = { getRandomValues: getRandomValues, randomUUID: randomUUID };
  }

  if (typeof g.Blob === 'undefined') {
    var toBytes = function (part) {
      if (typeof part === 'string') return new TextEncoder().encode(part);
      if (part instanceof g.Blob) return part._bytes;
      if (part instanceof ArrayBuffer) return new Uint8Array(part);
      if (ArrayBuffer.isView(part)) return new Uint8Array(part.buffer, part.byteOffset, part.byteLength);
      return new TextEncoder().encode(String(part));
    };
    g.Blob = class Blob {
      constructor(parts, options) {
        var chunks = (parts || []).map(toBytes);
        var total = chunks.reduce(function (n, c) { return n + c.length; }, 0);
        var bytes = new Uint8Array(total);
        var off = 0;
        chunks.forEach(function (c) { bytes.set(c, off); off += c.length; });
        this._bytes = bytes;
        this.type = String((options && options.type) || '').toLowerCase();
      }
      get size() { return this._bytes.length; }
      async text() { return new TextDecoder().decode(this._bytes); }
      async arrayBuffer() { return this._bytes.slice().buffer; }
      async bytes() { return this._bytes.slice(); }
      slice(start, end, type) { return new g.Blob([this._bytes.slice(start, end)], { type: type || '' }); }
    };
  }
  if (typeof g.File === 'undefined') {
    g.File = class File extends g.Blob {
      constructor(parts, name, options) {
        super(parts, options);
        this.name = String(name);
        this.lastModified = (options && options.lastModified) || Date.now();
      }
    };
  }
  if (typeof g.FormData === 'undefined') {
    g.FormData = class FormData {
      constructor() { this._entries = []; }
      _value(value, filename) {
        if (value instanceof g.Blob) {
          if (filename !== undefined || !(value instanceof g.File)) return new g.File([value], filename === undefined ? 'blob' : filename, { type: value.type });
          return value;
        }
        return String(value);
      }
      append(name, value, filename) { this._entries.push([String(name), this._value(value, filename)]); }
      set(name, value, filename) { this.delete(name); this.append(name, value, filename); }
      delete(name) { name = String(name); this._entries = this._entries.filter(function (e) { return e[0] !== name; }); }
      get(name) { name = String(name); var e = this._entries.find(function (e) { return e[0] === name; }); return e ? e[1] : null; }
      getAll(name) { name = String(name); return this._entries.filter(function (e) { return e[0] === name; }).map(function (e) { return e[1]; }); }
      has(name) { name = String(name); return this._entries.some(function (e) { return e[0] === name; }); }
      forEach(fn, thisArg) { var self = this; this._entries.forEach(function (e) { fn.call(thisArg, e[1], e[0], self); }); }
      entries() { return this._entries.map(function (e) { return [e[0], e[1]]; })[Symbol.iterator](); }
      keys() { return this._entries.map(function (e) { return e[0]; })[Symbol.iterator](); }
      values() { return this._entries.map(function (e) { return e[1]; })[Symbol.iterator](); }
      [Symbol.iterator]() { return this.entries(); }
    };
  }
})(globalThis);`

var (
	webCompatOnce   sync.Once
	webCompatScript *moejs.Script
	webCompatErr    error
)

// installWebCompat 在插件代码运行前补齐全局对象。
func (p *Plugin) installWebCompat(rt *moejs.Runtime) error {
	webCompatOnce.Do(func() {
		webCompatScript, webCompatErr = moejs.CompileScript("lunabox:web-compat", webCompatPrelude)
	})
	if webCompatErr != nil {
		return fmt.Errorf("编译 web 兼容层失败: %w", webCompatErr)
	}
	randomHex := p.safeNative("__lunabox_random_hex", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		n := int(argFloat(args, 0))
		if n < 0 || n > 65536 {
			return moejs.Undefined(), throwJSError(r, "random byte count out of range")
		}
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			return moejs.Undefined(), throwJSError(r, err.Error())
		}
		return moejs.String(hex.EncodeToString(buf)), nil
	})
	if err := rt.SetGlobal("__lunabox_random_hex", randomHex); err != nil {
		return err
	}
	_, err := rt.RunScript(webCompatScript)
	return err
}
