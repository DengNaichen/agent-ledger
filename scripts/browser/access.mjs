// Run against TestWorkspaceAuthorizationBrowser, never a maintained instance.
// node scripts/browser/access.mjs /tmp/access-fixture.json /tmp/screenshots
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const fixturePath = process.argv[2];
const output = process.argv[3];
if (!fixturePath || !output) throw new Error("Usage: node scripts/browser/access.mjs <fixture.json> <output-directory>");
const fixture = JSON.parse(fs.readFileSync(fixturePath, "utf8"));
if (new URL(fixture.url).hostname !== "127.0.0.1") throw new Error("Only the loopback test fixture is supported");
fs.mkdirSync(output, { recursive: true });
const chrome = process.env.CHROME_PATH || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const profile = fs.mkdtempSync(path.join(os.tmpdir(), "inftask-access-chrome-"));
const browser = spawn(chrome, ["--headless", `--user-data-dir=${profile}`, "--remote-debugging-port=0", "--no-first-run", "--no-default-browser-check", "about:blank"], { stdio: ["ignore", "ignore", "pipe"] });
const exited = new Promise((resolve) => { browser.on("exit", resolve); browser.on("error", resolve); });
function limit(promise, name) {
  let timer;
  return Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`Timed out waiting for ${name}`)), 30000); })]).finally(() => clearTimeout(timer));
}
let socket;
try {
  const url = await limit(new Promise((resolve, reject) => {
    let log = "";
    browser.stderr.on("data", (chunk) => { log += chunk; const match = log.match(/DevTools listening on (ws:\/\/\S+)/); if (match) resolve(match[1]); });
    browser.on("error", reject);
    browser.on("exit", (code) => reject(new Error(`Chrome exited ${code}: ${log}`)));
  }), "Chrome startup");
  socket = new WebSocket(url);
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
  await page("Network.enable");
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
  const navigate = async (route) => { await page("Page.navigate", { url: fixture.url + route }); };
  const login = async (user, route) => {
    await page("Network.setCookie", { name: fixture.cookie_name, value: fixture.cookies[user], url: fixture.url, httpOnly: true, sameSite: "Lax" });
    await navigate(route);
  };
  const click = (text) => evaluate(`(() => { const button = [...document.querySelectorAll('button')].find(e => e.textContent.trim() === ${JSON.stringify(text)}); if (!button) throw new Error('Missing button: '+${JSON.stringify(text)}); button.click(); })()`);
  const screenshot = async (name) => {
    const { data } = await page("Page.captureScreenshot", { format: "png" }); fs.writeFileSync(path.join(output, name + ".png"), Buffer.from(data, "base64"));
  };
  await page("Emulation.setDeviceMetricsOverride", { width: 1280, height: 900, deviceScaleFactor: 1, mobile: false });
  await login("user-1", "/alpha/settings");
  await wait(`document.querySelectorAll('.member-row').length === 1`, "administrator directory");
  await click("Suspend");
  await wait(`document.body.innerText.includes('cannot remove the last active administrator')`, "last administrator error");
  await screenshot("last-admin-error");
  await click("Create new invite link");
  await wait(`!!document.querySelector('[aria-label="Invitation link"]')`, "invitation link");
  const old = await evaluate(`document.querySelector('[aria-label="Invitation link"]').value`);
  await click("Create new invite link");
  await wait(`document.querySelector('[aria-label="Invitation link"]').value !== ${JSON.stringify(old)}`, "rotated invitation");
  const invitation = await evaluate(`document.querySelector('[aria-label="Invitation link"]').value`);
  await login("user-2", new URL(old).pathname + new URL(old).search);
  await wait(`document.body.innerText.includes('Accept invitation')`, "join page");
  await click("Accept invitation");
  await wait(`document.body.innerText.includes('invitation is invalid or has been reset')`, "old invitation rejection");
  await navigate(new URL(invitation).pathname + new URL(invitation).search);
  await wait(`document.body.innerText.includes('Accept invitation')`, "new invitation page");
  await click("Accept invitation");
  await wait(`location.pathname === '/alpha/' && !!document.querySelector('.workspace-trigger')`, "successful join");
  await navigate("/alpha/settings");
  await wait(`document.querySelectorAll('.member-row').length === 2`, "member directory");
  if (await evaluate(`!!document.querySelector('.members-settings select') || !!document.querySelector('.invite-settings')`)) throw new Error("Member sees admin controls");
  await screenshot("member");
  await login("user-1", "/alpha/settings");
  await wait(`document.querySelectorAll('.member-row').length === 2`, "updated admin directory");
  await evaluate(`document.querySelectorAll('.member-row')[1].querySelector('button').click()`);
  await wait(`document.querySelectorAll('.member-row')[1].innerText.includes('Suspended')`, "suspend success");
  await login("user-2", "/alpha/settings");
  await wait(`document.body.innerText.includes('workspace access denied')`, "suspended user rejection");
  await login("user-1", "/alpha/settings");
  await wait(`document.querySelectorAll('.member-row').length === 2`, "admin reload");
  await click("Restore");
  await wait(`!document.body.innerText.includes('Suspended')`, "restore success");
  await evaluate(`(() => { const select=document.querySelectorAll('.member-row')[1].querySelector('select');select.value='admin';select.dispatchEvent(new Event('change',{bubbles:true})); })()`);
  await wait(`document.querySelectorAll('.member-row')[1].querySelector('select').value === 'admin' && document.body.innerText.includes('Member updated.')`, "promotion");
  await page("Emulation.setDeviceMetricsOverride", { width: 390, height: 844, deviceScaleFactor: 1, mobile: false });
  await screenshot("admin-narrow");
  if (await evaluate(`document.documentElement.scrollWidth > innerWidth`)) throw new Error("Narrow screen overflow");
  await page("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: "dark" }] });
  await screenshot("admin-dark");
  console.log("PASS: invite rotation/join, last-admin error, member controls, suspension/restoration, promotion, narrow layout");
} finally {
  socket?.close(); browser.kill(); await exited; fs.rmSync(profile, { recursive: true, force: true });
  fs.writeFileSync(fixturePath + ".done", "done");
}
