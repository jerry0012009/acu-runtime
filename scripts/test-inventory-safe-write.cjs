const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const web = path.resolve(__dirname, '../web');
const output = path.join(process.env.ACU_TEST_WORKSPACE || '/tmp/acu-inventory-safe', 'output/inventory-safe');
fs.mkdirSync(output, { recursive: true });

(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
      const page = await browser.newPage({ viewport: { width, height: 950 }, colorScheme: theme });
      const errors = [], writes = [];
      page.on('pageerror', error => errors.push(error.message));
      const catalog = { own: true, only: false, ownHidden: false, models: [
        { id: 'acu-runtime/acu-auto', name: 'acu-auto', inUse: true },
        { id: 'acu-runtime/gemini-4-carbon', name: 'Gemini Carbon' },
        { id: 'acu-runtime/gemini-4-argon', name: 'Gemini Argon' },
      ] };
      const agent = { id: 'codex', name: 'Codex', detected: true, connected: true, catalog };
      await page.route('http://inventory.test/**', async route => {
        const req = route.request(), url = new URL(req.url());
        if (url.pathname === '/runtime/status') return route.fulfill({ json: { desktop: true, platform: 'windows', ready: true, agents: [agent] } });
        if (url.pathname === '/runtime/agents/models') {
          const body = req.postDataJSON(); writes.push(body);
          if ('only' in body) catalog.only = body.only;
          if ('ownHidden' in body) catalog.ownHidden = body.ownHidden;
          if (body.hidden) catalog.models.forEach(model => { model.hidden = body.hidden.includes(model.id); });
          if (body.shown) catalog.models.forEach(model => { model.hidden = !body.shown.includes(model.id); });
          if (body.order) catalog.models.sort((a,b) => (body.order.indexOf(a.id) < 0 ? 1000 : body.order.indexOf(a.id))-(body.order.indexOf(b.id) < 0 ? 1000 : body.order.indexOf(b.id)));
          return route.fulfill({ json: catalog });
        }
        if (url.pathname.startsWith('/kernel/api/agent-models/') && req.method() === 'POST') {
          errors.push('unsafe legacy write');
          return route.fulfill({ status: 409, json: { error: '请在 Codex 卡片选择模型完成安全配置。' } });
        }
        if (url.pathname.startsWith('/runtime/') || url.pathname.startsWith('/api/') || url.pathname.startsWith('/kernel/')) return route.fulfill({ json: {} });
        const file = path.join(web, url.pathname === '/' ? 'index.html' : url.pathname.replace('/ui/', ''));
        if (!fs.existsSync(file)) return route.fulfill({ status: 404 });
        return route.fulfill({ contentType: file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html', body: fs.readFileSync(file, 'utf8').replace('__ACU_TOKEN__', 'fixture') });
      });
      await page.goto('http://inventory.test/');
      await page.locator('[data-action="model-inventory"][data-agent-id="codex"]:enabled').click();
      const save = async action => {
        const response = page.waitForResponse(r => r.url().endsWith('/runtime/agents/models'));
        await action(); await response;
        await page.waitForFunction(() => document.querySelector('#toast')?.textContent.includes('模型显示设置已保存'));
      };
      await save(() => page.locator('[data-inventory-flag="only"]').click());
      assert.equal(catalog.only, true);
      await save(() => page.locator('[data-model-visibility][data-model-id="acu-runtime/gemini-4-argon"]').click());
      assert.equal(catalog.models.find(m => m.id.endsWith('argon')).hidden, true);
      await save(() => page.locator('[data-inventory-flag="own"]').click());
      assert.equal(catalog.ownHidden, true);
      assert.equal(catalog.only, true);
      await page.locator('[data-inventory-tab="order"]').click();
      await save(() => page.locator('[data-model-move="up"][data-model-id="acu-runtime/gemini-4-carbon"]').click());
      assert.equal(catalog.models[0].id, 'acu-runtime/gemini-4-carbon');
      assert.equal(writes.length, 4);
      assert.deepEqual(errors, []);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      await page.screenshot({ path: path.join(output, `${theme}-${width}.png`), fullPage: true });
      await page.close();
    }
    console.log('PASS: Windows Codex inventory only/hidden/own/order safe writes, both themes and widths.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
