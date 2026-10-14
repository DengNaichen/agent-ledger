// Verify Cycle flows only on the disposable fixture created by seed.mjs.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
const [base, output] = process.argv.slice(2);
if (!base || !output) throw new Error("Usage: node scripts/browser/cycles.mjs <loopback-url> <output-dir>");
const url = new URL(base);
if (url.protocol !== "http:" || url.hostname !== "127.0.0.1" || url.port === "8700") throw new Error("Disposable loopback only");
const fixture = await (await fetch(new URL("/api/w/default/issues/AL-2",url), {signal: AbortSignal.timeout(5000)})).json();
assert.equal(fixture.title,"Define the agent_ready gate for tickets");
fs.mkdirSync(output,{recursive:true});
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

  const clickText = (text) => evaluate(`(() => { const button = [...document.querySelectorAll('button')].find(e => e.textContent.trim() === ${JSON.stringify(text)}); if (!button) throw Error('Missing button'); button.click(); })()`);
  const fill = (selector,value) => evaluate(`(() => { const el=document.querySelector(${JSON.stringify(selector)}); const proto=el.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:el.tagName==='SELECT'?HTMLSelectElement.prototype:HTMLInputElement.prototype; Object.getOwnPropertyDescriptor(proto,'value').set.call(el,${JSON.stringify(value)}); el.dispatchEvent(new Event(el.tagName==='SELECT'?'change':'input',{bubbles:true})); })()`);
  const navigate = async (pathname,ready) => {await page('Page.navigate',{url:new URL(pathname,url).href});await wait(ready,pathname);};
  await page('Emulation.setDeviceMetricsOverride',{width:1280,height:900,deviceScaleFactor:1,mobile:false});
  await navigate('/default/team/AL/cycles',"!!document.querySelector('.cycle-empty')");
  await clickText('Enable cycles');
  await wait("!!document.querySelector('dialog[open]')",'settings dialog');
  await screenshot('settings-desktop');
  await clickText('Save changes');
  await wait("document.querySelectorAll('.cycle-card').length===3 && !document.querySelector('dialog')",'enabled cycles');
  const currentPath=await evaluate("new URL(document.querySelector('.cycle-card.current').href).pathname");
  const currentID=Number(currentPath.split('/').at(-1));
  await click('.cycle-card.current');
  await wait("!!document.querySelector('.cycle-overview')",'cycle detail');
  await clickText('New issue');
  await wait("!!document.querySelector('dialog[open]')",'new issue');
  assert.equal(await evaluate("Number(document.querySelector('select[aria-label=\"New issue cycle\"]').value)"),currentID);
  await fill('.composer-title input','Cycle browser fixture');
  await clickText('Create issue');
  await wait("!document.querySelector('dialog') && document.body.textContent.includes('Cycle browser fixture')",'new cycle issue');
  const issuePath=await evaluate("[...document.querySelectorAll('a')].find(e=>e.textContent==='View issue').getAttribute('href')");
  await navigate(issuePath,"!!document.querySelector('select[aria-label^=\"Change cycle\"]')");
  await wait("!document.querySelector('select[aria-label^=\"Change cycle\"]').disabled",'cycle choices');
  const select='select[aria-label^="Change cycle"]';
  await fill(select,'0');
  await wait(`document.querySelector(${JSON.stringify(select)}).value==='0' && !document.querySelector(${JSON.stringify(select)}).disabled`,'cleared cycle');
  await fill(select,String(currentID));
  await wait(`document.querySelector(${JSON.stringify(select)}).value==='${currentID}' && !document.querySelector(${JSON.stringify(select)}).disabled`,'saved cycle');
  await page('Network.emulateNetworkConditions',{offline:true,latency:0,downloadThroughput:-1,uploadThroughput:-1});
  await fill(select,'0');
  await wait("!!document.querySelector('[role=alert]')",'failed cycle save');
  assert.equal(await evaluate(`document.querySelector(${JSON.stringify(select)}).value`),String(currentID));
  await screenshot('assignment-offline');
  await page('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1});
  await navigate(issuePath,"!!document.querySelector('select[aria-label^=\"Change cycle\"]')");
  await wait(`document.querySelector(${JSON.stringify(select)}).value==='${currentID}' && !document.querySelector(${JSON.stringify(select)}).disabled`,'persisted cycle');
  await navigate('/default/team/AL/issues',"!!document.querySelector('.cycle-filter select')");
  await wait("!document.querySelector('.cycle-filter select').disabled",'cycle filter');
  await fill('.cycle-filter select',String(currentID));
  await wait("document.body.textContent.includes('Cycle browser fixture') && document.querySelectorAll('.list-row').length===1",'cycle filtered issues');
  await navigate('/default/team/AL/cycles',"document.querySelectorAll('.cycle-card').length===3");
  await clickText('Cycle settings');
  await wait("!!document.querySelector('dialog')",'settings');
  await evaluate(`(() => {const el=[...document.querySelectorAll('dialog label')].find(e=>e.textContent==='Upcoming cycles').querySelector('input');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(el,'3');el.dispatchEvent(new Event('input',{bubbles:true}));})()`);
  await page('Network.emulateNetworkConditions',{offline:true,latency:0,downloadThroughput:-1,uploadThroughput:-1});
  await clickText('Save changes');
  await wait("!!document.querySelector('dialog [role=alert]')",'settings failure');
  await screenshot('settings-offline');
  await page('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1});
  await page('Page.bringToFront');
  await evaluate(`document.querySelector('dialog footer button').focus()`);
  await page('Input.dispatchKeyEvent',{type:'keyDown',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
  await page('Input.dispatchKeyEvent',{type:'keyUp',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
  await wait("!document.querySelector('dialog')",'Escape');
  for(const width of [1280,1024,760,390]) {
    await page('Emulation.setDeviceMetricsOverride',{width,height:900,deviceScaleFactor:1,mobile:false});
    for(const pathname of ['/default/team/AL/cycles',currentPath,issuePath]) {
      await navigate(pathname,pathname.endsWith('/cycles')?"document.querySelectorAll('.cycle-card').length===3":pathname===currentPath?"!!document.querySelector('.cycle-overview')":"!!document.querySelector('.detail-properties')");
      assert(!(await evaluate('document.documentElement.scrollWidth > innerWidth')),`overflow ${width} ${pathname}`);
      for(const theme of ['light','dark']) {await page('Emulation.setEmulatedMedia',{features:[{name:'prefers-color-scheme',value:theme}]});await screenshot(`${pathname===currentPath?'cycle':pathname===issuePath?'issue':'cycles'}-${width}-${theme}`);}
    }
  }
  console.log('PASS: configure, create in cycle, assign/clear/persist, filter, offline assignment/settings, Escape, desktop/1024/760/390, light/dark');
} finally {
  socket?.close();
  browser.kill();
  await exited;
  fs.rmSync(profile, { recursive: true, force: true });
}
