// Polyfills injected by esbuild (`inject`) for gaps in the moejs plugin runtime.
// Exports replace *free* identifier references in the bundle only; code that probes
// `globalThis.X` still sees the real (missing) global and takes its own fallback path.

// moejs has WeakRef but no FinalizationRegistry (see moejs TODO.md, "Builtins").
// pi-schema constructs one at module top level for a regex cache; a no-op is safe
// (the cache entries just never get evicted).
class NoopFinalizationRegistry {
  constructor(_cleanup) {}
  register() {}
  unregister() { return false; }
}
export { NoopFinalizationRegistry as FinalizationRegistry };

// moejs webapi has no FormData / Blob / File. The OpenAI and Anthropic SDKs only
// construct them for file uploads, but test `body instanceof FormData` on every
// request; class stubs make those checks false and fail loudly if actually used.
function unsupported(name) {
  return class {
    constructor() {
      throw new TypeError(`${name} is not supported in the LunaBox plugin runtime`);
    }
    static [Symbol.hasInstance]() { return false; }
  };
}
const FormDataStub = globalThis.FormData ?? unsupported('FormData');
const BlobStub = globalThis.Blob ?? unsupported('Blob');
const FileStub = globalThis.File ?? unsupported('File');
export { FormDataStub as FormData, BlobStub as Blob, FileStub as File };
