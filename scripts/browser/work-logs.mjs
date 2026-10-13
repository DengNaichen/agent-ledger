// Browser verification writes only to a disposable loopback InfTask fixture.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const [base, identifier, output] = process.argv.slice(2);
if (!base || !identifier || !output) throw new Error("Usage: node scripts/browser/work-logs.mjs <loopback-url> <test-identifier> <output-dir>");
const url = new URL(base);
if (url.protocol !== "http:" || url.hostname !== "127.0.0.1" || url.port === "8700") throw new Error("Only a disposable loopback test server is supported");
const response = await fetch(new URL(`/api/w/default/issues/${encodeURIComponent(identifier)}`, url), { signal: AbortSignal.timeout(5000) });
assert.equal(response.status, 200);
const fixture = await response.json();
assert(fixture.decisions.length > 0 && fixture.work_log_count > 20, "Prepare a decision and over 20 work logs before browsing");
fs.mkdirSync(output, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), "inftask-work-logs-chrome-"));
const browser = spawn(process.env.CHROME_PATH || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", [
  "--headless", `--user-data-dir=${profile}`, "--remote-debugging-port=0", "--no-first-run", "--no-default-browser-check", "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
const exited = new Promise((resolve) => { browser.on("exit", resolve); browser.on("error", resolve); });
function limit(promise, name) {
  let timer;
  return Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`Timed out waiting for ${name}`)), 30000); })]).finally(() => clearTimeout(timer));
}
let socket;
try {
  const endpoint = await limit(new Promise((resolve, reject) => {
    let log = "";
    browser.stderr.on("data", (chunk) => { log += chunk; const match = log.match(/DevTools listening on (ws:\/\/\S+)/); if (match) resolve(match[1]); });
    browser.on("error", reject);
    browser.on("exit", (code) => reject(new Error(`Chrome exited ${code}`)));
  }), "Chrome startup");
  socket = new WebSocket(endpoint);
  await limit(new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; }), "DevTools connection");
  let id = 0;
  const pending = new Map();
  socket.onmessage = ({ data }) => {
    const message = JSON.parse(data);
    if (!message.id) return;
    const call = pending.get(message.id); pending.delete(message.id);
    if (message.error) call.reject(new Error(message.error.message)); else call.resolve(message.result);
  };
  const send = (method, params = {}, sessionId) => limit(new Promise((resolve, reject) => {
    const next = ++id; pending.set(next, { resolve, reject }); socket.send(JSON.stringify({ id: next, method, params, sessionId }));
  }), method);
  const { targetId } = await send("Target.createTarget", { url: "about:blank" });
  const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
  const page = (method, params) => send(method, params, sessionId);
  await page("Page.enable");
  const evaluate = async (expression) => {
    const result = await page("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text);
    return result.result.value;
  };
  const wait = async (expression, name) => {
    const deadline = Date.now() + 10000;
    while (Date.now() < deadline) { if (await evaluate(expression)) return; await new Promise((resolve) => setTimeout(resolve, 100)); }
    throw new Error(`Timed out waiting for ${name}`);
  };
  const screenshot = async (name) => {
    const { data } = await page("Page.captureScreenshot", { format: "png" });
    fs.writeFileSync(path.join(output, name + ".png"), Buffer.from(data, "base64"));
  };
  await page("Network.enable");
  const click = (selector) => evaluate(`document.querySelector(${JSON.stringify(selector)}).click()`);
  const draft = "Browser test human feedback";
  const fillComment = () => evaluate(`(() => { const textarea = document.querySelector('textarea[aria-label="Comment"]'); Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set.call(textarea, ${JSON.stringify(draft)}); textarea.dispatchEvent(new Event('input', {bubbles:true})); })()`);
  for (const width of [1280, 1024, 760, 390]) {
    await page("Emulation.setDeviceMetricsOverride", { width, height: 900, deviceScaleFactor: 1, mobile: false });
    await page("Page.navigate", { url: new URL(`/default/issue/${encodeURIComponent(identifier)}`, url).href });
    await wait("!!document.querySelector('#tab-comments')", "detail");
    assert(await evaluate("document.querySelector('#tab-comments').getAttribute('aria-selected') === 'true'"));
    assert.equal(await evaluate("document.querySelectorAll('.work-log-list').length"), 0);
    await click(".decision-section .work-log-heading");
    await wait("!!document.querySelector('.decision-section .work-log-body .markdown')", "decision body");
    await evaluate("document.querySelector('.decision-section').scrollIntoView({block:'start'})");
    for (const theme of ["light", "dark"]) {
      await page("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: theme }] });
      await screenshot(`decisions-${width}-${theme}`);
    }
    await fillComment();
    await evaluate("document.querySelector('#tab-comments').focus()");
    await page("Input.dispatchKeyEvent", { type: "keyDown", key: "ArrowRight", code: "ArrowRight" });
    await page("Input.dispatchKeyEvent", { type: "keyUp", key: "ArrowRight", code: "ArrowRight" });
    await wait("document.querySelectorAll('.work-log-list .work-log').length === 20", "work log page");
    assert(await evaluate("document.activeElement.id === 'tab-work-logs'"), "Keyboard focus follows selected tab");
    await evaluate("document.querySelector('.issue-activity').scrollIntoView({block:'start'})");
    for (const theme of ["light", "dark"]) {
      await page("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: theme }] });
      await screenshot(`work-logs-${width}-${theme}`);
    }
    await click(".work-log-list > button");
    await wait(`document.querySelectorAll('.work-log-list .work-log').length === ${fixture.work_log_count}`, "older records");
    assert(!(await evaluate("document.documentElement.scrollWidth > innerWidth")), "Page overflow");
    assert(!(await evaluate("[...document.querySelectorAll('.work-log')].some(e => e.scrollWidth > e.clientWidth + 1)")), "Record overflow");
    await click("#tab-comments");
    assert.equal(await evaluate("document.querySelector('textarea[aria-label=Comment]').value"), draft);
  }
  // Network failure must preserve the comment draft and provide list retries.
  await page("Network.emulateNetworkConditions", { offline: true, latency: 0, downloadThroughput: 0, uploadThroughput: 0 });
  await click(".comment-form button");
  await wait("!!document.querySelector('[role=alert]')", "failed comment feedback");
  assert.equal(await evaluate("document.querySelector('textarea[aria-label=Comment]').value"), draft);
  await click("#tab-work-logs");
  await wait("!!document.querySelector('.work-log-error button')", "failed work log retry");
  await screenshot("work-logs-offline-390");
  await page("Network.emulateNetworkConditions", { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
  await click(".work-log-error button");
  await wait("document.querySelectorAll('.work-log-list .work-log').length === 20", "retried work log page");
  await click("#tab-comments");
  await click(".comment-form button");
  await wait("document.querySelector('textarea[aria-label=Comment]').value === ''", "comment saved");
  const saved = await (await fetch(new URL(`/api/w/default/issues/${encodeURIComponent(identifier)}`, url), { signal: AbortSignal.timeout(5000) })).json();
  assert(saved.comments.some((entry) => entry.body === draft), "Comment persists to the service");
  assert.equal(saved.work_log_count, fixture.work_log_count, "Human comment does not add a work log");
  console.log("PASS: real desktop/1024/760/390px, light/dark, default human comments, lazy decision body, keyboard tabs, pagination, offline retry, draft preservation and saved human comment");
} finally {
  socket?.close();
  browser.kill();
  await exited;
  fs.rmSync(profile, { recursive: true, force: true });
}
