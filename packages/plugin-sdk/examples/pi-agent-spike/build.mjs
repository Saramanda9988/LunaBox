// Bundles src/index.js into a single ESM main.js for the LunaBox plugin runtime.
// Differs from `lunabox-plugin build` (platform 'neutral') in two ways pi-ai needs:
//  - platform 'browser': @anthropic-ai/sdk and openai map their Node-only internals
//    (node:child_process, node:crypto, node:os, ...) to browser shims through the
//    package.json "browser" field, which esbuild only honours on the browser platform.
//  - inject src/polyfills.js: FinalizationRegistry, FormData/Blob/File (missing in moejs).
// Env: PI_SPIKE_MINIFY=1, PI_SPIKE_NO_POLYFILLS=1, PI_SPIKE_PLATFORM=neutral, PI_SPIKE_META=1.
import * as esbuild from 'esbuild';
import fs from 'node:fs';

const noPolyfills = process.env.PI_SPIKE_NO_POLYFILLS === '1';

const result = await esbuild.build({
  entryPoints: ['src/index.js'],
  outfile: 'main.js',
  bundle: true,
  format: 'esm',
  platform: process.env.PI_SPIKE_PLATFORM || 'browser',
  target: 'es2022',
  external: ['lunabox', 'lunabox/jsx-runtime', 'node:buffer'],
  inject: noPolyfills ? [] : ['src/polyfills.js'],
  // moejs cannot chunk dynamic import(); everything must fold into main.js.
  splitting: false,
  metafile: true,
  logLevel: 'info',
  minify: process.env.PI_SPIKE_MINIFY === '1',
});
if (process.env.PI_SPIKE_META === '1') fs.writeFileSync('meta.json', JSON.stringify(result.metafile));
const size = fs.statSync('main.js').size;
console.log(`main.js: ${(size / 1024).toFixed(1)} KiB`);
