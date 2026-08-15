import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const localesDirectory = join(frontendRoot, "src", "locales");
const sourceDirectory = join(frontendRoot, "src");
const checkOnly = process.argv.includes("--check");

const localePaths = readdirSync(localesDirectory)
  .filter(name => name.endsWith(".json"))
  .map(name => join(localesDirectory, name));

const snapshots = new Map(
  localePaths.map(path => [
    path,
    {
      data: JSON.parse(readFileSync(path, "utf8")),
      raw: readFileSync(path, "utf8"),
    },
  ]),
);

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function flattenLeaves(value, prefix = "", result = new Map()) {
  if (!isObject(value)) {
    result.set(prefix, value);
    return result;
  }

  for (const [key, child] of Object.entries(value)) {
    flattenLeaves(child, prefix ? `${prefix}.${key}` : key, result);
  }

  return result;
}

function restoreOriginalOrder(current, original) {
  if (!isObject(current) || !isObject(original)) {
    return current;
  }

  const ordered = {};

  for (const key of Object.keys(original)) {
    if (Object.hasOwn(current, key)) {
      ordered[key] = restoreOriginalOrder(current[key], original[key]);
    }
  }

  for (const key of Object.keys(current)) {
    if (!Object.hasOwn(original, key)) {
      ordered[key] = current[key];
    }
  }

  return ordered;
}

function listSourceFiles(directory, result = []) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      listSourceFiles(path, result);
    }
    else if (/\.(?:js|jsx|ts|tsx)$/.test(entry.name)) {
      result.push(path);
    }
  }

  return result;
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function findLiteralReferences(keys) {
  const sources = listSourceFiles(sourceDirectory).map(path => ({
    path,
    text: readFileSync(path, "utf8"),
  }));
  const references = new Map();

  for (const key of keys) {
    const escapedKey = escapeRegExp(key);
    const pattern = new RegExp(
      `(?:"${escapedKey}"|'${escapedKey}'|${"`"}${escapedKey}${"`"})`,
    );
    const matches = sources
      .filter(source => pattern.test(source.text))
      .map(source => source.path.slice(frontendRoot.length + 1));

    if (matches.length > 0) {
      references.set(key, matches);
    }
  }

  return references;
}

function formatJSON(value, originalRaw) {
  const eol = originalRaw.includes("\r\n") ? "\r\n" : "\n";
  return `${JSON.stringify(value, null, 2).replaceAll("\n", eol)}${eol}`;
}

function restoreSnapshots() {
  for (const [path, snapshot] of snapshots) {
    writeFileSync(path, snapshot.raw, "utf8");
  }
}

const cliPath = join(
  frontendRoot,
  "node_modules",
  "i18next-cli",
  "dist",
  "esm",
  "cli.js",
);

if (!existsSync(cliPath)) {
  throw new Error("i18next-cli is missing; run pnpm install first.");
}

const extraction = spawnSync(process.execPath, [cliPath, "extract"], {
  cwd: frontendRoot,
  stdio: "inherit",
});

if (extraction.status !== 0) {
  restoreSnapshots();
  process.exit(extraction.status ?? 1);
}

const results = [];
const allRemovedKeys = new Set();
const missingTranslations = [];
const invalidTranslations = [];
const unexpectedEnglishTranslations = [];
const unsafeValueChanges = [];
const pluralReplacements = [];

for (const path of localePaths) {
  const snapshot = snapshots.get(path);
  const extracted = JSON.parse(readFileSync(path, "utf8"));
  const before = flattenLeaves(snapshot.data);
  const after = flattenLeaves(extracted);
  const removed = [...before.keys()].filter(key => !after.has(key));
  const added = [...after.keys()].filter(key => !before.has(key));

  for (const key of added) {
    missingTranslations.push(`${path}: ${key}`);
  }

  for (const [key, value] of after) {
    if (value === "" || value === key) {
      invalidTranslations.push(`${path}: ${key}`);
    }
    if (
      path.endsWith("en-US.json")
      && typeof value === "string"
      && /[\u3400-\u9FFF\u3040-\u30FF]/u.test(value)
    ) {
      unexpectedEnglishTranslations.push(`${path}: ${key}`);
    }
  }

  for (const key of removed) {
    allRemovedKeys.add(key);
    if (added.some(addedKey => addedKey.startsWith(`${key}_`))) {
      pluralReplacements.push(`${path}: ${key}`);
    }
  }

  for (const [key, previousValue] of before) {
    if (after.has(key) && after.get(key) !== previousValue) {
      unsafeValueChanges.push(`${path}: ${key}`);
    }
  }

  const ordered = restoreOriginalOrder(extracted, snapshot.data);
  const formatted = formatJSON(ordered, snapshot.raw);
  results.push({ added, after, formatted, path, removed });
}

const literalReferences = findLiteralReferences(allRemovedKeys);
const allResourceKeys = new Set(
  results.flatMap(result => [...result.after.keys()]),
);
const inconsistentResourceKeys = results.flatMap(result =>
  [...allResourceKeys]
    .filter(key => !result.after.has(key))
    .map(key => `${result.path}: ${key}`),
);

if (
  missingTranslations.length > 0
  || invalidTranslations.length > 0
  || unexpectedEnglishTranslations.length > 0
  || inconsistentResourceKeys.length > 0
  || unsafeValueChanges.length > 0
  || pluralReplacements.length > 0
  || literalReferences.size > 0
) {
  restoreSnapshots();
  console.error(
    "\nTranslation cleanup stopped because protected content would change.",
  );

  for (const item of missingTranslations) {
    console.error(`Missing translation: ${item}`);
  }
  for (const item of invalidTranslations) {
    console.error(`Empty or placeholder translation: ${item}`);
  }
  for (const item of unexpectedEnglishTranslations) {
    console.error(`Unexpected CJK text in English translation: ${item}`);
  }
  for (const item of inconsistentResourceKeys) {
    console.error(`Inconsistent locale key: ${item}`);
  }
  for (const item of unsafeValueChanges) {
    console.error(`Value change: ${item}`);
  }
  for (const item of pluralReplacements) {
    console.error(`Plural replacement: ${item}`);
  }
  for (const [key, paths] of literalReferences) {
    console.error(`Referenced key: ${key} (${paths.join(", ")})`);
  }

  process.exit(1);
}

const changed = results.filter(
  result => result.formatted !== snapshots.get(result.path).raw,
);

if (checkOnly) {
  restoreSnapshots();
  if (changed.length > 0) {
    console.error("\nTranslation resources need cleanup:");
    for (const result of changed) {
      console.error(
        `${result.path.slice(frontendRoot.length + 1)}: +${result.added.length} -${result.removed.length}`,
      );
    }
    process.exit(1);
  }

  console.log("\nTranslation resources are clean.");
  process.exit(0);
}

for (const result of results) {
  writeFileSync(result.path, result.formatted, "utf8");
  console.log(
    `${result.path.slice(frontendRoot.length + 1)}: +${result.added.length} -${result.removed.length}`,
  );
}
