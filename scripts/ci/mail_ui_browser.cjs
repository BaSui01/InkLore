#!/usr/bin/env node
"use strict";
// Real Creator/Admin pages and HTTP responses. No routes or delivery are mocked.
// Password authentication already occurred in Python; real server cookies are
// reused so another journey's operator login quota is never consumed here.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { randomUUID } = require("node:crypto");
const modulePath = process.env.PLAYWRIGHT_PACKAGE_PATH;
assert.ok(modulePath && path.isAbsolute(modulePath), "PLAYWRIGHT_PACKAGE_PATH is required");
const { chromium } = require(modulePath);
const fixture = JSON.parse(fs.readFileSync(0, "utf8"));
const evidence = process.env.E2E_EVIDENCE_DIR || "/tmp/inklore-mail-ui-evidence";
fs.mkdirSync(evidence, { recursive: true });
const settingsPath = "/v1/platform/mail-settings";
const fields = ["provider", "enabled", "smtp_host", "smtp_port", "smtp_security", "smtp_username", "sender_email", "sender_name"];
const checks = [];
const scriptErrors = [];
let stage = "launch";
let pageForFailure;
const verify = message => { checks.push(message); console.log("PASS browser " + message); };

async function context(browser, audience, supplied) {
  const origin = audience === "admin" ? "http://127.0.0.1:3001" : "http://127.0.0.1:3000";
  const instance = await browser.newContext({ locale: "zh-CN", viewport: { width: 1280, height: 900 } });
  if (supplied) {
    assert.ok(supplied.cookies.length && supplied.cookies.every(cookie => cookie.httpOnly), "Missing real HttpOnly cookie");
    await instance.addCookies(supplied.cookies);
  }
  const page = await instance.newPage();
  page.on("pageerror", error => scriptErrors.push(error.name));
  return { context: instance, page, origin, session: supplied?.session };
}
async function request(client, method, route, body, expected = 200) {
  const response = await client.context.request.fetch(client.origin + route, {
    method, ...(body === undefined ? {} : { data: body }),
    headers: { Authorization: "Bearer " + client.session.access_token, Origin: client.origin,
      "X-CSRF-Token": client.session.csrf_token, ...(method === "GET" ? {} : { "Idempotency-Key": randomUUID() }) },
  });
  assert.equal(response.status(), expected, `Real ${method} ${route} returned unexpected status`);
  const result = await response.json();
  if (expected >= 300) return result;
  assert.equal(result.code, "ok");
  return result.data;
}
function updateBody(settings, changes = {}) {
  return { ...Object.fromEntries(fields.map(field => [field, settings[field]])), expected_revision: settings.revision, ...changes };
}
async function saveResponse(page, expected) {
  const pending = page.waitForResponse(response => new URL(response.url()).pathname === settingsPath && response.request().method() === "PUT");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  const response = await pending;
  assert.equal(response.status(), expected, "Mail form returned an unexpected real API status");
  const headers = response.request().headers();
  assert.ok(headers["x-csrf-token"] && headers["idempotency-key"], "Mail form omitted CSRF or idempotency header");
  return response.json();
}
async function widthFits(page) {
  assert.ok(await page.locator("#root").evaluate(element => element.scrollWidth <= element.clientWidth + 1), "Mail/Auth page overflowed horizontally");
  if (await page.locator("form").count()) {
    assert.ok(await page.locator("form").evaluate(element => element.scrollWidth <= element.clientWidth + 1), "Mail/Auth form overflowed horizontally");
  }
}
async function waitForValue(locator, expected) {
  for (let count = 0; count < 100; count++) {
    if (await locator.inputValue() === expected) return;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error("Refreshed form did not use the current server value");
}
async function disabledLogin(browser, audience) {
  const client = await context(browser, audience);
  const page = client.page;
  pageForFailure = page;
  await page.setViewportSize({ width: 390, height: 900 });
  const pending = page.waitForResponse(response => new URL(response.url()).pathname === "/v1/auth/email-capabilities");
  await page.goto(client.origin + "/login", { waitUntil: "domcontentloaded" });
  const capability = await pending;
  assert.equal(capability.status(), 200);
  assert.equal((await capability.json()).data.enabled, false);
  await page.getByText("邮箱验证码服务尚未启用，请先使用密码登录", { exact: true }).waitFor();
  assert.ok(await page.getByRole("button", { name: "验证码登录", exact: true }).isDisabled());
  assert.ok(await page.getByRole("button", { name: "密码登录", exact: true }).isEnabled());
  assert.ok(await page.locator('input[type="password"]').isEnabled());
  assert.equal(await page.getByRole("button", { name: "忘记密码？", exact: true }).count(), 0);
  await widthFits(page);
  await page.screenshot({ path: path.join(evidence, `mail-auth-${audience}-390.png`), fullPage: true });
  await client.context.close();
}

(async () => {
  const browser = await chromium.launch({ headless: true,
    ...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE } : {}),
    args: ["--no-sandbox"] });
  try {
    stage = "disabled-auth-capability";
    await disabledLogin(browser, "creator");
    await disabledLogin(browser, "admin");
    verify("Creator/Admin use the real disabled capability, disable email login, retain password form and fit 390px");

    stage = "password-session-restoration";
    const creator = await context(browser, "creator", fixture.creator);
    const me = creator.page.waitForResponse(response => new URL(response.url()).pathname === "/v1/auth/refresh" && response.status() === 200);
    await creator.page.goto(creator.origin + "/login", { waitUntil: "domcontentloaded" });
    assert.equal((await (await me).json()).data.principal.user_id, fixture.creator.session.principal.user_id);
    await creator.page.waitForURL(creator.origin + "/");
    assert.equal((await request(creator, "GET", "/v1/auth/me")).user_id, fixture.creator.session.principal.user_id);
    await creator.context.close();
    verify("Real password-issued Creator cookie refreshes into the original authenticated identity with mail disabled");

    stage = "operator-permission";
    const operator = await context(browser, "admin", fixture.operator);
    pageForFailure = operator.page;
    await operator.page.goto(operator.origin + "/mail-settings", { waitUntil: "domcontentloaded" });
    await operator.page.getByRole("alert").filter({ hasText: "仅全局超级管理员可以管理邮件服务。" }).waitFor();
    assert.equal(await operator.page.getByLabel("邮件服务商").count(), 0);
    assert.equal((await request(operator, "GET", settingsPath, undefined, 403)).code, "auth.forbidden");
    await operator.page.screenshot({ path: path.join(evidence, "mail-operator-forbidden.png"), fullPage: true });
    await operator.context.close();
    verify("Tenant operator sees an explicit boundary and the server also denies global settings");

    stage = "super-admin-save";
    const admin = await context(browser, "admin", fixture.admin);
    const page = admin.page;
    pageForFailure = page;
    await page.goto(admin.origin + "/mail-settings", { waitUntil: "domcontentloaded" });
    await page.getByLabel("邮件服务商").waitFor();
    assert.ok(await page.getByRole("button", { name: "发送测试邮件", exact: true }).isDisabled());
    await page.getByLabel("邮件服务商").selectOption("resend");
    await page.getByText(/通过 Resend 的 HTTPS 接口投递/).waitFor();
    await page.getByLabel("发件人名称", { exact: true }).fill("Mail UI saved through form");
    const saved = (await saveResponse(page, 200)).data;
    assert.equal(saved.provider, "resend");
    assert.equal(saved.enabled, false);
    assert.equal(saved.sender_name, "Mail UI saved through form");
    assert.equal(saved.has_credential, false);
    assert.deepEqual(Object.keys(saved).sort(), [...fields, "has_credential", "credential_kind", "revision", "updated_at"].sort());
    assert.equal((await request(admin, "GET", settingsPath)).revision, saved.revision);
    await page.getByText("邮件配置已保存。请发送测试邮件确认实际投递。", { exact: true }).waitFor();
    assert.equal(await page.locator('input[type="password"]').inputValue(), "");
    verify("Super admin form saves the real Resend contract with CAS/CSRF/idempotency and exposes no credential");

    stage = "real-cas-conflict";
    await page.getByLabel("发件人名称", { exact: true }).fill("Unsaved author-controlled configuration");
    const concurrent = await request(admin, "PUT", settingsPath, updateBody(saved, { sender_name: "Concurrent server change" }));
    const conflict = await saveResponse(page, 409);
    assert.equal(conflict.code, "platform.version_conflict");
    await page.getByRole("status").filter({ hasText: "platform.version_conflict" }).waitFor();
    assert.equal(await page.getByLabel("发件人名称", { exact: true }).inputValue(), "Unsaved author-controlled configuration");
    assert.equal((await request(admin, "GET", settingsPath)).sender_name, concurrent.sender_name);
    await page.setViewportSize({ width: 390, height: 900 });
    await widthFits(page);
    await page.screenshot({ path: path.join(evidence, "mail-cas-conflict-390.png"), fullPage: true });
    verify("A real competing write produces visible 409 and preserves the unsaved form without overwriting the server");

    stage = "real-validation-error";
    await page.getByRole("status").filter({ hasText: "platform.version_conflict" }).getByRole("button", { name: "关闭", exact: true }).click();
    const refreshed = page.waitForResponse(response => new URL(response.url()).pathname === settingsPath && response.request().method() === "GET");
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    assert.equal((await refreshed).status(), 200);
    await waitForValue(page.getByLabel("发件人名称", { exact: true }), concurrent.sender_name);
    await page.getByLabel("邮件服务商").selectOption("smtp");
    await page.getByLabel("SMTP 主机", { exact: true }).fill(fixture.blocked_host);
    const invalid = await saveResponse(page, 400);
    assert.equal(invalid.code, "platform.validation_failed");
    await page.getByRole("status").filter({ hasText: "platform.validation_failed" }).waitFor();
    assert.equal(await page.getByLabel("SMTP 主机", { exact: true }).inputValue(), fixture.blocked_host);
    assert.equal((await request(admin, "GET", settingsPath)).revision, concurrent.revision);
    assert.ok(await page.getByRole("button", { name: "发送测试邮件", exact: true }).isDisabled());
    assert.equal(scriptErrors.length, 0, "Browser script errors occurred");
    await page.screenshot({ path: path.join(evidence, "mail-validation-error-390.png"), fullPage: true });
    verify("Unapproved host reaches real backend validation, displays its actual error and does not claim mail acceptance");
    fs.writeFileSync(path.join(evidence, "mail-ui-browser.json"), JSON.stringify({ checks, mail_enabled: false,
      otp_browser: "NOT_VERIFIED", external_delivery: "NOT_VERIFIED", fixtures: "HTTP accounts; explicit PG roles; real server cookies" }, null, 2));
    await admin.context.close();
  } catch (error) {
    if (pageForFailure && !pageForFailure.isClosed()) {
      try { await pageForFailure.screenshot({ path: path.join(evidence, "mail-ui-failure.png"), fullPage: true }); } catch {}
    }
    throw error;
  } finally { await browser.close(); }
})().catch(error => {
  fs.writeFileSync(path.join(evidence, "mail-ui-failure.json"), JSON.stringify({ stage, error_type: error.name }, null, 2));
  console.error("Mail UI browser failed at " + stage + ": " + error.name + ": " + String(error.message).split("\n")[0].slice(0, 160));
  process.exitCode = 1;
});
