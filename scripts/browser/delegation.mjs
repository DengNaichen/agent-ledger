// Read-only browser verification against a disposable loopback InfTask.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const [base, identifier, output] = process.argv.slice(2);
if (!base || !identifier || !output) throw new Error("Usage: node scripts/browser/delegation.mjs <loopback-url> <test-identifier> <output-dir>");
const url = new URL(base);
if (url.protocol !== "http:" || url.hostname !== "127.0.0.1" || url.port === "8700") throw new Error("Only a disposable loopback test server is supported");
const response = await fetch(new URL(`/api/w/default/issues/${encodeURIComponent(identifier)}`, url), { signal: AbortSignal.timeout(5000) });
assert.equal(response.status, 200);
const fixture = await response.json();
assert(fixture.delegations.some((entry) => entry.agent_id), "Prepare main and child delegation records before browsing");
fs.mkdirSync(output, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), "inftask-delegation-chrome-"));
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
  for (const width of [1280, 390]) {
    await page("Emulation.setDeviceMetricsOverride", { width, height: 900, deviceScaleFactor: 1, mobile: false });
    await page("Page.navigate", { url: new URL(`/default/issue/${encodeURIComponent(identifier)}`, url).href });
    await wait(`document.querySelectorAll('.delegation-history li').length === ${fixture.delegations.length}`, "delegation history");
    assert(await evaluate("document.body.innerText.includes('Delegated by')"));
    assert(await evaluate("document.querySelector('.delegation-history').innerText.includes('Subagent')"));
    await screenshot(`delegation-detail-${width}`);
    await evaluate("document.querySelector('.delegation-link').click()");
    await wait("document.querySelector('.detail').scrollTop > 0 && document.querySelector('.delegation-history').getBoundingClientRect().top < 150", "delegation history scroll");
    await screenshot(`delegation-history-${width}`);
    const copied = await evaluate(`(() => {
      const element = document.querySelector('.delegation-history li:last-child code');
      const range = document.createRange(); range.selectNodeContents(element);
      const selection = getSelection(); selection.removeAllRanges(); selection.addRange(range);
      return selection.toString();
    })()`);
    assert.equal(copied, fixture.delegations.at(-1).session_id, "Session text must remain selectable in full");
    await evaluate("document.querySelector('.delegation-history li:last-child').scrollIntoView()");
    await screenshot(`delegation-long-id-${width}`);
    assert(!(await evaluate("document.documentElement.scrollWidth > innerWidth")), "Page overflow");
    assert(!(await evaluate("[...document.querySelectorAll('.delegation-history dd')].some(e => e.scrollWidth > e.clientWidth + 1)")), "Metadata overflow");
  }
  console.log("PASS: real detail, main/child history, Delegated by jump, selectable full session ID, desktop/390px without overflow");
} finally {
  socket?.close();
  browser.kill();
  await exited;
  fs.rmSync(profile, { recursive: true, force: true });
}
