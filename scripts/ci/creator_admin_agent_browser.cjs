#!/usr/bin/env node
"use strict";
/*
 * Real Chromium acceptance for the two physically isolated Vite applications.
 * Runs only against disposable local API/PG/Redis, with generated CI accounts.
 * Browser automation is an acceptance CLI, not a permanent test fixture.
 */
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const modulePath = process.env.PLAYWRIGHT_PACKAGE_PATH;
if (!modulePath || !path.isAbsolute(modulePath)) {
  throw new Error("PLAYWRIGHT_PACKAGE_PATH must point to a temporary pinned Playwright package");
}
const { chromium } = require(modulePath);
const creds = JSON.parse(fs.readFileSync(process.env.E2E_FIXTURE_PATH, "utf8"));

async function authenticatedPage(browser, origin, principal) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  // The real Auth API issues HttpOnly cookies. The context request shares those
  // cookies with the browser; no localStorage injection or mocked frontend data.
  const response = await context.request.post(origin + "/v1/auth/login", {
    data: principal,
    headers: { Origin: origin, "Content-Type": "application/json" },
    timeout: 15000,
  });
  assert.equal(response.status(), 200, "HTTP login via " + origin + " failed");
  const body = await response.json();
  assert.equal(body.code, "ok");
  assert.equal(body.data.principal.audience, principal.audience);
  const page = await context.newPage();
  page.on("pageerror", err => {
    // Never dump JS errors containing API request details or credentials.
    console.error("Browser page script error type: " + err.name);
  });
  return { context, page };
}

async function verifyCreator(browser) {
  const origin = "http://127.0.0.1:3000";
  const { context, page } = await authenticatedPage(browser, origin, {
    email: creds.creator_email, password: creds.creator_password, audience: "creator",
  });
  try {
    await page.goto(origin + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
    await page.waitForFunction(
      () => document.querySelector("#root")?.textContent?.trim().length > 40,
      null, { timeout: 20000 },
    );
    await page.goto(origin + "/outline", { waitUntil: "domcontentloaded", timeout: 20000 });
    // React initially renders the session bootstrap state. URL alone does not
    // prove that the protected page has hydrated and loaded its real novel.
    let loaded = false;
    try {
      await page.waitForFunction(
        () => location.pathname === "/outline" &&
          (document.querySelector("#root")?.innerText?.trim().length ?? 0) > 40 &&
          !document.querySelector("#root")?.innerText?.includes("Loading..."),
        null, { timeout: 20000 },
      );
      loaded = true;
    } catch {
      const info = await page.evaluate(() => ({
        route: location.pathname,
        length: document.querySelector("#root")?.innerText?.trim().length ?? 0,
      }));
      console.error("Creator protected page diagnostics: " + JSON.stringify(info));
    }
    assert.ok(loaded, "Creator protected outline view did not become ready");
    assert.ok(!page.url().includes("/login"), "Creator browser lost its real session");
    console.log("Creator Chromium: protected page and HttpOnly-cookie session passed.");
  } finally {
    await context.close();
  }
}

async function verifyAdmin(browser) {
  const origin = "http://127.0.0.1:3001";
  const { context, page } = await authenticatedPage(browser, origin, {
    email: creds.admin_email, password: creds.admin_password,
    tenant_id: creds.tenant_id, audience: "admin",
  });
  try {
    await page.goto(origin + "/agents", { waitUntil: "domcontentloaded", timeout: 20000 });
    await page.waitForURL(url => url.pathname === "/agents", { timeout: 20000 });
    await page.waitForFunction(
      () => document.querySelector("#root")?.textContent?.trim().length > 40,
      null, { timeout: 20000 },
    );
    const content = await page.locator("#root").innerText({ timeout: 12000 });
    assert.ok(!page.url().includes("/login"), "Admin browser lost its real session");
    await page.getByText(creds.definition_title, { exact: false }).first()
      .waitFor({ state: "visible", timeout: 20000 });
    assert.ok(content.length > 40, "Admin governance view rendered blank");
    console.log("Admin/Agent Chromium: protected governance route and persisted definition visible.");
  } finally {
    await context.close();
  }
}

(async () => {
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  try {
    await verifyCreator(browser);
    await verifyAdmin(browser);
    console.log("Creator/Admin/Agent real-browser acceptance passed.");
  } finally {
    await browser.close();
  }
})().catch(err => {
  console.error("Browser acceptance failed: " + err.name + ": " +
    String(err.message).split("\n")[0].slice(0,240));
  process.exitCode = 1;
});
