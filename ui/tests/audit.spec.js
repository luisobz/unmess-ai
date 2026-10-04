import { test, expect } from "@playwright/test";

async function openCodex(page) {
  await page.goto("/#/agents");
  await page.locator(".agent-item").filter({ hasText: "Codex" }).click();
  await page
    .locator(".session-item")
    .filter({ hasText: "Corrige el login" })
    .click();
  await expect(page.locator("#session-title")).toContainText(
    "Corrige el login",
  );
}

test("families, sessions and files separate evidence from concurrent edits", async ({
  page,
}) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await openCodex(page);
  await expect(page.locator(".agent-item")).toHaveCount(2);
  await expect(page.locator("#session-files .audit-file")).toHaveCount(4);
  await expect(page.locator("#session-files")).not.toContainText("manual.md");
  await expect(page.locator("#session-files")).not.toContainText("noise.md");
  await page.locator("#session-candidates").check();
  await expect(page.locator("#session-files .audit-file")).toHaveCount(5);
  await expect(
    page.locator('#session-files [data-path="projects/demo/docs/manual.md"]'),
  ).toContainText("Sin atribución confirmada");
  await page.locator("#session-file-search").fill("login");
  await expect(page.locator("#session-files .audit-file")).toHaveCount(1);
  const file = page.locator(
    '#session-files [data-path="projects/demo/src/login.go"]',
  );
  await file.locator("summary").click();
  await expect(file).toContainText("apply_patch");
  await file.getByRole("button", { name: "Ver cambios", exact: true }).click();
  await expect(page.locator("#agentdiff-viewer")).toContainText("fixed login");
  await expect(page.locator("#agentdiff-viewer")).toContainText("old login");
  await page.keyboard.press("Escape");
  expect(errors).toEqual([]);
});

test("missing backups do not offer fictional recovery", async ({ page }) => {
  await openCodex(page);
  const file = page.locator(
    '#session-files [data-path="projects/demo/ignored.go"]',
  );
  await file.locator("summary").click();
  await expect(file).toContainText("no hay una copia guardada");
  await expect(file.getByRole("button")).toHaveCount(0);
});

test("session search and selection ignore delayed previous-agent responses", async ({
  page,
}) => {
  await openCodex(page);
  await page.locator("#session-search").fill("nonexistent");
  await expect(page.locator(".session-item")).toHaveCount(0);
  await page.locator("#session-search").fill("demo");
  await expect(page.locator(".session-item")).toHaveCount(1);
  await page.route("**/api/agent/session?*", async (route) => {
    if (route.request().url().includes("agent=codex"))
      await new Promise((r) => setTimeout(r, 500));
    await route.continue();
  });
  await page.locator(".session-item").click();
  await page.locator(".agent-item").filter({ hasText: "Claude Code" }).click();
  await page
    .locator(".session-item")
    .filter({ hasText: "Revisa el otro" })
    .click();
  await expect(page.locator("#session-title")).toContainText("Revisa el otro");
  await page.waitForTimeout(650);
  await expect(page.locator("#session-title")).toContainText("Revisa el otro");
  await expect(page.locator("#session-files-empty")).toContainText(
    "No hay escrituras confirmadas",
  );
  await expect(page.locator("#btn-session-revert")).toBeDisabled();
});

test("language, theme and laptop layout remain usable", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await openCodex(page);
  await page.locator("#btn-theme").click();
  await page.locator("#btn-settings").click();
  await page.locator("#cfg-lang").selectOption("en");
  await page.keyboard.press("Escape");
  await expect(page.locator("#session-view")).toContainText(
    "Files in this session",
  );
  await expect(page.locator("#session-view")).toContainText("Recorded write");
  await page.screenshot({
    path: "test-results/audit-laptop.png",
    fullPage: true,
  });
  const overflow = await page.evaluate(() =>
    [...document.querySelectorAll("body *")]
      .filter(
        (e) =>
          !e.closest("[hidden]") &&
          e.getBoundingClientRect().right > innerWidth + 1,
      )
      .map((e) => e.id || e.className)
      .slice(0, 15),
  );
  expect(overflow).toEqual([]);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    (await page.locator("#session-title").boundingBox()).width,
  ).toBeGreaterThan(250);
  await expect(page.locator("#theme-icon")).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.locator("#session-title").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: "test-results/audit-mobile.png",
    fullPage: true,
  });
});

test("plan, recovery and undo preserve later work", async ({ page }) => {
  await openCodex(page);
  await page.locator("#btn-session-revert").click();
  await expect(page.locator("#revert-plan")).toContainText("src/login.go");
  await expect(page.locator("#revert-plan")).toContainText("Trabajo posterior");
  await expect(page.locator("#revert-plan")).not.toContainText("manual.md");
  await page.locator("#revert-confirm").click();
  await expect(page.locator("#revert-undo")).toBeVisible();
  // Use the normal authenticated client, without exposing its token.
  const read = (path) =>
    page.evaluate(async (path) => {
      const { api } = await import("/src/core/api.js");
      return (
        await api(
          "/api/content?" + new URLSearchParams({ path, version: "current" }),
        )
      ).text();
    }, path);
  expect(await read("projects/demo/src/login.go")).toContain("old login");
  expect(await read("projects/demo/src/conflict.go")).toBe("later work");
  await page.locator("#revert-undo").click();
  await expect(page.locator("#revert-overlay")).toBeHidden();
  expect(await read("projects/demo/src/login.go")).toContain("fixed login");
  expect(await read("projects/demo/src/conflict.go")).toBe("later work");
});

test("modal traps keyboard focus and returns it on Escape", async ({
  page,
}) => {
  await page.goto("/");
  await page.locator("#btn-settings").click();
  await expect(page.locator("#settings-overlay")).toBeVisible();
  await page.locator('[data-close="settings"]').first().focus();
  await page.keyboard.press("Shift+Tab");
  await expect(page.locator("#settings-save")).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(page.locator('[data-close="settings"]').first()).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.locator("#btn-settings")).toBeFocused();
});

test("shared file links to the other agent session", async ({ page }) => {
  await openCodex(page);
  const file = page.locator(
    '#session-files [data-path="projects/demo/src/shared.go"]',
  );
  await file.locator("summary").click();
  await expect(file).toContainText("otra sesión simultánea");
  await file.getByRole("button", { name: /Claude Code · Otra sesión/ }).click();
  await expect(page.locator("#session-title")).toContainText(
    "Otra sesión sobre el archivo compartido",
  );
  await expect(page.locator("#sessions-agent-name")).toHaveText("Claude Code");
});
