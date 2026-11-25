// `lunabox-plugin init` / `lunabox-plugin skills`
//
// Templates and the agent skill live in a separate repository (DEFAULT_SOURCE):
//   templates/<name>/   one plugin project per template (plugin.json required)
//   skills/<skill>/     SKILL.md for `npx skills add` (https://github.com/vercel-labs/skills)
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { createInterface } from 'node:readline/promises';

export const DEFAULT_SOURCE = 'umbrae-lab/lunabox-plugins';
const DEFAULT_REF = 'main';
const TEMPLATES_DIR = 'templates';
const DEFAULT_TEMPLATE = 'basic';
// Keep in sync with pluginIDPattern in packages/backend/plugin/manifest.go
const ID_PATTERN = /^[a-z][a-z0-9-]{0,63}$/;
const SKIP_NAMES = new Set(['node_modules', '.git', '.DS_Store']);
const VALUE_OPTIONS = new Set(['template', 'id', 'name', 'source']);

function parseArgs(args) {
  const opts = { positional: [] };
  for (const a of args) {
    if (a === '-y' || a === '--yes') opts.yes = true;
    else if (a === '--force') opts.force = true;
    else if (a === '--skills') opts.skills = true;
    else if (a === '--no-skills') opts.skills = false;
    else if (a.startsWith('--') && a.includes('=')) {
      const i = a.indexOf('=');
      const key = a.slice(2, i);
      if (!VALUE_OPTIONS.has(key)) throw new Error(`Unknown option: --${key}`);
      opts[key] = a.slice(i + 1);
    } else if (a.startsWith('-')) {
      throw new Error(`Unknown option: ${a}`);
    } else {
      opts.positional.push(a);
    }
  }
  return opts;
}

// --source accepts a local directory (for template authors / offline use) or owner/repo[#ref].
export function resolveSource(source = DEFAULT_SOURCE) {
  const local = path.resolve(source);
  if (fs.existsSync(local) && fs.statSync(local).isDirectory()) {
    return { kind: 'local', root: local, label: local, skillSource: local };
  }
  const m = /^([\w.-]+)\/([\w.-]+)(?:#(.+))?$/.exec(source);
  if (!m) throw new Error(`Invalid --source "${source}": expected owner/repo[#ref] or an existing directory`);
  const [, owner, repo, ref = DEFAULT_REF] = m;
  return { kind: 'github', owner, repo, ref, label: `${owner}/${repo}#${ref}`, skillSource: `${owner}/${repo}` };
}

function githubHeaders(withToken) {
  const headers = { 'User-Agent': 'lunabox-plugin-cli' };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (withToken && token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function fetchTree(src) {
  const url = `https://api.github.com/repos/${src.owner}/${src.repo}/git/trees/${encodeURIComponent(src.ref)}?recursive=1`;
  const res = await fetch(url, { headers: { ...githubHeaders(true), Accept: 'application/vnd.github+json' } });
  if (res.status === 404) {
    throw new Error(`Template source ${src.label} not found (private repositories need GITHUB_TOKEN)`);
  }
  if (res.status === 403 || res.status === 429) {
    throw new Error(`GitHub API rate limit hit while reading ${src.label}; set GITHUB_TOKEN and retry`);
  }
  if (!res.ok) throw new Error(`Failed to read ${src.label}: HTTP ${res.status}`);
  const body = await res.json();
  if (body.truncated) console.warn(`Warning: file list of ${src.label} is truncated by GitHub`);
  return body.tree.filter((e) => e.type === 'blob').map((e) => e.path);
}

async function fetchRaw(src, filePath) {
  const encoded = filePath.split('/').map(encodeURIComponent).join('/');
  const url = `https://raw.githubusercontent.com/${src.owner}/${src.repo}/${encodeURIComponent(src.ref)}/${encoded}`;
  const res = await fetch(url, { headers: githubHeaders(true) });
  if (!res.ok) throw new Error(`Failed to download ${filePath}: HTTP ${res.status}`);
  return Buffer.from(await res.arrayBuffer());
}

function skipped(relPath) {
  return relPath.split('/').some((seg) => SKIP_NAMES.has(seg)) || relPath.endsWith('.zip');
}

// Returns Map<templateName, string[] of paths relative to the template directory>.
async function listTemplates(src) {
  const templates = new Map();
  const add = (name, rel) => {
    if (skipped(rel)) return;
    if (!templates.has(name)) templates.set(name, []);
    templates.get(name).push(rel);
  };
  if (src.kind === 'local') {
    const dir = path.join(src.root, TEMPLATES_DIR);
    if (!fs.existsSync(dir)) throw new Error(`No ${TEMPLATES_DIR}/ directory in ${src.label}`);
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const base = path.join(dir, entry.name);
      for (const rel of fs.readdirSync(base, { recursive: true })) {
        if (fs.statSync(path.join(base, rel)).isFile()) add(entry.name, rel.split(path.sep).join('/'));
      }
    }
  } else {
    const prefix = `${TEMPLATES_DIR}/`;
    for (const p of await fetchTree(src)) {
      if (!p.startsWith(prefix)) continue;
      const parts = p.slice(prefix.length).split('/');
      if (parts.length >= 2) add(parts[0], parts.slice(1).join('/'));
    }
  }
  for (const [name, files] of templates) {
    if (!files.includes('plugin.json')) templates.delete(name);
  }
  if (templates.size === 0) throw new Error(`No templates (${TEMPLATES_DIR}/<name>/plugin.json) found in ${src.label}`);
  return templates;
}

async function readTemplateFile(src, name, rel) {
  if (src.kind === 'local') return fs.readFileSync(path.join(src.root, TEMPLATES_DIR, name, ...rel.split('/')));
  return fetchRaw(src, `${TEMPLATES_DIR}/${name}/${rel}`);
}

function pickLocale(map) {
  if (!map || typeof map !== 'object') return typeof map === 'string' ? map : '';
  const locale = Intl.DateTimeFormat().resolvedOptions().locale;
  return map[locale] || map[locale.split('-')[0]] || map['en-US'] || Object.values(map)[0] || '';
}

async function describeTemplates(src, templates) {
  const out = [];
  for (const name of [...templates.keys()].sort()) {
    let desc = '';
    try {
      const manifest = JSON.parse((await readTemplateFile(src, name, 'plugin.json')).toString('utf8'));
      desc = pickLocale(manifest.description);
    } catch {
      // A broken manifest only loses the description here; copying reports the real error.
    }
    out.push({ name, desc });
  }
  return out;
}

function deriveId(dir) {
  const id = path
    .basename(dir)
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/^[^a-z]+/, '')
    .replace(/-+/g, '-')
    .replace(/-$/, '')
    .slice(0, 64);
  return id || 'my-plugin';
}

function packageManager() {
  const ua = process.env.npm_config_user_agent || '';
  for (const pm of ['pnpm', 'yarn', 'bun']) if (ua.startsWith(pm)) return pm;
  return 'npm';
}

function isEmptyDir(dir) {
  return !fs.existsSync(dir) || fs.readdirSync(dir).length === 0;
}

function writeJSON(file, value) {
  fs.writeFileSync(file, JSON.stringify(value, null, 2) + '\n');
}

function createPrompter(interactive) {
  let rl = interactive ? createInterface({ input: process.stdin, output: process.stdout }) : null;
  return {
    async ask(question, fallback) {
      if (!rl) return fallback;
      const answer = (await rl.question(`${question}${fallback ? ` (${fallback})` : ''}: `)).trim();
      return answer || fallback;
    },
    async confirm(question, fallback) {
      if (!rl) return fallback;
      const answer = (await rl.question(`${question} ${fallback ? '(Y/n)' : '(y/N)'}: `)).trim().toLowerCase();
      return answer ? answer === 'y' || answer === 'yes' : fallback;
    },
    close() {
      rl?.close();
      rl = null;
    },
  };
}

async function chooseTemplate(prompt, interactive, src, templates, requested) {
  if (requested) {
    if (!templates.has(requested)) {
      throw new Error(`Template "${requested}" not found in ${src.label}; available: ${[...templates.keys()].sort().join(', ')}`);
    }
    return requested;
  }
  if (templates.size === 1) return [...templates.keys()][0];
  if (!interactive) {
    if (templates.has(DEFAULT_TEMPLATE)) return DEFAULT_TEMPLATE;
    throw new Error(`Pass --template=<name>; available: ${[...templates.keys()].sort().join(', ')}`);
  }
  const list = await describeTemplates(src, templates);
  console.log('Templates:');
  list.forEach((t, i) => console.log(`  ${i + 1}. ${t.name}${t.desc ? ` - ${t.desc}` : ''}`));
  const defaultIndex = Math.max(0, list.findIndex((t) => t.name === DEFAULT_TEMPLATE)) + 1;
  for (;;) {
    const answer = await prompt.ask('Select a template', String(defaultIndex));
    const hit = list[Number(answer) - 1] || list.find((t) => t.name === answer);
    if (hit) return hit.name;
    console.log(`Enter a number between 1 and ${list.length}.`);
  }
}

async function copyTemplate(src, name, files, targetDir) {
  fs.mkdirSync(targetDir, { recursive: true });
  const queue = [...files];
  const worker = async () => {
    for (let rel = queue.shift(); rel; rel = queue.shift()) {
      const dest = path.join(targetDir, ...rel.split('/'));
      fs.mkdirSync(path.dirname(dest), { recursive: true });
      fs.writeFileSync(dest, await readTemplateFile(src, name, rel));
    }
  };
  await Promise.all(Array.from({ length: Math.min(8, files.length) }, worker));
}

function personalize(targetDir, id, displayName) {
  const manifestPath = path.join(targetDir, 'plugin.json');
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  manifest.id = id;
  const locales = manifest.name && typeof manifest.name === 'object' ? Object.keys(manifest.name) : [];
  manifest.name = Object.fromEntries((locales.length ? locales : ['zh-CN', 'en-US']).map((l) => [l, displayName]));
  writeJSON(manifestPath, manifest);

  const pkgPath = path.join(targetDir, 'package.json');
  if (!fs.existsSync(pkgPath)) return null;
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
  pkg.name = id;
  writeJSON(pkgPath, pkg);
  return pkg;
}

function quoteForCmd(arg) {
  return /[\s"&|<>^()%!]/.test(arg) ? `"${arg.replace(/"/g, '""')}"` : arg;
}

export function runSkillsAdd(dir, source, yes) {
  const args = ['-y', 'skills', 'add', source];
  if (yes) args.push('-y');
  console.log(`\n> npx ${args.join(' ')}`);
  // npx is a .cmd shim on Windows, which Node only spawns through a shell (given as one quoted command line).
  const res =
    process.platform === 'win32'
      ? spawnSync(['npx', ...args].map(quoteForCmd).join(' '), { cwd: dir, stdio: 'inherit', shell: true })
      : spawnSync('npx', args, { cwd: dir, stdio: 'inherit' });
  if (res.status === 0) return true;
  console.warn(`Skill installation failed; retry later with: npx skills add ${source}`);
  return false;
}

export async function initCommand(args) {
  const opts = parseArgs(args);
  const interactive = Boolean(process.stdin.isTTY) && !opts.yes;
  const prompt = createPrompter(interactive);
  try {
    const src = resolveSource(opts.source);
    const dirArg = opts.positional[0] || (await prompt.ask('Project directory', 'my-lunabox-plugin'));
    const targetDir = path.resolve(dirArg);
    if (!isEmptyDir(targetDir) && !opts.force) {
      throw new Error(`${targetDir} is not empty (use --force to write into it anyway)`);
    }

    console.log(`Reading templates from ${src.label} ...`);
    const templates = await listTemplates(src);
    const template = await chooseTemplate(prompt, interactive, src, templates, opts.template);

    let id = opts.id || (await prompt.ask('Plugin id', deriveId(targetDir)));
    while (!ID_PATTERN.test(id)) {
      if (!interactive) throw new Error(`Invalid plugin id "${id}": lowercase letter first, then lowercase letters, digits or "-" (max 64)`);
      console.log('Plugin id must start with a lowercase letter and contain only lowercase letters, digits or "-".');
      id = await prompt.ask('Plugin id', deriveId(targetDir));
    }
    const displayName = opts.name || (await prompt.ask('Display name', id));

    await copyTemplate(src, template, templates.get(template), targetDir);
    const pkg = personalize(targetDir, id, displayName);
    console.log(`Created ${id} from template "${template}" in ${targetDir}`);

    const wantSkills = opts.skills ?? (await prompt.confirm('Install the LunaBox plugin skill for AI coding agents?', true));
    // `skills add` asks its own questions on the same stdin.
    prompt.close();
    if (wantSkills) runSkillsAdd(targetDir, src.skillSource, opts.yes);

    const pm = packageManager();
    const run = pm === 'npm' ? 'npm run' : pm;
    const rel = path.relative(process.cwd(), targetDir) || '.';
    console.log('\nNext steps:');
    console.log(`  cd ${/\s/.test(rel) ? `"${rel}"` : rel}`);
    if (pkg) console.log(`  ${pm} install`);
    console.log(pkg?.scripts?.build ? `  ${run} build` : '  lunabox-plugin build');
    console.log('  Then add this folder in LunaBox: Plugins -> Add dev directory');
    if (!wantSkills) console.log(`  (AI skill: npx skills add ${src.skillSource})`);
  } finally {
    prompt.close();
  }
}

export async function skillsCommand(args) {
  const opts = parseArgs(args);
  const dir = path.resolve(opts.positional[0] || '.');
  if (!fs.existsSync(dir)) throw new Error(`${dir} does not exist`);
  const src = resolveSource(opts.source);
  if (!runSkillsAdd(dir, src.skillSource, opts.yes)) process.exitCode = 1;
}
