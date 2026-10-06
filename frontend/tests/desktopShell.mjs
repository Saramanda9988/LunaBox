/* eslint-disable antfu/no-top-level-await -- Standalone browser regression runner. */
import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import process from "node:process";

// Start pnpm dev first; PLAYWRIGHT_MODULE may point to an existing Playwright installation.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright",
);
const browser = await chromium.launch({ channel: "chrome", headless: true });
const artifacts = resolve("../output/playwright");
await mkdir(artifacts, { recursive: true });
const failures = [];
let cases = 0;

async function assertLayout(page) {
  const layout = await page.evaluate(() => {
    const root = document.querySelector("[data-testid=root-layout]");
    const titlebar = document
      .querySelector("[data-testid=chrome]")
      .getBoundingClientRect();
    return [
      window.scrollX,
      window.scrollY,
      root.scrollLeft,
      root.scrollTop,
      titlebar.x,
      titlebar.y,
      titlebar.width - window.innerWidth,
    ];
  });
  assert.deepEqual(layout, [0, 0, 0, 0, 0, 0, 0]);
}

try {
  for (const width of [1280, 540]) {
    for (const fontSize of [16, 20, 24]) {
      for (const mode of ["light", "dark", "glass"]) {
        const page = await browser.newPage({
          viewport: { width, height: 800 },
        });
        page.setDefaultTimeout(8000);
        page.on("pageerror", error => failures.push(error.message));
        await page.goto(
          `${process.env.TEST_BASE_URL ?? "http://127.0.0.1:9245"}/tests/desktopShell.html${mode === "glass" ? "?glass=1" : ""}`,
        );
        const open = page.getByTestId("open");
        await open.waitFor();
        await page.evaluate(
          ({ fontSize, mode }) => {
            document.documentElement.style.fontSize = `${fontSize}px`;
            document.documentElement.classList.toggle("dark", mode === "dark");
          },
          { fontSize, mode },
        );
        const drawer = page.getByRole("dialog", {
          name: "Test drawer",
          exact: true,
        });
        await assertLayout(page);
        await open.click();
        await drawer
          .getByRole("heading", { name: "Test drawer", exact: true })
          .waitFor();
        await assertLayout(page);
        await page.waitForTimeout(100);
        await assertLayout(page);
        await page.waitForTimeout(350);
        await assertLayout(page);
        const bounds = await page
          .locator("[id^=headlessui-dialog-panel]")
          .first()
          .boundingBox();
        assert.equal(Math.round(bounds.y), 28);
        assert.equal(Math.round(bounds.x + bounds.width), width);
        await drawer.getByRole("button", { name: "One", exact: true }).click();
        await page.getByRole("option", { name: "Two", exact: true }).click();
        assert.equal(
          await drawer
            .getByRole("heading", { name: "Test drawer", exact: true })
            .isVisible(),
          true,
        );
        await drawer.getByRole("button", { name: "Two", exact: true }).click();
        await page.keyboard.press("Escape");
        assert.equal(await page.getByRole("listbox").count(), 0);
        assert.equal(
          await drawer
            .getByRole("heading", { name: "Test drawer", exact: true })
            .isVisible(),
          true,
        );
        await page.getByTestId("nested").click();
        const nested = page.getByRole("dialog", {
          name: "Nested drawer",
          exact: true,
        });
        await nested
          .getByRole("heading", { name: "Nested drawer", exact: true })
          .waitFor();
        await page.keyboard.press("Escape");
        await nested.waitFor({ state: "detached" });
        assert.equal(
          await drawer
            .getByRole("heading", { name: "Test drawer", exact: true })
            .isVisible(),
          true,
        );
        await drawer.getByRole("textbox").click();
        for (let index = 0; index < 8; index++) {
          await page.keyboard.press("Tab");
          // Headless UI forwards focus from its sentinels on the next animation frame.
          await page.waitForFunction(() =>
            document.activeElement?.closest("[role=\"dialog\"]"),
          );
          assert.equal(
            await drawer.evaluate(element =>
              element.contains(document.activeElement),
            ),
            true,
          );
        }
        await assertLayout(page);
        if (fontSize === 20) {
          await page.screenshot({
            path: resolve(artifacts, `${width}-${mode}-drawer.png`),
          });
        }
        await page.mouse.click(5, 100);
        await drawer.waitFor({ state: "detached" });
        assert.equal(
          await open.evaluate(element => element === document.activeElement),
          true,
        );
        await assertLayout(page);
        await open.click();
        await drawer
          .getByRole("heading", { name: "Test drawer", exact: true })
          .waitFor();
        await page.keyboard.press("Escape");
        await drawer.waitFor({ state: "detached" });
        await open.click();
        await drawer
          .getByRole("button", { name: "Close", exact: true })
          .click();
        await drawer.waitFor({ state: "detached" });
        await assertLayout(page);
        await page.getByTestId("open-modal").click();
        assert.equal(
          Math.round((await page.getByTestId("local-modal").boundingBox()).y),
          28,
        );
        await page.getByRole("button", { name: "Close modal" }).click();
        await assertLayout(page);
        await page.close();
        cases++;
      }
    }
  }
  assert.deepEqual(failures, []);
  process.stdout.write(
    `Drawer regression passed: ${cases} cases. Screenshots: ${artifacts}\n`,
  );
}
finally {
  await browser.close();
}
