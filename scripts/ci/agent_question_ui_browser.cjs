#!/usr/bin/env node
"use strict";
// Real authenticated Creator/Admin browsers against the disposable API/PG.
// Route interception injects transport faults only; writes are sent to the real
// backend before their acknowledgement is truncated. No model is simulated.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const modulePath = process.env.PLAYWRIGHT_PACKAGE_PATH;
assert.ok(modulePath && path.isAbsolute(modulePath), "PLAYWRIGHT_PACKAGE_PATH is required");
const { chromium } = require(modulePath);
const fixture = JSON.parse(fs.readFileSync(0, "utf8"));
const evidence = process.env.E2E_EVIDENCE_DIR || "/tmp/inklore-agent-ui-evidence";
fs.mkdirSync(evidence, { recursive: true });
let stage = "launch";
let pageForFailure;
const checks = [];
const verify = message => { checks.push(message); console.log("PASS browser " + message); };

async function login(browser, principal, width = 1280, locale = "zh-CN", session) {
  const origin = principal.audience === "admin" ? "http://127.0.0.1:3001" : "http://127.0.0.1:3000";
  const context = await browser.newContext({ locale, viewport: { width, height: 900 } });
  let token;
  if (session) {
    assert.ok(session.cookies.length > 0 && session.cookies.every(cookie => cookie.httpOnly), "Missing real server-issued HttpOnly session");
    await context.addCookies(session.cookies);
    token = session.access_token;
  } else {
    const response = await context.request.post(origin + "/v1/auth/login", { data: principal, headers: { Origin: origin } });
    const result = await response.json();
    assert.equal(response.status(), 200, `Fixture ${principal.audience} login failed: HTTP ${response.status()}, code ${result.code}`);
    assert.equal(result.code, "ok");
    token = result.data.access_token;
  }
  const page = await context.newPage();
  const scriptErrors = [];
  page.on("pageerror", error => scriptErrors.push(error.name));
  return { context, page, origin, token, scriptErrors };
}
function runURL(question) {
  return "http://127.0.0.1:3000/editor?" + new URLSearchParams({ novelId: fixture.novel_id, agentChapter: fixture.chapter_id, agentRun: question.run_id });
}
async function get(session, route) {
  const response = await session.context.request.get(session.origin + route, { headers: { Authorization: "Bearer " + session.token } });
  assert.equal(response.status(), 200, "Fixture authoritative read failed");
  return (await response.json()).data;
}
async function waitFor(condition, message) {
  for (let count = 0; count < 100; count++) {
    if (await condition()) return;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error(message);
}
async function widthFits(page, selector = '[role="dialog"]') {
  const fit = await page.locator(selector).evaluate(element => element.scrollWidth <= element.clientWidth + 1);
  assert.ok(fit, "Responsive panel overflowed horizontally");
}

(async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE } : {}), args: ["--no-sandbox"] });
  try {
    const creator = await login(browser, fixture.creator, 390);
    const admin = await login(browser, fixture.admin, 1280, "zh-CN", fixture.admin_session);
    const page = creator.page;
    pageForFailure = page;
    let creates = 0;
    const answers = [];
    page.on("request", request => {
      const url = new URL(request.url());
      if (request.method() === "POST" && url.pathname === "/v1/agents/runs") creates++;
      if (request.method() === "POST" && url.pathname.endsWith("/answer")) answers.push({ path: url.pathname, key: request.headers()["idempotency-key"] });
    });

    stage = "text-question-refresh-offline-ime";
    const text = fixture.questions[0];
    // An empty stream simulates EOF without a terminal event. State must remain
    // the persisted question, and the client must reconnect to the same Run.
    let streams = 0;
    const textEvents = `**/v1/agents/runs/${text.run_id}/events`;
    await page.route(textEvents, async route => {
      streams++;
      if (streams === 1) await route.fulfill({ status: 200, contentType: "text/event-stream", body: "" });
      else await route.continue();
    });
    await page.goto(runURL(text), { waitUntil: "domcontentloaded" });
    await page.getByTestId("agent-question").waitFor({ timeout: 25000 });
    await waitFor(() => streams >= 2, "EOF did not reconnect original Run");
    await widthFits(page);
    await page.screenshot({ path: path.join(evidence, "agent-question-390.png"), fullPage: true });
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.getByTestId("agent-question").waitFor({ timeout: 25000 });
    assert.equal(new URL(page.url()).searchParams.get("agentRun"), text.run_id);
    assert.equal(new URL(page.url()).searchParams.get("agentChapter"), fixture.chapter_id);
    await creator.context.setOffline(true);
    await page.getByText("网络离线；当前运行继续在服务端执行，重新连接后可恢复查看。", { exact: true }).waitFor();
    assert.ok(await page.getByRole("button", { name: "提交回答并继续", exact: true }).isDisabled());
    await creator.context.setOffline(false);
    await waitFor(async () => !(await page.getByRole("button", { name: "提交回答并继续", exact: true }).isDisabled()), "Question did not recover after offline");
    const textbox = page.getByRole("textbox", { name: "你的回答", exact: true });
    await textbox.dispatchEvent("compositionstart");
    await textbox.fill("中文输入用于作者澄清");
    await textbox.press("Enter");
    assert.equal(answers.length, 0, "IME Enter submitted an answer");
    await textbox.dispatchEvent("compositionend", { data: "中文输入用于作者澄清" });
    await page.waitForTimeout(80);
    await page.keyboard.press("Tab");
    assert.ok(await page.locator('[role="dialog"]').evaluate(element => element.contains(document.activeElement)), "Keyboard focus escaped dialog");
    await page.keyboard.press("Escape");
    await page.getByRole("dialog").waitFor({ state: "hidden" });
    await page.getByRole("button", { name: "打开任务工作台", exact: true }).click();
    assert.equal(await page.getByRole("textbox", { name: "你的回答", exact: true }).inputValue(), "中文输入用于作者澄清", "Closing the workbench discarded an unsent question draft");
    assert.equal(answers.length, 0, "Closing or reopening submitted a question draft");
    verify("390px, non-first chapter restore, refresh, offline, EOF and keyboard/IME retain original Run");
    verify("Closing to consult author text and reopening preserves the unsent answer draft");

    stage = "truncated-answer-acknowledgement";
    // Pause event delivery so the acknowledgement retry can be observed before
    // authoritative polling learns that the original answer has committed.
    await page.unroute(textEvents);
    await page.route(textEvents, route => route.abort("connectionreset"));
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.getByTestId("agent-question").waitFor({ timeout: 25000 });
    await page.getByRole("textbox", { name: "你的回答", exact: true }).fill("中文输入用于作者澄清");
    let truncate = true;
    await page.route(`**/questions/${text.question_id}/answer`, async route => {
      const response = await route.fetch();
      assert.equal(response.status(), 200, "Real answer did not commit");
      if (truncate) { truncate = false; await route.fulfill({ response, contentType: "application/json", body: "{" }); }
      else await route.fulfill({ response });
    });
    await page.getByRole("button", { name: "提交回答并继续", exact: true }).click();
    await page.getByRole("button", { name: "重试原请求", exact: true }).waitFor({ timeout: 10000 });
    await page.keyboard.press("Escape");
    await page.getByRole("dialog").waitFor({ state: "hidden" });
    await page.getByRole("button", { name: "打开任务工作台", exact: true }).click();
    await page.getByRole("button", { name: "重试原请求", exact: true }).waitFor();
    await page.getByRole("button", { name: "重试原请求", exact: true }).click();
    await waitFor(async () => (await get(creator, `/v1/agents/runs/${text.run_id}/questions/${text.question_id}`)).status === "answered", "Text answer was not persisted");
    const textKeys = answers.filter(item => item.path.includes(text.question_id)).map(item => item.key);
    assert.ok(textKeys.length >= 2 && new Set(textKeys).size === 1 && textKeys[0], "Truncated acknowledgement lost its original idempotency key");
    assert.equal(creates, 0, "Recovery created another Run");
    verify("Committed text answer with truncated JSON retains its CAS body/key across closing and reopening");

    stage = "single-choice-empty-acknowledgement";
    const single = fixture.questions[1];
    await page.route(`**/v1/agents/runs/${single.run_id}/events`, route => route.abort("connectionreset"));
    let empty = true;
    await page.route(`**/questions/${single.question_id}/answer`, async route => {
      const response = await route.fetch();
      assert.equal(response.status(), 200, "Real single-choice answer did not commit");
      if (empty) { empty = false; await route.fulfill({ response, contentType: "application/json", body: JSON.stringify({ code: "ok", data: null }) }); }
      else await route.fulfill({ response });
    });
    await page.goto(runURL(single), { waitUntil: "domcontentloaded" });
    await page.getByTestId("agent-question").waitFor({ timeout: 25000 });
    await page.getByRole("radio", { name: "third", exact: true }).check();
    await page.getByRole("button", { name: "提交回答并继续", exact: true }).click();
    await page.getByRole("button", { name: "重试原请求", exact: true }).waitFor({ timeout: 10000 });
    await page.getByRole("button", { name: "重试原请求", exact: true }).click();
    const singleKeys = answers.filter(item => item.path.includes(single.question_id)).map(item => item.key);
    assert.ok(singleKeys.length >= 2 && new Set(singleKeys).size === 1, "Empty answer result cleared the operation key");
    await waitFor(async () => (await get(creator, `/v1/agents/runs/${single.run_id}/questions/${single.question_id}`)).status === "answered", "Single-choice answer did not persist");
    verify("Single-choice form rejects an empty acknowledgement and retains stable retry");

    stage = "multiple-choice-expired-cursor";
    const multiple = fixture.questions[2];
    let cursorRequests = 0;
    await page.route(`**/v1/agents/runs/${multiple.run_id}/events`, async route => {
      cursorRequests++;
      const headers = { ...route.request().headers() };
      // This fixture really pruned seq 1 and retains seq 3, so the server emits
      // an actual 410 for the first reconnect header.
      if (cursorRequests === 1) headers["last-event-id"] = "1";
      await route.continue({ headers });
    });
    await page.goto(runURL(multiple), { waitUntil: "domcontentloaded" });
    await page.getByTestId("agent-question").waitFor({ timeout: 25000 });
    await waitFor(() => cursorRequests >= 2, "Expired cursor did not fetch/reconnect original Run");
    const english = await login(browser, fixture.creator, 1280, "en-US");
    await english.page.goto(runURL(multiple), { waitUntil: "domcontentloaded" });
    await english.page.getByRole("button", { name: "Submit answer and continue", exact: true }).waitFor({ timeout: 25000 });
    assert.equal(await english.page.getByRole("checkbox").count(), 3);
    await english.context.close();
    await page.getByRole("checkbox", { name: "mystery", exact: true }).check();
    await page.getByRole("checkbox", { name: "family", exact: true }).check();
    await page.getByRole("button", { name: "提交回答并继续", exact: true }).click();
    await waitFor(async () => (await get(creator, `/v1/agents/runs/${multiple.run_id}/questions/${multiple.question_id}`)).status === "answered", "Multiple-choice answer did not persist");
    assert.equal(creates, 0, "Expired cursor recovery created a Run");
    const chapter = await get(creator, `/v1/content/chapters/${fixture.chapter_id}/body`);
    assert.equal(chapter.body, fixture.source_body, "Question flow changed author text");
    verify("Multiple-choice form, actual 410 recovery and original author body");

    stage = "admin-immutable-policy-and-redacted-audit";
    pageForFailure = admin.page;
    await admin.page.goto(admin.origin + "/agents", { waitUntil: "domcontentloaded" });
    await admin.page.getByRole("button", { name: "任务策略", exact: true }).click();
    await admin.page.getByRole("article").filter({ has: admin.page.getByRole("heading", { name: /^UI Task Policy/ }) }).getByRole("button", { name: "编辑并发布新版本", exact: true }).click();
    await admin.page.getByLabel("最大输出 tokens", { exact: true }).fill("896");
    await admin.page.getByRole("button", { name: "发布不可变版本", exact: true }).click();
    await admin.page.getByText(/已发布版本 v3/).waitFor({ timeout: 15000 });
    const policy = await get(admin, "/v1/llm/task-policies/" + fixture.policy_id);
    assert.equal(policy.version, 3); assert.equal(policy.max_output_tokens, 896);
    await admin.page.getByRole("button", { name: "运行诊断", exact: true }).click();
    await admin.page.getByRole("button", { name: new RegExp(multiple.run_id) }).click();
    await admin.page.getByText("节点数:", { exact: true }).waitFor();
    await admin.page.getByText("未知：尚无可核验的计费回执", { exact: true }).waitFor();
    const content = await admin.page.locator("#root").innerText();
    assert.ok(!content.includes(fixture.source_body) && !content.includes("private fixture instruction") && !content.includes("中文输入用于作者澄清"), "Admin audit leaked owner data");
    assert.equal(await admin.page.getByRole("button", { name: "提交回答并继续" }).count(), 0, "Admin can answer owner question");
    await admin.page.setViewportSize({ width: 390, height: 900 });
    await waitFor(async () => (await admin.page.locator("aside").boundingBox()).width <= 65, "Admin sidebar did not collapse at mobile width");
    await widthFits(admin.page, "#root");
    await admin.page.screenshot({ path: path.join(evidence, "agent-audit-390.png"), fullPage: true });
    verify("Admin publishes immutable policy and sees only tenant-scoped diagnostic fields at 390px");

    stage = "authoritative-forbidden-clears-run";
    pageForFailure = page;
    // Use the real Admin principal on this Owner endpoint. The real server
    // returns 403; the UI must discard previously received sensitive Run data.
    await page.goto(runURL(multiple), { waitUntil: "domcontentloaded" });
    await page.locator('[role="dialog"]').waitFor();
    await page.route(`**/v1/agents/runs/${multiple.run_id}`, route => route.continue({ headers: { ...route.request().headers(), authorization: "Bearer " + admin.token } }));
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.getByText("访问权限已失效，已清除当前运行的敏感内容。", { exact: true }).waitFor({ timeout: 25000 });
    assert.equal(new URL(page.url()).searchParams.get("agentRun"), null);
    assert.equal(await page.getByTestId("agent-question").count(), 0);
    assert.equal(creates, 0, "Permission denial created a Run");
    verify("Actual 403 clears sensitive Run state and never starts a replacement");

    stage = "unreceived-create-reload-and-explicit-discard";
    const emptyURL = creator.origin + "/editor?" + new URLSearchParams({ novelId: fixture.novel_id, agentChapter: fixture.empty_chapter_id });
    await page.goto(emptyURL, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "打开任务工作台", exact: true }).click();
    await page.getByLabel("创作指令", { exact: true }).waitFor();
    const policyOption = page.getByRole("option", { name: /^UI Task Policy · v3$/ });
    await page.getByLabel("任务策略版本", { exact: true }).selectOption(await policyOption.getAttribute("value"));
    await page.getByLabel("任务类型", { exact: true }).selectOption("chapter_draft");
    await page.getByLabel("创作指令", { exact: true }).fill("Network fault before the real server receives this request");
    // Abort before forwarding: the real history must remain empty. No fake API
    // success is supplied and no replacement CreateRun is sent on restoration.
    await page.route("**/v1/agents/runs", route => route.request().method() === "POST" ? route.abort("connectionreset") : route.continue());
    await page.getByRole("button", { name: "创建运行", exact: true }).click();
    await page.getByRole("button", { name: "重试原创建请求", exact: true }).waitFor();
    assert.ok(new URL(page.url()).searchParams.get("agentCreate"), "Unconfirmed create lost its recovery pointer");
    assert.equal(creates, 1, "Unexpected initial create count");
    await page.reload({ waitUntil: "domcontentloaded" });
    const discard = page.getByRole("button", { name: "放弃未确认请求并返回填写", exact: true });
    await discard.waitFor();
    await waitFor(async () => !(await discard.isDisabled()), "Successful empty history did not permit explicit discard");
    const noRuns = await get(creator, "/v1/agents/runs");
    assert.equal(noRuns.items.filter(run => run.resource?.resource_id === fixture.empty_chapter_id).length, 0, "Aborted request reached the real server");
    await discard.click();
    await page.getByLabel("创作指令", { exact: true }).waitFor();
    assert.equal(new URL(page.url()).searchParams.get("agentCreate"), null);
    assert.equal(creates, 1, "Recovery or discard automatically submitted a replacement Run");
    verify("Unreceived CreateRun reload reads real empty history and permits explicit discard without resubmission");
    assert.equal(creator.scriptErrors.length + admin.scriptErrors.length, 0, "Browser script errors occurred");
    fs.writeFileSync(path.join(evidence, "agent-ui-browser.json"), JSON.stringify({ checks, live_model: "NOT_VERIFIED", question_source: "explicit persisted PG fixture" }, null, 2));
    await creator.context.close(); await admin.context.close();
  } catch (error) {
    if (pageForFailure) {
      try { await pageForFailure.screenshot({ path: path.join(evidence, "agent-ui-failure.png"), fullPage: true }); } catch {}
    }
    fs.writeFileSync(path.join(evidence, "agent-ui-failure.json"), JSON.stringify({ stage, error_type: error.name }, null, 2));
    throw error;
  } finally { await browser.close(); }
})().catch(error => {
  fs.writeFileSync(path.join(evidence, "agent-ui-failure.json"), JSON.stringify({ stage, error_type: error.name }, null, 2));
  console.error("Agent UI browser failed at " + stage + ": " + error.name + ": " + String(error.message).split("\n")[0].slice(0, 200));
  process.exitCode = 1;
});
