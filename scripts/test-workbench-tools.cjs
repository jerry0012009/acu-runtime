/* global AbortSignal, fetch, setTimeout */
const assert = require('node:assert/strict');
const fs = require('node:fs');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const reserve = () => new Promise((resolve, reject) => {
  const server = net.createServer().listen(0, '127.0.0.1', () => {
    const port = server.address().port;
    server.close(error => error ? reject(error) : resolve(port));
  });
});
const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
async function ready(url, check = response => response.ok) {
  for (let attempt = 0; attempt < 150; attempt++) {
    try { if (await check(await fetch(url, { redirect: 'manual', signal: AbortSignal.timeout(1500) }))) return; } catch {}
    await delay(200);
  }
  throw new Error(`Timed out waiting for ${new URL(url).pathname}`);
}

async function main() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'acu-workbench-tools-'));
  const key = 'acu-isolated-magpie-key-20261116';
  const magpiePort = await reserve();
  const runtimePort = await reserve();
  const gatewayPort = await reserve();
  const env = { ...process.env, HOME: root, USERPROFILE: root,
    XDG_CONFIG_HOME: path.join(root, 'config'), XDG_DATA_HOME: path.join(root, 'data'),
    CODEX_HOME: path.join(root, 'codex'), CLAUDE_CONFIG_DIR: path.join(root, 'claude'),
    MAGPIE_WEB_KEY: key, ACU_KERNEL_KEY: key, MAGPIE_ADDR: `127.0.0.1:${gatewayPort}` };
  fs.mkdirSync(env.XDG_CONFIG_HOME, { recursive: true });
  fs.mkdirSync(env.XDG_DATA_HOME, { recursive: true });
  const magpie = spawn(process.env.MAGPIE_BIN || '/root/jerry/magpie/bin/magpie',
    ['web', '--addr', `127.0.0.1:${magpiePort}`, '--no-open'], { env, stdio: 'ignore' });
  let runtime;
  let runtimeLog = '';
  let browser;
  try {
    await ready(`http://127.0.0.1:${magpiePort}/?k=${key}`, response => response.status === 303);
    runtime = spawn(process.env.ACU_RUNTIME_TEST_EXE || path.join(__dirname, '../.build/workbench-runtime'),
    ['--web', `--listen=127.0.0.1:${runtimePort}`, `--kernel-url=http://127.0.0.1:${magpiePort}`],
    { env, stdio: 'pipe' });
  runtime.stdout.on('data', data => { runtimeLog += data; });
  runtime.stderr.on('data', data => { runtimeLog += data; });
    try { await ready(`http://127.0.0.1:${runtimePort}/`, response => response.ok); }
    catch (error) { throw new Error(`${error.message}: ${runtimeLog}`); }
    browser = await chromium.launch({ headless: true });
    const errors = [];
    const page = await browser.newPage();
    page.on('pageerror', error => errors.push(error.message));
    for (const [width, theme] of [[1440, 'light'], [390, 'light'], [1440, 'dark'], [390, 'dark']]) {
      await page.setViewportSize({ width, height: 1050 });
      await page.emulateMedia({ colorScheme: theme });
      await page.goto(`http://127.0.0.1:${runtimePort}/`);
      const other = page.locator('[data-agent-section="other"]');
      await other.locator(':scope > summary').click();
      await page.locator('.tapnow-companion').waitFor();
      assert.equal(await page.locator('.tapnow-backup').getAttribute('open'), null, 'backup starts secondary and collapsed');
      assert.equal(await page.locator('.tapnow-agent > .tapnow-heading').count(), 0, 'generic plugin heading no longer outranks companion');
      assert((await page.locator('.tapnow-companion').innerText()).includes('副驾驶'));
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `TapNow layout overflow ${width} ${theme}`);
      await page.screenshot({ path: `/tmp/acu-workbench-tapnow-${width}-${theme}.png`, fullPage: true });
    }

    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.emulateMedia({ colorScheme: 'light' });
    await page.goto(`http://127.0.0.1:${runtimePort}/`);
    await page.locator('[data-agent-section="install"] > summary').click();
    await page.getByRole('button', { name: /Agent 安装目录/ }).click();
    const agentFrame = page.frameLocator('iframe[title="Agent 安装目录"]');
    await agentFrame.locator('#agentsInstall').waitFor();
    await agentFrame.locator('.ag-install-head').waitFor();
    try { await agentFrame.locator('#agentsInstall .ag-install-row').first().waitFor({ timeout: 10000 }); }
    catch (error) { console.error(await agentFrame.locator('#agentsInstall').innerText()); await page.screenshot({ path:'/tmp/acu-workbench-install-failed.png', fullPage:true }); throw error; }
    await page.screenshot({ path: '/tmp/acu-workbench-agent-install.png', fullPage: true });

    await page.goto(`http://127.0.0.1:${runtimePort}/`);
    await page.getByRole('button', { name: '安装工具与 MCP' }).click();
    const library = page.frameLocator('iframe[title="工具与 MCP"]');
    await library.locator('#view-library:not([hidden])').waitFor();
    try { await library.locator('.lib-tabs button[aria-pressed="true"]').filter({ hasText: /^MCP\b/ }).waitFor({ timeout: 10000 }); }
    catch (error) { console.error(await library.locator('.lib-tabs').evaluate(node=>({html:node.outerHTML,buttons:[...node.querySelectorAll('button')].map(b=>({text:b.textContent,onclick:typeof b.onclick})),url:location.href,classes:document.body.className})),errors); await page.screenshot({path:'/tmp/acu-workbench-library-failed.png',fullPage:true}); throw error; }
    await page.screenshot({ path: '/tmp/acu-workbench-mcp-library.png', fullPage: true });
    const context7 = library.locator('.mk-card[data-id="context7"]');
    await context7.waitFor();
    const saved = page.waitForResponse(response => response.url().includes('/api/library/market/server') && response.request().method() === 'POST');
    await context7.locator('.mk-add').click();
    assert((await saved).ok(), 'native MCP installation succeeds');
    await library.locator('.lib-server-name,.lib-name,.name').filter({hasText:/context7/i}).first().waitFor();
    for (const width of [1440, 390]) {
      await page.setViewportSize({width, height:1050});
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
      await page.screenshot({path:`/tmp/acu-workbench-mcp-installed-${width}.png`,fullPage:true});
    }
    assert.deepEqual(errors, []);
    console.log('PASS: real Magpie Agent install directory and Context7 MCP installation reused in an isolated user directory; TapNow hierarchy and responsive layouts verified.');
  } finally {
    if (browser) await browser.close();
    for (const child of [runtime, magpie]) {
      if (child && child.exitCode === null) { child.kill(); await new Promise(resolve => child.once('exit', resolve)); }
    }
    fs.rmSync(root, { recursive: true, force: true });
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
