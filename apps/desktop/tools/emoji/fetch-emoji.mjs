// 下载 emoji-picker-element 默认使用的表情数据（emoji-picker-element-data@^1 的 en/emojibase/data.json），
// 压缩成 ui/assets/emoji.json 给 components.EmojiPicker 内嵌使用：原版在运行时从 CDN 下载，原型不联网。
//
//   node tools/emoji/fetch-emoji.mjs
import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const url = "https://cdn.jsdelivr.net/npm/emoji-picker-element-data@^1/en/emojibase/data.json";
const out = join(dirname(fileURLToPath(import.meta.url)), "../../ui/assets/emoji.json");

const res = await fetch(url);
if (!res.ok)
  throw new Error(`${url}: ${res.status}`);
const data = await res.json();
// 每项：[表情, 分组, 名称, 标签, 五种肤色的变体（没有时省略）]；与 emoji-picker-element 一样按 order 排序。
const rows = data
  .filter(e => e.group !== undefined && e.group !== 2) // 2 是 component（肤色、发型部件），picker 不显示
  .sort((a, b) => a.order - b.order)
  .map((e) => {
    const row = [e.emoji, e.group, e.annotation, e.tags ?? []];
    if (e.skins?.length)
      row.push(e.skins.sort((a, b) => a.tone - b.tone).map(s => s.emoji));
    return row;
  });
writeFileSync(out, `${JSON.stringify(rows)}\n`);
console.log(`${rows.length} emoji -> ${out}`);
