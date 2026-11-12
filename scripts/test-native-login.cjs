const assert = require('node:assert/strict');
const path = require('node:path');
const { createRequire } = require('node:module');
const { spawn } = require('node:child_process');

const root = process.env.ACU_TEST_WORKSPACE || path.resolve(__dirname, '../..');
const { chromium } = process.env.PLAYWRIGHT_MODULE ? require(process.env.PLAYWRIGHT_MODULE) : createRequire(path.join(root, '.build/package.json'))('playwright');
const port = Number(process.env.ACU_NATIVE_INSPECT_PORT || 9231);
const native = spawn(process.env.ACU_NATIVE_TEST_EXE || path.join(root, '.build/acu-runtime-native-login-smoke.exe'), ['-test.run', '^TestNativeLoginSmoke$', '-test.v'], {
  env: { ...process.env, ACU_NATIVE_LOGIN_SMOKE: '1', ACU_NATIVE_INSPECT_PORT: String(port) },
  windowsHide: true,
});
let output = '';
native.stdout.on('data', chunk => { output += chunk; });
native.stderr.on('data', chunk => { output += chunk; });
const exited = new Promise((resolve, reject) => { native.once('exit', resolve); native.once('error', reject); });
const pause = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));

async function until(check, timeout = 60000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const result = await check();
    if (result) return result;
    await pause(200);
  }
  throw new Error('native login / renewal smoke test timed out');
}

async function run() {
  let browser;
  let page;
  let refreshes = 0;
  try {
    await until(async () => {
      try { return (await fetch(`http://127.0.0.1:${port}/json/version`)).ok; } catch { return false; }
    });
    browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`);
    const context = browser.contexts()[0];
    if (process.env.ACU_NATIVE_PROFILE) await context.addCookies([{ name: 'acu-isolated-persistence-check', value: 'fixture-only', url: 'https://acu.best', httpOnly: true, secure: true, sameSite: 'Lax', expires: Math.floor(Date.now()/1000)+3600 }]);
    await context.route('https://acu.best/**', async route => {
      const request = route.request();
      if (new URL(request.url()).pathname === '/api/user/auth/refresh') {
        refreshes++;
        if (refreshes === 2) return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ success: false, code: 'AUTH_INTERNAL_ERROR' }) });
        const renewed = refreshes > 2;
        return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ success: true, data: {
          access_token: renewed ? 'fixture-renewed-access' : 'fixture-first-access',
          access_expires_at: Math.floor(Date.now() / 1000) + (renewed ? 900 : 5),
          session: { sid: 'fixture-server-session' },
        } }) });
      }
      await route.fulfill({ contentType: 'text/html', body: '<!doctype html><meta charset="utf-8"><title>隔离登录测试</title><button id="fixture-login" onclick="history.pushState({},\'\',\'/dashboard\');this.remove()">登录隔离测试账号</button>' });
    });
    page = await until(() => context.pages().find(candidate => candidate.url().startsWith('http://127.0.0.1:') && !candidate.url().includes('/panel')));
    await page.waitForSelector('meta[name="acu-token"]', { state: 'attached' });
    const call = (endpoint, input) => page.evaluate(async ({ endpoint, input }) => {
      const token = document.querySelector('meta[name="acu-token"]').content;
      const response = await fetch(endpoint, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-ACU-Token': token, 'Content-Type': 'application/json' }, body: input === undefined ? undefined : JSON.stringify(input) });
      if (!response.ok) throw new Error(`Runtime test request failed: ${response.status}`);
      return response.json();
    }, { endpoint, input });
    const loginOpened = context.waitForEvent('page');
    await call('/runtime/account/login', { site: 'acu.best' });
    const login = await loginOpened;
    await login.locator('#fixture-login').click();
    await until(async () => (await call('/runtime/status')).account?.selected === 7);
    await page.selectOption('#quick-key', 'account:9');
    await until(async () => (await call('/runtime/status')).account?.selected === 9);
    await until(async () => {
      const status = await call('/runtime/status');
      return refreshes >= 3 && status.account?.loggedIn && !status.account.refreshing && !status.account.error;
    });
    assert.equal(login.isClosed(), false, 'login WebView must remain available for silent renewal');
    assert.equal((await call('/runtime/status')).account.selected, 9, 'renewal must preserve the user-selected Key');
    await page.locator('[data-page="preferences"]').click().catch(async error => {console.error('Native modal:', await page.locator('#overlay').innerText());throw error;});
    await page.getByText('摘要更新', { exact: true }).waitFor().catch(async error => { console.error('Native preference view:', (await page.locator('body').innerText()).slice(-4000)); throw error; });
    await page.locator('.preference-change summary').focus();
    await page.keyboard.press('Enter');
    assert.equal(await page.locator('.preference-change details').getAttribute('open'), '');
    const fs = require('node:fs');
    const screenshotDirectory = path.join(root, 'output/runtime-preferences');
    fs.mkdirSync(screenshotDirectory, { recursive: true });
    await page.screenshot({ path: path.join(screenshotDirectory, 'preferences-native-windows.png'), fullPage: true });
    await page.locator('.starfish-svg').waitFor();
    assert.equal(await page.locator('.starfish-ray-list button').count(),process.env.ACU_NATIVE_STARFISH_MODE==='bipolar'?192:3);
    const graphOutput=path.join(root,'output/runtime-starfish');fs.mkdirSync(graphOutput,{recursive:true});
    for(const colorScheme of ['light','dark'])for(const width of [920,1240]){
      await page.emulateMedia({colorScheme});await page.setViewportSize({width,height:850});
      await page.waitForFunction(theme=>document.documentElement.dataset.acuTheme===theme,colorScheme);
      await page.screenshot({path:path.join(graphOutput,`native-${process.env.ACU_NATIVE_STARFISH_MODE||'ordinary'}-${colorScheme}-${width}.png`),fullPage:false});
    }
    assert.equal((await call('/runtime/status')).account.selected, 9, 'viewing preferences must preserve the selected Key');
    await call('/runtime/desktop/quit', {});
    await browser.close(); browser = null;
    const exitCode = await exited;
    process.stdout.write(output);
    assert.equal(exitCode, 0, 'native Go assertions failed');
    process.stdout.write('PASS: native login, Key switching, transient refresh failure, access expiry, silent recovery and preference comparison.\n');
  } finally {
    if (native.exitCode === null) {
      try {
        if (page && !page.isClosed()) await page.evaluate(() => fetch('/runtime/desktop/quit', { method: 'POST', headers: { 'X-ACU-Token': document.querySelector('meta[name="acu-token"]').content } }));
      } catch {}
    }
    if (browser) await browser.close().catch(() => {});
  }
}

run().catch(error => { process.stderr.write(`${error.stack}\n${output}`); process.exitCode = 1; });
