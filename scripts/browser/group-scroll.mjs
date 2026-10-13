// Seeds only an empty disposable server; reruns require the exact fixture.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const [base, output] = process.argv.slice(2);
if (!base || !output) throw new Error("Usage: node scripts/browser/group-scroll.mjs <loopback-url> <output-dir>");
const url = new URL(base);
if (url.protocol !== "http:" || url.hostname !== "127.0.0.1" || url.port === "8700") throw new Error("Only a disposable loopback test server is supported");
const request = async (route, body) => {
  const response = await fetch(new URL(route, url), {
    method: body ? "POST" : "GET", headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined, signal: AbortSignal.timeout(5000),
  });
  assert(response.ok, `${route}: HTTP ${response.status}`);
  return response.json();
};
const counts = { triage: 40, backlog: 1, todo: 30, agent_ready: 1, in_progress: 80, in_review: 1, done: 1000, canceled: 120 };
const workspaceName = "Group scroll browser fixture";
const { workspaces } = await request("/api/workspaces");
if (workspaces.length === 0) {
  await request("/api/workspaces", { slug: "default", name: workspaceName });
  await request("/api/w/default/teams", { key: "AL", name: "Scroll fixture" });
  for (const [status, count] of Object.entries(counts)) {
    for (let i = 0; i < count; i++) await request("/api/w/default/issues", { team: "AL", title: `Scroll fixture ${status} ${i}`, status, type: "bug" });
  }
} else {
  assert(workspaces.length === 1 && workspaces[0].slug === "default" && workspaces[0].name === workspaceName, "Refusing to seed an existing non-fixture workspace");
}
const { issues } = await request("/api/w/default/issues?limit=-1");
assert.equal(issues.length, Object.values(counts).reduce((sum, count) => sum + count, 0), "Fixture count");
for (const [status, count] of Object.entries(counts)) assert.equal(issues.filter((issue) => issue.status === status).length, count, status);
fs.mkdirSync(output, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), "inftask-group-scroll-chrome-"));
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
  const settle = () => evaluate("new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(resolve))))");
  const selector = (status) => `.group-header[aria-controls="rows-${status}"]`;
  const state = (status) => evaluate(`(() => {
    const list = document.querySelector('.list-view');
    const button = document.querySelector(${JSON.stringify(selector(status))});
    const rect = button.getBoundingClientRect();
    const group = button.closest('.list-group');
    const wrap = button.parentElement;
    return { top: rect.top, left: rect.left, width: rect.width, height: rect.height,
      expanded: button.getAttribute('aria-expanded'), scroll: list.scrollTop,
      max: list.scrollHeight - list.clientHeight, listTop: list.getBoundingClientRect().top,
      listBottom: list.getBoundingClientRect().bottom,
      groupStart: group.getBoundingClientRect().top - list.getBoundingClientRect().top + list.scrollTop,
      natural: group.getBoundingClientRect().top - list.getBoundingClientRect().top + list.scrollTop + parseFloat(getComputedStyle(wrap).paddingTop) };
  })()`);
  const position = async (status, offset) => {
    const viewport = await state(status);
    await page("Input.dispatchMouseEvent", { type: "mouseWheel", x: viewport.left + 40, y: viewport.listTop + 80, deltaX: 0, deltaY: 1 });
    await settle();
    // Virtual row estimates settle as a group first enters the viewport.
    // Scroll again as a user would until the requested position is reachable.
    for (let attempt = 0; attempt < 10; attempt++) {
      await evaluate(`(() => {
        const list = document.querySelector('.list-view');
        const group = document.querySelector(${JSON.stringify(selector(status))}).closest('.list-group');
        list.scrollTop += group.getBoundingClientRect().top - list.getBoundingClientRect().top + ${offset};
      })()`);
      await settle();
      const current = await state(status);
      const target = Math.max(0, Math.min(current.max, current.groupStart + offset));
      if (Math.abs(current.scroll - target) <= 1) return;
    }
    throw new Error(`Could not position ${status}`);
  };
  const toggle = async (status, input, name) => {
    const before = await state(status);
    assert(before.top >= before.listTop && before.top + before.height <= before.listBottom, `${name}: header must start visible: ${JSON.stringify(before)}`);
    // Observe the first painted layout, not just the eventual settled position.
    await evaluate(`document.querySelector(${JSON.stringify(selector(status))}).addEventListener('click', () => {
      window.scrollFrame = new Promise(resolve => requestAnimationFrame(() => resolve(document.querySelector(${JSON.stringify(selector(status))}).getBoundingClientRect().top)));
    }, { once: true })`);
    if (input === "mouse") {
      const coordinates = { x: before.left + before.width / 2, y: before.top + before.height / 2, button: "left", clickCount: 1 };
      await page("Input.dispatchMouseEvent", { type: "mousePressed", ...coordinates });
      await page("Input.dispatchMouseEvent", { type: "mouseReleased", ...coordinates });
    } else {
      await evaluate(`document.querySelector(${JSON.stringify(selector(status))}).focus({ preventScroll: true })`);
      const code = input === "Enter" ? 13 : 32;
      await page("Input.dispatchKeyEvent", { type: "keyDown", key: input === "Enter" ? "Enter" : " ", code: input, text: input === "Enter" ? "\r" : " ", windowsVirtualKeyCode: code });
      await page("Input.dispatchKeyEvent", { type: "keyUp", key: input === "Enter" ? "Enter" : " ", code: input, windowsVirtualKeyCode: code });
    }
    await settle();
    const after = await state(status);
    assert.notEqual(after.expanded, before.expanded, `${name}: toggled`);
    const target = after.natural - (before.top - after.listTop);
    const expected = after.listTop + after.natural - Math.max(0, Math.min(after.max, target));
    assert(Math.abs(after.top - expected) <= 1, `${name}: header ${before.top} -> ${after.top}; expected ${expected}`);
    assert(Math.abs(after.scroll - Math.max(0, Math.min(after.max, target))) <= 1, `${name}: expanded content must start below its header`);
    const firstFrame = await evaluate("window.scrollFrame");
    assert(Math.abs(firstFrame - expected) <= 1, `${name}: first frame ${firstFrame}; expected ${expected}`);
    assert(after.top >= after.listTop && after.top + after.height <= after.listBottom, `${name}: header stays visible`);
    if (target >= 0 && target <= after.max) assert(Math.abs(after.top - before.top) <= 1, `${name}: stable anchor`);
    console.log(`${name}: ${before.top} -> ${after.top}, scroll ${before.scroll} -> ${after.scroll}`);
  };
  for (const width of [1280, 390]) {
    await page("Emulation.setDeviceMetricsOverride", { width, height: 900, deviceScaleFactor: 1, mobile: false });
    await page("Page.navigate", { url: new URL('/default/', url).href });
    await wait("document.querySelector('.group-header[aria-controls=\"rows-done\"] small')?.textContent === '1000'", "fixture list");
    await evaluate("document.fonts.ready");
    await settle();
    // Following content allows virtual groups to collapse without clamping.
    await position("canceled", -180);
    await toggle("canceled", "mouse", `${width} prepare following content`);
    await position("todo", 350);
    await toggle("todo", "mouse", `${width} ordinary sticky collapse`);
    await toggle("todo", "Enter", `${width} ordinary expand`);
    await position("in_progress", 1800);
    await toggle("in_progress", "Space", `${width} virtual sticky collapse`);
    await toggle("in_progress", "mouse", `${width} virtual expand`);
    await position("done", -180);
    for (let i = 0; i < 3; i++) {
      await toggle("done", "mouse", `${width} thousand expand ${i}`);
      assert(await evaluate("document.querySelectorAll('#rows-done .list-row').length < 100"), "Virtual rows remain bounded");
      await position("done", 12000);
      await toggle("done", "Enter", `${width} thousand sticky collapse ${i}`);
    }
    await screenshot(`group-scroll-${width}`);
    await toggle("done", "mouse", `${width} thousand reexpand`);
    await position("done", 18000);
    assert(await evaluate(`(() => {
      const list = document.querySelector('.list-view').getBoundingClientRect();
      return [...document.querySelectorAll('#rows-done .list-row')].some(row => {
        const rect = row.getBoundingClientRect(); return rect.top > list.top + 60 && rect.bottom < list.bottom;
      });
    })()`), "Virtual rows visible after manual scrolling");
    await position("canceled", 2500);
    await toggle("canceled", "mouse", `${width} last group collapse at boundary`);
    await toggle("canceled", "Space", `${width} last group reexpand`);
    await position("canceled", 2500);
    await toggle("canceled", "Space", `${width} last group recollapse`);
    await screenshot(`group-scroll-boundary-${width}`);
    assert(await evaluate("document.documentElement.scrollWidth <= innerWidth"), "No horizontal page overflow");
  }
  console.log("PASS: group scroll anchors, native boundaries, keyboard, virtual rows, desktop and narrow viewport");
} finally {
  socket?.close();
  browser.kill();
  await exited;
  fs.rmSync(profile, { recursive: true, force: true });
}
