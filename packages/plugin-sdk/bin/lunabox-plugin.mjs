#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

const [, , cmd, ...rest] = process.argv;
const flags = new Set(rest.filter((a) => a.startsWith('--')));
const positional = rest.filter((a) => !a.startsWith('--'));
const targetArg = positional[0];
const cwd = targetArg ? path.resolve(targetArg) : process.cwd();

function flagValue(name, fallback) {
  const prefix = `--${name}=`;
  const hit = rest.find((a) => a.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : fallback;
}

async function buildPlugin() {
  const esbuild = await import('esbuild');
  const manifestPath = path.join(cwd, 'plugin.json');
  if (!fs.existsSync(manifestPath)) {
    throw new Error(`plugin.json not found in ${cwd}`);
  }
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const candidates = ['src/index.tsx', 'src/index.ts', 'src/main.tsx', 'src/main.ts', 'index.tsx', 'index.ts'];
  const entry = candidates.map((c) => path.join(cwd, c)).find((p) => fs.existsSync(p));
  if (!entry) {
    throw new Error(`No entry file found in ${cwd} (expected one of ${candidates.join(', ')})`);
  }
  const outFile = path.join(cwd, manifest.main || 'main.js');
  await esbuild.build({
    entryPoints: [entry],
    outfile: outFile,
    bundle: true,
    format: 'esm',
    // moejs provides Web APIs (fetch, streams, TextDecoder...), not Node built-ins:
    // "browser" lets packages such as the OpenAI / Anthropic SDKs pick their browser builds.
    platform: flagValue('platform', 'browser'),
    target: 'es2022',
    minify: flags.has('--minify'),
    jsx: 'automatic',
    jsxImportSource: 'lunabox',
    external: ['lunabox', 'lunabox/jsx-runtime', 'node:buffer'],
    sourcemap: false,
  });
  console.log(`Built ${manifest.id}@${manifest.version} -> ${outFile}`);
}

function packPlugin() {
  const manifestPath = path.join(cwd, 'plugin.json');
  if (!fs.existsSync(manifestPath)) {
    throw new Error(`plugin.json not found in ${cwd}`);
  }
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const zipName = `${manifest.id}-${manifest.version}.zip`;
  const outZip = path.join(cwd, zipName);
  if (fs.existsSync(outZip)) fs.unlinkSync(outZip);

  const candidates = ['plugin.json', manifest.main || 'main.js', 'locales', 'assets'];
  if (typeof manifest.icon === 'string' && manifest.icon.trim() && !manifest.icon.startsWith('assets/')) {
    candidates.push(manifest.icon.trim());
  }
  const include = [...new Set(candidates)].filter((p) =>
    fs.existsSync(path.join(cwd, p))
  );

  if (process.platform === 'win32') {
    const items = include.map((i) => `'${path.join(cwd, i).replace(/'/g, "''")}'`).join(',');
    execFileSync('powershell', [
      '-NoProfile',
      '-Command',
      `Compress-Archive -Path ${items} -DestinationPath '${outZip.replace(/'/g, "''")}' -Force`,
    ]);
  } else {
    execFileSync('zip', ['-r', outZip, ...include], { cwd });
  }
  console.log(`Packed ${outZip}`);
}

async function main() {
  if (cmd === 'build') {
    await buildPlugin();
  } else if (cmd === 'pack') {
    packPlugin();
  } else {
    console.log('Usage: lunabox-plugin build [dir] [--minify] [--platform=browser|neutral]');
    console.log('       lunabox-plugin pack [dir]');
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
