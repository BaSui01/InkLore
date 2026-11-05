#!/usr/bin/env node
"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const modulePath = process.env.PLAYWRIGHT_PACKAGE_PATH;
assert.ok(modulePath && path.isAbsolute(modulePath), "Use the existing pinned CI Playwright installation");
const { chromium } = require(modulePath);
const fixture = JSON.parse(fs.readFileSync(0, "utf8"));
const evidence = process.env.E2E_EVIDENCE_DIR || "/tmp";
const origin = "http://127.0.0.1:3000";

(async () => {
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 960 }, acceptDownloads: true });
    const login = await context.request.post(origin + "/v1/auth/login", {
      data: { email: fixture.email, password: fixture.password, audience: "creator" },
      headers: { Origin: origin },
    });
    assert.equal(login.status(), 200);
    const token = (await login.json()).data.access_token;
    const page = await context.newPage();
    const keys = [];
    await page.route("**/v1/content/novels/*/exports", async route => {
      keys.push(route.request().headers()["idempotency-key"]);
      const response = await route.fetch(); // All writes reach the real API.
      if (keys.length === 1) await route.abort("failed"); // Lose only the successful response.
      else await route.fulfill({ response });
    });
    await page.goto(origin + "/editor?novelId=" + fixture.novel_id);
    await page.getByRole("button", { name: "导出作品", exact: true }).click();
    await page.getByLabel("文件格式").selectOption("epub");
    await page.getByRole("button", { name: "生成并下载", exact: true }).click();
    const downloading = page.waitForEvent("download");
    await page.getByRole("button", { name: "重试本次导出", exact: true }).click();
    const download = await downloading;
    assert.ok(download.suggestedFilename().endsWith(".epub"));
    await download.saveAs(path.join(evidence, "browser-export.epub"));
    assert.equal(keys.length, 2);
    assert.ok(keys[0] && keys[0] === keys[1], "Network recovery must reuse the same idempotency key");
    const jobs = await context.request.get(origin + "/v1/content/novels/" + fixture.novel_id + "/jobs", {
      headers: { Authorization: "Bearer " + token },
    });
    assert.equal((await jobs.json()).data.items.length, 5, "Lost response must not create a second export");
    await page.reload();
    await page.getByRole("button", { name: "导出作品", exact: true }).click();
    await page.getByRole("button", { name: "下载", exact: true }).first().waitFor();
    await page.screenshot({ path: path.join(evidence, "export-desktop.png") });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: path.join(evidence, "export-mobile.png") });
    const replayDownload = page.waitForEvent("download");
    await page.getByRole("button", { name: "下载", exact: true }).first().click();
    assert.ok((await replayDownload).suggestedFilename().endsWith(".epub"));
    assert.equal(keys.length, 2, "Downloading history must not submit a new export");
    await context.close();
  } finally { await browser.close(); }
})().catch(error => {
  console.error("Content browser acceptance failed: " + error.name + ": " + String(error.message).split("\n")[0].slice(0, 220));
  process.exitCode = 1;
});
