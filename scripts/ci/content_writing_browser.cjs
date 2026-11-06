#!/usr/bin/env node
"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const modulePath = process.env.PLAYWRIGHT_PACKAGE_PATH;
assert.ok(modulePath && path.isAbsolute(modulePath), "Use the pinned CI Playwright installation");
const { chromium } = require(modulePath);
const fixture = JSON.parse(fs.readFileSync(0, "utf8"));
const evidence = process.env.E2E_EVIDENCE_DIR || "/tmp";
const origin = "http://127.0.0.1:3000";

(async () => {
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"],
    ...(process.env.E2E_CHROMIUM_PATH ? { executablePath: process.env.E2E_CHROMIUM_PATH } : {}) });
  let page;
  const pageErrors = [];
  let stage = "login";
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN" });
    const login = await context.request.post(origin + "/v1/auth/login", {
      data: { email: fixture.email, password: fixture.password, audience: "creator" }, headers: { Origin: origin },
    });
    assert.equal(login.status(), 200);
    const token = (await login.json()).data.access_token;
    page = await context.newPage();
    page.on("pageerror", error => pageErrors.push(String(error.message).slice(0, 160)));
    page.setDefaultTimeout(20_000);
    stage = "confirmed progress";
    await page.goto(origin + "/editor?novelId=" + fixture.novel_id);
    const progress = page.getByTestId("novel-writing-progress");
    await progress.getByText("完成 160% · 目标 5 字", { exact: true }).waitFor();
    assert.ok((await progress.innerText()).includes("累计已确认正文：8 字"));
    assert.ok((await page.getByTestId("today-writing-progress").innerText()).includes("今日新增：22 字"));
    await page.screenshot({ path: path.join(evidence, "writing-desktop.png") });

    stage = "saving goals and recovering incomplete responses";
    await page.getByRole("button", { name: "写作目标设置", exact: true }).click();
    await page.getByLabel("目标总字数（可留空）", { exact: true }).fill("4");
    await page.getByLabel("目标完结日期（可留空）", { exact: true }).fill("2029-12-31");
    const keys = [];
    await page.route("**/v1/content/novels/" + fixture.novel_id, async route => {
      if (route.request().method() !== "PUT") return route.continue();
      keys.push(route.request().headers()["idempotency-key"]);
      const response = await route.fetch(); // Every submission reaches the real API.
      assert.equal(response.status(), 200);
      if (keys.length === 1) await route.abort("failed");
      else if (keys.length === 2) await route.fulfill({ response, body: '{"code":"ok","data":' });
      else if (keys.length === 3) await route.fulfill({ response, json: { code: "ok", data: null } });
      else await route.fulfill({ response });
    });
    const form = page.getByTestId("writing-goals-form");
    const save = form.getByRole("button", { name: "保存", exact: true });
    for (let attempt = 0; attempt < 4; attempt++) {
      await save.click();
      if (attempt < 3) {
        await form.getByRole("alert").waitFor();
        await page.waitForFunction(() => document.querySelector('[data-testid="writing-goals-form"]')?.getAttribute("aria-busy") === "false");
        assert.equal(await page.getByLabel("目标总字数（可留空）", { exact: true }).inputValue(), "4");
      }
    }
    await progress.getByText("完成 200% · 目标 4 字", { exact: true }).waitFor();
    assert.equal(keys.length, 4);
    assert.ok(keys[0] && keys.every(key => key === keys[0]), "Lost or invalid responses must reuse the goal-update key");
    await page.reload();
    await progress.getByText("完成 200% · 目标 4 字", { exact: true }).waitFor();
    assert.ok((await progress.innerText()).includes("2029-12-31"));
    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForFunction(() => {
      const progress = document.querySelector('[data-testid="novel-writing-progress"]');
      return progress && progress.getBoundingClientRect().width >= 250;
    });
    assert.equal(await page.locator("aside").count(), 0, "Narrow editor starts with a collapsed chapter drawer");
    await page.screenshot({ path: path.join(evidence, "writing-mobile.png") });

    stage = "empty writing progress";
    const emptyResponse = await context.request.post(origin + "/v1/content/novels", {
      data: { title: "Empty writing progress", summary: "", category: "", target_words: 0 },
      headers: { Authorization: "Bearer " + token, Origin: origin, "Idempotency-Key": require("node:crypto").randomUUID() },
    });
    assert.equal(emptyResponse.status(), 201);
    const empty = (await emptyResponse.json()).data;
    await page.goto(origin + "/editor?novelId=" + empty.id);
    await page.getByText("尚无已确认正文。写好章节并确认后，这里会展示进度。", { exact: true }).waitFor();
    await page.screenshot({ path: path.join(evidence, "writing-empty.png") });
    await context.close();
    console.log("PASS writing browser: 160%/200% progress, real goal persistence, same-key network/JSON recovery, refresh and empty state.");
  } catch (error) {
    if (page) await page.screenshot({ path: path.join(evidence, "writing-failure.png") }).catch(() => {});
    throw new Error(stage + ": " + error.name + ": " + String(error.message).split("\n")[0] + (pageErrors.length ? "; page: " + pageErrors[0] : ""));
  } finally { await browser.close(); }
})().catch(error => {
  console.error("Writing browser acceptance failed: " + String(error.message).slice(0, 240));
  process.exitCode = 1;
});
