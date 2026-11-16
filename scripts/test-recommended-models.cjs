const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const root = process.env.ACU_TEST_WORKSPACE || '/tmp/acu-recommended-models-browser';
const { chromium } = process.env.PLAYWRIGHT_MODULE ? require(process.env.PLAYWRIGHT_MODULE) : createRequire(path.join(root, 'package.json'))('playwright');
const web = path.resolve(__dirname, '../web');
const output = path.join(root, 'output/runtime-recommendations');
fs.mkdirSync(output, { recursive: true });

(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    for (const theme of ['light', 'dark']) for (const width of [390, 1440]) for (const connected of [false, true]) for (const agentId of ['workbuddy','codex','claude','hermes']) {
      const page = await browser.newPage({ viewport: { width, height: 950 }, colorScheme: theme });
      const errors = [], changes = [], configurations = [];
      page.on('pageerror', error => errors.push(error.message));
      const choice = { id: 'acu-runtime', name: 'ACU', masked: 'fixture', models: ['acu-auto', 'claude-fable-5', 'claude-fable-5-1','claude-fable-5-2','gemini-4-carbon','gemini-4-hydrogen','kimi-k3.1-pro','grok-4.7','gemini-3.8-flash'] };
      const agent = { id: agentId, name: agentId, detected: true, connected, wired: connected, modelOptions: choice.models.map(model=>({value:'acu-runtime/'+model,ref:'acu-runtime/'+model})) };
      const catalog = { models: [
        { id: 'acu-runtime/acu-auto' },
        { id: 'acu-runtime/claude-fable-5-1', hidden: true },
        { id: 'acu-runtime/claude-fable-5-2', hidden: true },
        { id: 'acu-runtime/gemini-4-carbon', hidden: true },
        { id: 'acu-runtime/gemini-4-hydrogen', hidden: true },
        { id: 'acu-runtime/kimi-k3.1-pro', hidden: true },
        { id: 'acu-runtime/grok-4.7', hidden: true },
        { id: 'acu-runtime/gemini-3.8-flash', hidden: true },
        { id: 'acu-runtime/claude-fable-5' },
        { id: 'other/visible' },
      ] };
      await page.route('http://recommendations.test/**', async route => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.pathname === '/runtime/status') return route.fulfill({ json: { ready: true, desktop: false, agents: [agent], connection: { known: true, configured: true, provider: choice.id, choices: [choice] } } });
        if (url.pathname === '/kernel/api/provider/models') return route.fulfill({json:{}});
        if (url.pathname === '/runtime/agents/configure') {
          configurations.push(request.postDataJSON());
          return route.fulfill({ json: { agents: [{ id: agentId, wired: true }] } });
        }
        if (url.pathname === '/kernel/api/agent-models/'+agentId) {
          if (request.method() === 'POST') { changes.push(request.postDataJSON()); return route.fulfill({ json: {} }); }
          return route.fulfill({ json: catalog });
        }
        if (url.pathname === '/runtime/capabilities') return route.fulfill({ json: { searchReady: true, tools: [] } });
        if (url.pathname.startsWith('/runtime/') || url.pathname.startsWith('/api/')) return route.fulfill({ json: {} });
        const file = path.join(web, url.pathname === '/' ? 'index.html' : url.pathname.replace('/ui/', ''));
        if (!fs.existsSync(file)) return route.fulfill({ status: 404, body: '' });
        return route.fulfill({ contentType: file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html', body: fs.readFileSync(file, 'utf8').replace('__ACU_TOKEN__', 'fixture') });
      });
      await page.goto('http://recommendations.test/');
      await page.locator('[data-action="apply-agents"]').click();
      await page.locator('.setup-adjustments > summary').click();
      assert.equal(await page.locator('#setup-model option[value="claude-fable-5-1"]').count(), 1);
      assert.equal(await page.locator('#setup-model option[value="claude-fable-5-2"]').count(), 1);
      assert.equal(await page.locator('#setup-model option[value="gemini-4-carbon"]').count(), 1);
      assert.equal(await page.locator('#setup-model option[value="claude-fable-5"]').count(), 0);
      for (const model of ['kimi-k3.1-pro','grok-4.7','gemini-3.8-flash']) assert.equal(await page.locator(`#setup-model option[value="${model}"]`).count(), 1);
      assert.equal(await page.locator('#setup-model').inputValue(), 'acu-auto');
      assert(await page.locator('body').evaluate(body => body.scrollWidth <= innerWidth));
      await page.screenshot({ path: path.join(output, `${connected ? 'existing' : 'fresh'}-${theme}-${width}.png`), fullPage: true });
      const visibilitySaved = page.waitForResponse(response => response.url().endsWith('/kernel/api/agent-models/'+agentId) && response.request().method() === 'POST');
      await page.locator('[data-confirm-setup]').click();
      await visibilitySaved;
      assert.equal(configurations.length, 1);
      assert.equal(configurations[0].model, 'acu-auto');
      assert.equal(configurations[0].preserve === true, connected);
      assert.deepEqual(changes, [{ hidden: [] }]);
      assert.deepEqual(errors, []);
      await page.close();
    }
    console.log('PASS: Fable 5.2/5.1 recommendations and fresh/existing WorkBuddy setup across both themes and widths.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
