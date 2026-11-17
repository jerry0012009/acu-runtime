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
      let failNext = false, delayNext = false;
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
          if (failNext) { failNext = false; return route.fulfill({ status: 409, json: { error: 'fixture save failed' } }); }
          if (delayNext) { delayNext = false; await new Promise(resolve => setTimeout(resolve, 600)); }
          for (const op of body.operations || [body]) {
            if ('only' in op) catalog.only = op.only;
            if ('ownHidden' in op) catalog.ownHidden = op.ownHidden;
            if (op.hidden) catalog.models.forEach(model => { model.hidden = op.hidden.includes(model.id); });
            if (op.shown) catalog.models.forEach(model => { model.hidden = !op.shown.includes(model.id); });
            if (op.order) catalog.models.sort((a,b) => (op.order.indexOf(a.id) < 0 ? 1000 : op.order.indexOf(a.id))-(op.order.indexOf(b.id) < 0 ? 1000 : op.order.indexOf(b.id)));
          }
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
      await page.locator('[data-inventory-flag="only"]').click();
      await page.locator('[data-model-visibility][data-model-id="acu-runtime/gemini-4-argon"]').click();
      await page.locator('[data-inventory-flag="own"]').click();
      await page.locator('[data-inventory-tab="order"]').click();
      await page.locator('[data-model-move="up"][data-model-id="acu-runtime/gemini-4-carbon"]').click();
      assert.equal(writes.length, 0);
      assert.equal(catalog.only, false);
      const response = page.waitForResponse(r => r.url().endsWith('/runtime/agents/models'));
      await page.locator('[data-inventory-save]').click();
      await response;
      await page.waitForFunction(() => document.querySelector('#toast')?.textContent.includes('模型显示设置已保存'));
      assert.equal(catalog.only, true);
      assert.equal(catalog.ownHidden, true);
      assert.equal(catalog.models.find(m => m.id.endsWith('argon')).hidden, true);
      assert.equal(catalog.models[0].id, 'acu-runtime/gemini-4-carbon');
      assert.equal(writes.length, 1);
      assert.equal(writes[0].operations.length, 4);
      await page.locator('[data-inventory-tab="all"]').click();
      await page.locator('[data-inventory-flag="only"]').click();
      await page.locator('.inventory-modal .modal-actions [data-close]').click();
      await page.locator('[data-inventory-stay]').click();
      assert.equal(await page.locator('[data-inventory-flag="only"]').getAttribute('aria-checked'), 'false');
      failNext = true;
      await page.locator('[data-inventory-save]').click();
      await page.getByRole('alert').filter({ hasText: 'fixture save failed' }).waitFor();
      assert.equal(catalog.only, true);
      assert.equal(await page.locator('[data-inventory-flag="only"]').getAttribute('aria-checked'), 'false');
      delayNext = true;
      const retry = page.waitForResponse(r => r.url().endsWith('/runtime/agents/models') && r.status() === 200);
      await page.locator('[data-inventory-save]').click();
      assert.equal(await page.locator('[data-inventory-save]').isDisabled(), true);
      await page.keyboard.press('Escape');
      assert.equal(await page.locator('.inventory-modal').count(), 1);
      await retry;
      await page.waitForFunction(() => document.querySelector('[data-inventory-save]')?.textContent === '保存');
      assert.equal(catalog.only, false);
      await page.locator('[data-inventory-flag="only"]').click();
      await page.locator('.inventory-modal .modal-actions [data-close]').click();
      await page.locator('[data-inventory-discard]').click();
      assert.equal(await page.locator('.inventory-modal').count(), 0);
      assert.equal(catalog.only, false);
      assert.equal(writes.length, 3);
      await page.locator('[data-action="model-inventory"][data-agent-id="codex"]:enabled').click();
      assert.deepEqual(errors, []);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      await page.screenshot({ path: path.join(output, `${theme}-${width}.png`), fullPage: true });
      await page.close();
    }
    console.log('PASS: Windows Codex inventory only/hidden/own/order safe writes, both themes and widths.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
