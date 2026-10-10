// Capture PNG screenshots with headless Chrome over the DevTools protocol.
// Usage: node scripts/browser/screenshot.mjs <shots.json>
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const usage = `Usage: node scripts/browser/screenshot.mjs <shots.json>

shots.json is a JSON array of {"url", "width", "height", "path"} objects.
Each entry loads url in a width x height viewport, waits until the network
is idle, and writes a PNG of the viewport to path; the directory must exist.
Optional fields per entry:
  click  CSS selector; after the network is idle, click that element, wait
         two animation frames, then capture. No match exits 1.
  scale  device pixel ratio, default 1; 4 turns 800x600 into a 3200x2400 PNG.
Chrome runs headless with a temporary profile and picks its own DevTools
port. Chrome is closed and the profile deleted when the script exits.
Set CHROME_PATH to use another Chrome binary. Waiting for DevTools to start
and for each networkIdle gives up after 30 seconds and exits 1.`;

const chrome = process.env.CHROME_PATH || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const waitLimitMs = 30_000;

// Rejects with a message naming what was awaited if promise takes over waitLimitMs.
function withLimit(promise, what) {
  let timer;
  const limit = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(`timed out after ${waitLimitMs / 1000}s waiting for ${what}`)), waitLimitMs);
  });
  return Promise.race([promise, limit]).finally(() => clearTimeout(timer));
}

const arg = process.argv[2];
if (arg === "--help") {
  console.log(usage);
  process.exit(0);
}
if (!arg) {
  console.error(usage);
  process.exit(2);
}

const shots = JSON.parse(fs.readFileSync(arg, "utf8"));
for (const shot of shots) {
  if (typeof shot.url !== "string" || !Number.isInteger(shot.width) || !Number.isInteger(shot.height) || typeof shot.path !== "string") {
    throw new Error(`${arg}: each entry needs url, width, height, and path: ${JSON.stringify(shot)}`);
  }
}

const profile = fs.mkdtempSync(path.join(os.tmpdir(), "inftask-chrome-"));
const browser = spawn(chrome, [
  "--headless",
  `--user-data-dir=${profile}`,
  "--remote-debugging-port=0",
  "--no-first-run",
  "--no-default-browser-check",
  "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
const exited = new Promise((resolve) => {
  browser.on("exit", resolve);
  browser.on("error", resolve);
});

try {
  const browserURL = await withLimit(new Promise((resolve, reject) => {
    let log = "";
    browser.stderr.on("data", (chunk) => {
      log += chunk;
      const match = log.match(/DevTools listening on (ws:\/\/\S+)/);
      if (match) resolve(match[1]);
    });
    browser.on("error", (err) => reject(new Error(`cannot start Chrome at ${chrome}: ${err.message}`)));
    browser.on("exit", (code) => reject(new Error(`Chrome at ${chrome} exited with ${code} before DevTools started:\n${log}`)));
  }), "DevTools to start");
  await capture(browserURL);
} catch (err) {
  console.error(err.message);
  process.exitCode = 1;
} finally {
  browser.kill();
  await exited;
  fs.rmSync(profile, { recursive: true, force: true });
}

async function capture(browserURL) {
  const socket = new WebSocket(browserURL);
  await new Promise((resolve, reject) => {
    socket.onopen = resolve;
    socket.onerror = () => reject(new Error(`cannot connect to ${browserURL}`));
  });

  let nextId = 0;
  const pending = new Map();
  const idleLoaders = new Set();
  let onIdle = () => {};
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (message.id) {
      const request = pending.get(message.id);
      pending.delete(message.id);
      if (message.error) request.reject(new Error(`${request.method}: ${message.error.message}`));
      else request.resolve(message.result);
    } else if (message.method === "Page.lifecycleEvent" && message.params.name === "networkIdle") {
      idleLoaders.add(message.params.loaderId);
      onIdle();
    }
  };
  const send = (method, params, sessionId) => new Promise((resolve, reject) => {
    const id = ++nextId;
    pending.set(id, { method, resolve, reject });
    socket.send(JSON.stringify({ id, method, params, sessionId }));
  });

  const { targetId } = await send("Target.createTarget", { url: "about:blank" });
  const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
  const page = (method, params = {}) => send(method, params, sessionId);
  await page("Page.enable");
  await page("Page.setLifecycleEventsEnabled", { enabled: true });

  for (const shot of shots) {
    await page("Emulation.setDeviceMetricsOverride", { width: shot.width, height: shot.height, deviceScaleFactor: shot.scale ?? 1, mobile: false });
    const { loaderId, errorText } = await page("Page.navigate", { url: shot.url });
    if (errorText) throw new Error(`${shot.url}: ${errorText}`);
    await withLimit(new Promise((resolve) => {
      onIdle = () => idleLoaders.has(loaderId) && resolve();
      onIdle();
    }), `networkIdle for ${shot.url}`);
    if (shot.click) {
      const { result, exceptionDetails } = await page("Runtime.evaluate", {
        expression: `(async () => {
          const el = document.querySelector(${JSON.stringify(shot.click)});
          if (!el) return false;
          el.click();
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
          return true;
        })()`,
        awaitPromise: true,
        returnByValue: true,
      });
      if (exceptionDetails) throw new Error(`${shot.url}: click ${shot.click}: ${exceptionDetails.text}`);
      if (!result.value) throw new Error(`${shot.url}: no element matches click selector ${shot.click}`);
    }
    const { data } = await page("Page.captureScreenshot", { format: "png" });
    fs.writeFileSync(shot.path, Buffer.from(data, "base64"));
    console.log(shot.path);
  }
  socket.close();
}
