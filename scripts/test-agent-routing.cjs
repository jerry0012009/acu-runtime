const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { spawn } = require('node:child_process');
const { createRequire } = require('node:module');
const workspace = path.resolve(__dirname, '../..');
const { chromium } = createRequire(path.join(workspace, '.build/package.json'))('playwright');
const assets = path.join(workspace, '.build/acu-runtime-upstream');
const executable = process.env.ACU_ROUTING_TEST_EXE || path.join(workspace, '.build/acu-runtime-routing-web.exe');
const writes = [];
const previews = [];
const catalog = { models: [
  { id: 'fixture/acu-auto', name: 'acu-auto', group: 'ACU', inUse: true },
  { id: 'fixture/gpt-6.1-sol', name: 'Fixture GPT', group: 'ACU' },
  { id: 'fixture/image', name: 'Image', group: 'ACU' },
  { id: 'auto/image', name: 'Image', group: 'Routing groups' },
], own: true, ownHidden: false, only: false };
const groups = [{ ref: 'auto/image', name: 'Image', members: ['fixture/image', 'other/image'], patterns: ['image-*'] }];
const codex = {
  id: 'codex', name: 'Codex', icon: '', path: '/isolated-test/codex/config.toml', wired: true,
  models: { listed: 4, shown: 4, by: [{ name: 'ACU', n: 3 }, { name: 'Routing groups', n: 1 }] },
  fields: [{ key: 'model', label: 'model', value: 'fixture/acu-auto', options: [
    { value: 'fixture/acu-auto', label: 'acu-auto', ref: 'fixture/acu-auto', group: 'ACU' },
    { value: 'fixture/gpt-6.1-sol', label: 'Fixture GPT', ref: 'fixture/gpt-6.1-sol', group: 'ACU' },
    { value: 'chatgpt', label: 'ChatGPT' },
  ] }],
};
const state = { agents: [codex], profiles: [], catalog: '', settings: { lang: 'zh', theme: 'dark' }, clients: [], served: 0 };
const fixture = http.createServer(async (request, response) => {
  const url = new URL(request.url, 'http://localhost');
  const json = data => { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify(data)); };
  if (url.searchParams.get('k') === 'isolated-routing-key') {
    response.setHeader('Set-Cookie', 'fixture=authenticated; Path=/; HttpOnly');
    response.writeHead(303, { Location: '/' }); response.end(); return;
  }
  if (!request.headers.cookie?.includes('fixture=authenticated')) { response.writeHead(401); response.end(); return; }
  if (request.method !== 'GET') {
    let body = '';
    for await (const chunk of request) body += chunk;
    writes.push({ route: url.pathname, body: body ? JSON.parse(body) : {} });
    if (url.pathname.startsWith('/api/agent-models/')) {
      const change = writes.at(-1).body;
      assert(Array.isArray(change.hidden));
      for (const model of catalog.models) model.hidden = !model.inUse && change.hidden.includes(model.id);
      return json(catalog);
    }
    if (url.pathname === '/api/set') {
      const change = writes.at(-1).body;
      if (change.agent !== 'codex' || change.field !== 'model' || !codex.fields[0].options.some(option => option.value === change.value)) {
        response.writeHead(400); return json({ error: 'unexpected mutation' });
      }
      codex.fields[0].value = change.value;
      return json(state);
    }
    if (url.pathname === '/api/agents/disconnect/codex') {
      codex.wired = false;
      codex.fields[0].value = 'chatgpt';
      return json(state);
    }
    response.writeHead(400); return json({ error: 'unexpected mutation' });
  }
  if (url.pathname === '/boot.js') {
    response.setHeader('Content-Type', 'text/javascript');
    response.end('window.bootPrefs={lang:"zh",theme:"dark",web:true,version:"0.1.2891"};'); return;
  }
  if (url.pathname === '/api/state') return json(state);
    if (url.pathname === '/api/agents/preview/codex') {
    previews.push(url.pathname);
      return json({ changes: [{ op: '~', text: `restore ${codex.path}` }, { op: '-', text: 'base_url = "http://127.0.0.1:3425/v1"' }] });
  }
    if (url.pathname === '/api/agents/disconnect/codex') {
      codex.wired = false;
      codex.fields[0].value = 'chatgpt';
      return json(state);
    }
  if (url.pathname === '/api/providers') return json({ providers: [], presets: [], excluded: [], gateway: { running: true, mine: false, url: 'http://127.0.0.1:3425', groups: [], calls: [] } });
  if (url.pathname === '/api/settings') return json({ ...state.settings, version: '0.1.2891', dir: '/isolated-test' });
  if (url.pathname === '/api/groups') return json({ groups, models: [] });
  if (url.pathname.startsWith('/api/agent-models/')) return json(catalog);
  if (url.pathname === '/api/plugins') return json({ plugins: [] });
  if (url.pathname === '/api/library') return json({ servers: [], skills: [], agents: [] });
  if (url.pathname.startsWith('/api/')) return json({});
  const name = url.pathname === '/' ? 'index.html' : decodeURIComponent(url.pathname.slice(1));
  const file = path.join(assets, 'internal_gui_assets_' + name.replaceAll('/', '_'));
  if (!fs.existsSync(file)) { response.writeHead(404); response.end(); return; }
  response.setHeader('Content-Type', { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html', '.svg': 'image/svg+xml' }[path.extname(name)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
async function listen(server) {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return server.address().port;
}
async function main() {
  const port = await listen(fixture);
  const reservation = http.createServer();
  const runtimePort = await listen(reservation);
  await new Promise(resolve => reservation.close(resolve));
  const origin = `http://127.0.0.1:${runtimePort}`;
  const child = spawn(executable, ['--web', `--listen=127.0.0.1:${runtimePort}`, `--kernel-url=http://127.0.0.1:${port}`], {
    windowsHide: true, stdio: 'pipe', env: { ...process.env, ACU_KERNEL_KEY: 'isolated-routing-key' },
  });
  let browser;
  try {
    for (let attempt = 0; attempt < 80; attempt++) {
      try { if ((await fetch(origin + '/runtime/status')).ok) break; } catch {}
      if (attempt === 79) throw new Error('isolated Runtime did not start');
      await delay(100);
    }
    browser = await chromium.launch({ headless: true, executablePath: process.env.ACU_TEST_CHROME || 'C:/Program Files/Google/Chrome/Application/chrome.exe' });
    const page = await browser.newPage({ viewport: { width: 1320, height: 1000 }, colorScheme: 'dark', reducedMotion: 'reduce' });
    const errors = [];
    page.on('pageerror', error => errors.push(error.stack || error.message));
    await page.goto(origin);
    await page.screenshot({ path: path.join(workspace, '.build/acu-runtime-home-dark.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.emulateMedia({ colorScheme: 'light' });
    await page.screenshot({ path: path.join(workspace, '.build/acu-runtime-home-mobile-light.png'), fullPage: true });
    await page.setViewportSize({ width: 1320, height: 1000 });
    await page.emulateMedia({ colorScheme: 'dark' });
    const card = page.locator('.agent-card[data-agent="codex"]');
    await card.locator('select').waitFor();
    assert.equal(await card.locator('select option').count(), 3);
    await card.locator('select').selectOption('fixture/gpt-6.1-sol');
    await page.waitForFunction(() => !document.querySelector('[data-agent-model="codex"]').disabled);
    assert.deepEqual(writes, [{ route: '/api/set', body: { agent: 'codex', field: 'model', value: 'fixture/gpt-6.1-sol' } }]);
    const openPreview = async () => {
      await card.locator('[data-action="agent-toggle"]').click();
      await page.locator('.disconnect-modal').waitFor();
      await page.locator('.disconnect-modal [data-confirm-disconnect]:not([disabled])').waitFor();
      const previewText = await page.locator('.disconnect-modal').innerText();
      if (!previewText.includes(codex.path)) console.error('disconnect preview:', previewText);
      assert(previewText.includes(codex.path));
      return page.locator('.disconnect-modal');
    };
    let disconnectModal = await openPreview();
    assert.equal(writes.length, 1, 'opening restore preview must not write');
    await disconnectModal.locator('[data-close]').first().click();
    await page.locator('.disconnect-modal').waitFor({ state: 'detached' });
    assert.equal(writes.length, 1, 'cancelling must not write');
    disconnectModal = await openPreview();
    await disconnectModal.locator('[data-confirm-disconnect]').click();
    await page.locator('.disconnect-modal').waitFor({ state: 'detached' });
    assert.equal(codex.wired, false);
    assert.equal(codex.fields[0].value, 'chatgpt');
    assert.deepEqual(writes.map(write => write.route), ['/api/set', '/api/agents/disconnect/codex']);
    await page.waitForFunction(() => !document.querySelector('.agent-card[data-agent="codex"] .route-toggle-on'));
    assert(previews.length >= 2);
    await delay(1000);
    assert.deepEqual(errors, []);
    const homeState = {
      ready: true, desktop: true, account: {}, routingGroups: groups,
      connection: { known: true, configured: true, provider: 'fixture', choices: [{ id: 'fixture', name: 'ACU', masked: 'sk-••••demo', models: ['acu-auto', 'gpt-6.1-sol'] }] },
      agents: [
        { id: 'codex', name: 'Codex', detected: true, connected: true, model: 'fixture/acu-auto', modelOptions: codex.fields[0].options },
        { id: 'claude', name: 'Claude Code', detected: true, connected: false, model: 'should-not-appear' },
        { id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: true, restartRequired: true },
        { id: 'hermes', name: 'Hermes', detected: true, connected: false },
      ],
    };
    for (const agent of homeState.agents) agent.catalog = structuredClone(catalog);
    await page.route('**/runtime/status', route => route.fulfill({ json: homeState }));
    await page.goto(origin);
    await page.locator('.quick-check.complete').waitFor();
    assert.equal(await page.locator('.agent-card').count(), 4);
    assert(!(await page.locator('body').innerText()).includes('should-not-appear'));
    assert((await page.locator('[data-agent="codex"] .agent-state').innerText()).includes('默认走 ACU'));
    assert.equal(await page.locator('[data-agent="workbuddy"] select').count(), 0);
    assert.equal(await page.locator('.welcome-help').getAttribute('open'), null);
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await page.screenshot({ path: path.join(workspace, `acu-runtime/preview-v1.21-home-${scheme}.png`), fullPage: true });
    }
    const beforeInventory = writes.length;
    await page.locator('[data-agent="workbuddy"] [data-action="model-inventory"]').click();
    await page.locator('.inventory-modal').waitFor();
    const inventoryText = await page.locator('.inventory-modal').innerText();
    for (const text of ['acu-auto', 'Image', 'WorkBuddy 原生模型', '保留']) assert(inventoryText.includes(text), text);
    for (const text of ['默认 / 角色路由', '未提供完整', '当前没有统一替换', '同名模型按来源区分']) assert(!inventoryText.includes(text), text);
    assert.equal(await page.locator('.inventory-modal table, .inventory-modal h3').count(), 0);
    assert.equal(await page.locator('[data-model-id="auto/image"]').count(), 0);
    assert(!(await page.locator('.inventory-modal').innerText()).includes('自动选择'));
    await page.locator('#inventory-search').fill('IMAGE');
    assert.equal(await page.locator('.inventory-modal [data-flow-search]:visible').count(), 1);
    homeState.agents[2].catalog.models[2].hidden = true;
    await page.waitForFunction(() => document.querySelector('.inventory-modal')?.textContent.includes('已隐藏'));
    assert.equal(await page.locator('#inventory-search').inputValue(), 'IMAGE');
    assert.equal(await page.locator('[data-model-id="auto/image"]').count(), 0);
    await page.locator('#inventory-search').fill('no-matching-model');
    await page.locator('[data-flow-empty]:visible').waitFor();
    await page.locator('#inventory-search').fill('');
    assert.equal(await page.locator('[data-model-id="auto/image"]').count(), 0);
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await page.screenshot({ path: path.join(workspace, `acu-runtime/preview-v1.21-models-${scheme}.png`) });
    }
    await page.setViewportSize({ width: 620, height: 700 });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    const inventoryButton = await page.locator('.inventory-modal .recommendation-footer').boundingBox();
    assert(inventoryButton.y + inventoryButton.height <= 700);
    await page.screenshot({ path: path.join(workspace, 'acu-runtime/preview-v1.21-models-narrow.png') });
    await page.locator('.inventory-modal [data-close]').first().click();
    assert.equal(writes.length, beforeInventory);
    await page.setViewportSize({ width: 1320, height: 1000 });
    await card.locator('[data-action="model-inventory"]').click();
    await page.locator('.inventory-modal').waitFor();
    assert.equal(writes.length, beforeInventory, 'opening the inline picker does not modify routing');
    await page.locator('[data-model-visibility][data-model-id="fixture/image"]').click();
    await page.waitForFunction(() => document.querySelector('#toast')?.textContent.includes('模型显示设置已保存')); 
    assert.equal(writes.at(-1).route, '/api/agent-models/codex');
    assert.equal(codex.wired, false, 'visibility switch must not reconnect the Agent');
    assert(writes.at(-1).body.hidden.includes('fixture/image'));
    assert(!writes.at(-1).body.hidden.includes('fixture/acu-auto'), 'in-use model remains available');
    assert.equal(await page.locator('[data-native-model-list]').count(), 0, 'the model list no longer jumps to native settings');
    await page.locator('.inventory-modal [data-close]').first().click();
    await page.locator('.agent-card').first().waitFor();
    await page.locator('[data-action="apply-agents"]').click();
    await page.locator('.setup-modal').waitFor();
    assert.equal(await page.locator('[data-setup-agent]:checked').count(), 2);
    await page.locator('.setup-modal [data-close]').first().click();
    homeState.agents[0].connected = false;
    homeState.account = { loggedIn: true, username: 'ACU 用户', keys: [] };
    await page.route('**/api/settings', route => route.fulfill({ json: { visionAuto: 'fixture/vision', visionModels: [{ id: 'fixture/vision', name: '识图模型', pname: '示例来源' }], imageGen: 'off', searchAPIs: [] } }));
    await page.goto(origin);
    const writesBeforePreview = writes.length;
    await card.locator('[data-action="agent-toggle"]').click();
    await page.locator('.recommendation-modal [data-confirm-setup]:enabled').waitFor();
    assert.equal(await page.locator('[data-setup-agent]').count(), 0);
    assert.equal(await page.locator('#setup-auxiliary').count(), 0);
    assert.equal(await page.locator('#setup-model').inputValue(), 'acu-auto');
    assert((await page.locator('.recommendation-modal').innerText()).includes('不把所有 GPT 模型统一改走 ACU'));
    assert((await page.locator('.capability-table').innerText()).includes('识图模型 · 示例来源'));
    await page.locator('#setup-model').selectOption('gpt-6.1-sol');
    assert((await page.locator('.recommendation-route').innerText()).includes('Codex → gpt-6.1-sol → ACU'));
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await page.screenshot({ path: path.join(workspace, `acu-runtime/preview-v1.18-recommendation-${scheme}.png`), fullPage: true });
    }
    await page.setViewportSize({ width: 620, height: 700 });
    await page.locator('.recommendation-models summary').click();
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    assert(await page.locator('[data-confirm-setup]').isVisible());
    const confirmBounds = await page.locator('[data-confirm-setup]').boundingBox();
    assert(confirmBounds.y + confirmBounds.height <= 700);
    await page.screenshot({ path: path.join(workspace, 'acu-runtime/preview-v1.18-recommendation-narrow.png'), fullPage: true });
    await page.locator('.setup-modal [data-close]').first().click();
    assert.equal(writes.length, writesBeforePreview, 'recommendation and model preview must not write');
    homeState.agents[2].connected = false;
    await page.goto(origin);
    await page.locator('[data-agent="workbuddy"] [data-action="agent-toggle"]').click();
    await page.locator('.recommendation-modal [data-confirm-setup]:enabled').waitFor();
    assert.equal(await page.locator('#setup-model').count(), 0);
    assert((await page.locator('.recommendation-modal').innerText()).includes('在 WorkBuddy 任务内选择'));
    const configured = [];
    await page.route('**/runtime/agents/configure', async route => {
      configured.push(route.request().postDataJSON());
      homeState.agents[2].connected = true;
      await route.fulfill({ json: { agents: [{ id: 'workbuddy', wired: true }] } });
    });
    await page.locator('[data-confirm-setup]').click();
    await page.locator('.setup-result').waitFor();
    assert.deepEqual(configured, [{ agent: 'workbuddy', provider: 'fixture', model: 'acu-auto', auxiliary: false }]);
    assert.equal(writes.length, writesBeforePreview + 1, 'confirmation also saves recommended visibility in the isolated kernel');
    assert.equal(writes.at(-1).route, '/api/agent-models/workbuddy');
    await page.unroute('**/runtime/agents/configure');
    await page.unroute('**/api/settings');
    await page.setViewportSize({ width: 1320, height: 1000 });
    const beforeHome = writes.length;
    await page.locator('#quick-key').focus();
    homeState.connection = { known: true, configured: false, choices: [] };
    await page.waitForFunction(() => !document.querySelector('.quick-check.complete'));
    assert(await page.locator('[data-action="apply-agents"]').isDisabled());
    assert(await page.locator('.welcome-steps').isVisible());
    await page.locator('[data-action="manage-connection"]').first().click();
    await page.locator('#acu-key').fill('isolated-unsaved-key');
    homeState.account = { error: '离线测试提示' };
    await page.locator('.quick-error').waitFor();
    assert.equal(await page.locator('#acu-key').inputValue(), 'isolated-unsaved-key');
    await page.locator('.manager-heading [data-action="manage-connection"]').click();
    await page.setViewportSize({ width: 620, height: 860 });
    await page.emulateMedia({ colorScheme: 'light' });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await page.screenshot({ path: path.join(workspace, 'acu-runtime/preview-v1.17-narrow-light.png'), fullPage: true });
    await page.locator('[data-page="settings"]').click();
    for (const text of ['工作区', '开发说明', '内核已就绪', '发布与跨平台', '查看开源许可', '内核路径']) assert(!(await page.locator('body').innerText()).includes(text));
    assert.equal(writes.length, beforeHome, 'home setup preview and cancellation must not mutate configuration');
    await page.unroute('**/runtime/status');
    await page.setViewportSize({ width: 1320, height: 1000 });
    await page.emulateMedia({ colorScheme: 'dark' });
    codex.wired = true;
    const beforePanel = writes.length;
    await page.goto(origin + '/panel');
    const overview = page.frameLocator('#panel');
    try { await overview.locator('.row.agent[data-id="codex"] .who').waitFor({ timeout: 10000 }); }
    catch (failure) {
      console.error(JSON.stringify({ errors, body: await page.locator('body').innerText(), frames: page.frames().map(frame => frame.url()) }));
      await page.screenshot({ path: path.join(workspace, '.build/panel-integration-failure.png') });
      throw failure;
    }
    assert.equal(await overview.locator('#ptabs button:visible').count(), 3);
    await overview.locator('.row.agent[data-id="codex"] .who').click();
    await overview.locator('.row.agent.open').waitFor();
    await delay(4500);
    assert.equal(await overview.locator('.row.agent.open').count(), 1, 'status polling must preserve expanded native controls');
    await overview.locator('.ag-conn[aria-checked="true"]').click();
    await overview.locator('.disconnect-ask').waitFor();
    await overview.locator('.disconnect-ask .bar button:not(.danger-fill)').click();
    assert.equal(writes.length, beforePanel, 'native panel preview and cancel must not mutate configuration');
    await overview.locator('[data-ptab="stats"]').click();
    await overview.locator('#panelUsage').waitFor();
    assert(await overview.locator('.acu-usage-note').isVisible());
    await overview.locator('[data-ptab="routing"]').click();
    await overview.locator('#panelRouting').waitFor();
    await overview.locator('[data-ptab="agents"]').click();
    await page.emulateMedia({ colorScheme: 'light' });
    await page.waitForFunction(() => document.querySelector('iframe').contentDocument.documentElement.dataset.acuTheme === 'light');
    assert.equal(writes.length, beforePanel);
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ nativeAssets: true, modelChange: true, restorePreview: true, cancelWrites: 0, confirmDisconnect: true, homepageUpdated: true, nativePanelTabs: 3, expandedControlsPreserved: true, systemTheme: true, realConfigurationWrites: 0 }));
  } finally {
    await browser?.close();
    child.kill();
    await new Promise(resolve => fixture.close(resolve));
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; fixture.close(); });



