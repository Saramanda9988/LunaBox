// 用原版的 posterLayout.ts 生成 poster_layout.json，TestPosterLayoutMatchesFrontend 拿它对照 Go 的布局。
// 在本目录运行：node poster_layout.mjs（Node 22.18+ 直接运行 TypeScript）
import { writeFileSync } from "node:fs";
import { buildPosterLayout } from "../../../../../desktop/frontend/src/utils/posterLayout.ts";

function games(n, duration) {
  return Array.from({ length: n }, (_, i) => ({
    game_id: `game-${String(i).padStart(3, "0")}-${(i * 2654435761 % 4096).toString(16)}`,
    game_name: `Game ${i}`,
    company: "",
    cover_url: "",
    total_duration: duration(i),
  }));
}

const shapes = {
  distinct: i => 36000 - i * 997,
  tied: () => 7200,
  podiumTie: i => (i < 2 ? 50000 : 40000 - i * 300),
  longTail: i => Math.round(200000 / (i + 1) ** 1.3) + 60,
};
const cases = [];
for (const [name, fn] of Object.entries(shapes)) {
  for (const n of [1, 2, 3, 4, 5, 6, 7, 12, 40, ...(name === "longTail" ? [90] : [])]) {
    for (const seed of [1, 2]) {
      const input = games(n, fn);
      cases.push({ name: `${name}-${n}-${seed}`, seed, input, tiles: buildPosterLayout(input, seed).map(t => ({ id: t.game.game_id, x: t.x, y: t.y, width: t.width, height: t.height, lift: t.lift, rotation: t.rotation })) });
    }
  }
}
writeFileSync("poster_layout.json", JSON.stringify(cases));
console.log(`${cases.length} cases`);
