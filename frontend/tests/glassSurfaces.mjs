/* eslint-disable antfu/no-top-level-await -- Standalone browser regression runner. */
import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import process from "node:process";

// Run against pnpm dev, using an existing Playwright installation if necessary.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright",
);
const browser = await chromium.launch({ channel: "chrome", headless: true });
const artifacts = resolve("../output/playwright/glass");
await mkdir(artifacts, { recursive: true });
const failures = [];
let comparisons = 0;

async function compareOutsideIsland(page, withIsland, withoutIsland, bounds) {
  return page.evaluate(
    async ({ withIsland, withoutIsland, bounds }) => {
      const decode = async (encoded) => {
        const image = new Image();
        image.src = `data:image/png;base64,${encoded}`;
        await image.decode();
        const canvas = document.createElement("canvas");
        canvas.width = image.width;
        canvas.height = image.height;
        const context = canvas.getContext("2d");
        context.drawImage(image, 0, 0);
        return context.getImageData(0, 0, image.width, image.height);
      };
      const [first, second] = await Promise.all([
        decode(withIsland),
        decode(withoutIsland),
      ]);
      let changed = 0;
      for (let y = 0; y < first.height; y++) {
        for (let x = 0; x < first.width; x++) {
          // Screenshots begin below the titlebar; allow the island's one-pixel ring.
          if (
            x >= bounds.x - 2
            && x <= bounds.x + bounds.width + 2
            && y + 28 >= bounds.y - 2
            && y + 28 <= bounds.y + bounds.height + 2
          ) {
            continue;
          }
          const index = (y * first.width + x) * 4;
          if (
            [0, 1, 2].some(
              channel =>
                Math.abs(
                  first.data[index + channel] - second.data[index + channel],
                ) > 2,
            )
          ) {
            changed++;
          }
        }
      }
      return changed;
    },
    {
      withIsland: withIsland.toString("base64"),
      withoutIsland: withoutIsland.toString("base64"),
      bounds,
    },
  );
}

try {
  for (const scenario of [
    { name: "light-90", query: "font=14.4", width: 1200 },
    { name: "light-100", query: "font=16", width: 1200 },
    { name: "dark-100", query: "font=16&dark=1", width: 1200 },
    { name: "light-narrow-125", query: "font=20", width: 600 },
    { name: "light-solid", query: "font=16&solid=1", width: 1200 },
    { name: "dark-solid", query: "font=16&dark=1&solid=1", width: 1200 },
    // This checks the CSS fallback, not the native WebKit rendering engine.
    { name: "webkit-light", query: "font=16&webkit=1", width: 1200 },
    { name: "webkit-dark", query: "font=16&webkit=1&dark=1", width: 1200 },
  ]) {
    for (const collapsed of [false, true]) {
      const captures = [];
      let bounds;
      let reference;
      for (const visible of [true, false]) {
        const page = await browser.newPage({
          viewport: { width: scenario.width, height: 800 },
        });
        page.setDefaultTimeout(8000);
        page.on("pageerror", error => failures.push(error.message));
        await page.goto(
          `${process.env.TEST_BASE_URL ?? "http://127.0.0.1:9245"}/tests/glassSurfaces.html?${scenario.query}${visible ? "" : "&no-island=1"}`,
        );
        await page
          .getByRole("heading", { name: "设置", exact: true })
          .waitFor();
        await page.waitForTimeout(450);
        const filter = await page
          .getByTestId("content")
          .evaluate(element => getComputedStyle(element).backdropFilter);
        const glassEnabled = !scenario.query.includes("solid");
        const nativeWebkit = scenario.query.includes("webkit");
        const darkMode = scenario.query.includes("dark");
        assert.equal(
          filter,
          "none",
          "The main viewport must preserve the configured wallpaper blur",
        );
        if (visible) {
          const surface = page
            .getByTestId("island-host")
            .locator("> div > div")
            .first();
          if (collapsed) {
            const initial = await surface.boundingBox();
            await page.mouse.move(
              initial.x + initial.width / 2,
              initial.y + initial.height / 2,
            );
            await page.mouse.down();
            await page.mouse.move(
              initial.x + initial.width / 2,
              initial.y - 60,
              { steps: 8 },
            );
            await page.mouse.up();
            await page.waitForTimeout(500);
            await page
              .getByRole("button", { name: "展开正在游玩的游戏", exact: true })
              .waitFor();
          }
          bounds = await surface.boundingBox();
        }
        const samples = [];
        // Place cards, a select, the section boundary and a filtered table header
        // behind the island. Fresh pages avoid compositor cache false negatives.
        for (const target of [
          page.getByTestId("account-card").first(),
          page.getByRole("button", { name: "跟随系统", exact: true }).first(),
          page.getByRole("heading", { name: "元数据设置", exact: true }),
          page.locator("thead"),
          page.getByTestId("game-card").getByRole("heading"),
        ]) {
          const scrollTop = await target.evaluate((element) => {
            const content = document.querySelector("[data-testid=content]");
            return content.scrollTop + element.getBoundingClientRect().top - 32;
          });
          await page.getByTestId("content").evaluate((element, top) => {
            element.scrollTop = top;
          }, scrollTop);
          await page.waitForTimeout(100);
          samples.push(
            await page.screenshot({
              clip: { x: 0, y: 28, width: scenario.width, height: 140 },
            }),
          );
        }
        captures.push(samples);
        if (visible) {
          const caption = page
            .getByTestId("game-card")
            .getByRole("heading")
            .locator("..");
          const captionFilter = await caption.evaluate(
            element => getComputedStyle(element).backdropFilter,
          );
          assert.equal(
            captionFilter,
            glassEnabled && !nativeWebkit
              ? `blur(${darkMode ? 12 : 24}px) saturate(${darkMode ? 1.8 : 1.25})`
              : "none",
          );
          if (!collapsed) {
            await page
              .getByTestId("game-card")
              .screenshot({
                path: resolve(artifacts, `${scenario.name}-game-card.png`),
              });
          }
          await page.getByTestId("content").evaluate((element) => {
            element.scrollTop = 0;
          });
          if (!collapsed) {
            await page.screenshot({
              path: resolve(artifacts, `${scenario.name}.png`),
            });
          }
          const select = page
            .getByRole("button", { name: "跟随系统", exact: true })
            .first();
          await select.click();
          const option = page.getByRole("option", {
            name: "自定义",
            exact: true,
          });
          const optionBounds = await option.boundingBox();
          assert.equal(
            await option.evaluate((element) => {
              const rect = element.getBoundingClientRect();
              return element.contains(
                document.elementFromPoint(
                  rect.x + rect.width / 2,
                  rect.y + rect.height / 2,
                ),
              );
            }),
            true,
            "Portaled options must remain above the clipped viewport",
          );
          assert.ok(optionBounds.width > 0);
          await option.click();
          await page.close();
        }
        else {
          reference = page;
        }
      }
      for (let index = 0; index < captures[0].length; index++) {
        const changed = await compareOutsideIsland(
          reference,
          captures[0][index],
          captures[1][index],
          bounds,
        );
        assert.equal(
          changed,
          0,
          `${scenario.name}, collapsed=${collapsed}, surface=${index}: glow outside the island`,
        );
        comparisons++;
      }
      await reference.close();
    }
    process.stdout.write(`Passed ${scenario.name}\n`);
  }
  assert.deepEqual(failures, []);
  process.stdout.write(
    `Glass surface regression passed: ${comparisons} image comparisons.\n`,
  );
}
finally {
  await browser.close();
}
