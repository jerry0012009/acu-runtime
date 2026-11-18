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
      const choice = { id: 'acu-runtime', name: 'ACU', masked: 'fixture', models: ['acu-auto', 'gpt-6.2-sol', 'gpt-6.2-astra', 'claude-opus-4-8', 'claude-opus-5-5', 'gpt-6-sol', 'gpt-6.1-sol', 'claude-fable-5', 'claude-fable-5-1','claude-fable-5-2','gemini-4-carbon','gemini-4-hydrogen','kimi-k3.1-pro','grok-4.7','gemini-3.8-flash'] };
      const agent = { id: agentId, name: agentId, detected: true, connected, wired: connected, model: connected ? 'acu-runtime/claude-fable-5' : '', modelOptions: choice.models.map(model=>({value:'acu-runtime/'+model,ref:'acu-runtime/'+model})) };
      if (agentId === 'codex' && connected) { agent.needsRepair = true; agent.driftKind = 'unwired'; }
      const catalog = { models: [
        { id: 'acu-runtime/acu-auto' },
        { id: 'acu-runtime/gpt-6.2-sol', hidden: true },
        { id: 'acu-runtime/gpt-6.2-astra', hidden: true },
        { id: 'acu-runtime/claude-fable-5-1', hidden: true },
        { id: 'acu-runtime/claude-fable-5-2', hidden: true },
        { id: 'acu-runtime/gemini-4-carbon', hidden: true },
        { id: 'acu-runtime/gemini-4-hydrogen', hidden: true },
        { id: 'acu-runtime/kimi-k3.1-pro', hidden: true },
        { id: 'acu-runtime/grok-4.7', hidden: true },
        { id: 'acu-runtime/gemini-3.8-flash', hidden: true },
        { id: 'acu-runtime/claude-fable-5' },
        { id: 'acu-runtime/claude-opus-4-8' },
        { id: 'acu-runtime/claude-opus-5-5', hidden: true },
        { id: 'acu-runtime/gpt-6-sol', hidden: true },
        { id: 'acu-runtime/gpt-6.1-sol', hidden: true },
        { id: 'other/visible' },
      ] };
      await page.route('http://recommendations.test/**', async route => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.pathname === '/runtime/status') return route.fulfill({ json: { ready: true, desktop: false, agents: [agent], connection: { known: true, configured: true, provider: choice.id, choices: [choice] } } });
        if (url.pathname === '/kernel/api/provider/models') return route.fulfill({json:{}});
        if (url.pathname === '/runtime/models/prepare') {
          for (const model of ['gpt-6.2-sol', 'gpt-6.2-astra']) assert(request.postDataJSON().models.includes(model));
          assert(request.postDataJSON().excludedModels.includes('gemini-4-carbon'));
          return route.fulfill({json:{ok:true}});
        }
        if (url.pathname === '/runtime/agents/models') {
          const body = request.postDataJSON(); changes.push(body);
          catalog.models = catalog.models.map(model=>({...model,hidden:body.hidden.includes(model.id)}));
          return route.fulfill({json:catalog});
        }
        if (url.pathname === '/runtime/agents/configure') {
          const body = request.postDataJSON();
          configurations.push(body);
          if (agentId === 'codex' && connected) agent.needsRepair=false;
          if (body.preserve !== true && agentId !== 'workbuddy') agent.model = `acu-runtime/${body.model}`;
          catalog.models = catalog.models.map(model => model.id === `acu-runtime/${body.model}` ? { ...model, hidden: false } : model);
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
      assert.equal(await page.locator('#setup-model option[value="claude-fable-5-1"]').count(), 0);
      assert.equal(await page.locator('#setup-model option[value="claude-opus-4-8"]').count(), 0);
      for (const model of ['claude-opus-5-5','gpt-6-sol','gpt-6.1-sol']) assert.equal(await page.locator(`#setup-model option[value="${model}"]`).count(),1);
      assert.equal(await page.locator('#setup-model option[value="claude-fable-5-2"]').count(), 1);
      assert.equal(await page.locator('#setup-model option[value="gemini-4-carbon"]').count(), 0);
      assert.equal(await page.locator('#setup-model option[value="gemini-4-hydrogen"]').count(), 1);
      for (const model of ['gpt-6.2-sol', 'gpt-6.2-astra']) assert.equal(await page.locator(`#setup-model option[value="${model}"]`).count(), 1);
      assert.equal(await page.locator('#setup-model option[value="claude-fable-5"]').count(), 0);
      for (const model of ['kimi-k3.1-pro','grok-4.7']) assert.equal(await page.locator(`#setup-model option[value="${model}"]`).count(), 1);
      assert.equal(await page.locator('#setup-model').inputValue(), 'acu-auto');
      if (!connected && agentId !== 'workbuddy') await page.locator('#setup-model').selectOption(theme === 'light' ? 'gpt-6.2-sol' : 'gpt-6.2-astra');
      assert(await page.locator('body').evaluate(body => body.scrollWidth <= innerWidth));
      assert(await page.locator('.setup-modal').evaluate(modal => modal.scrollWidth <= modal.clientWidth));
      await page.screenshot({ path: path.join(output, `${agentId}-${connected ? 'existing' : 'fresh'}-${theme}-${width}.png`), fullPage: true });
      const visibilitySaved = page.waitForResponse(response => response.url().endsWith('/runtime/agents/models') && response.request().method() === 'POST');
      await page.locator('[data-confirm-setup]').click();
      await visibilitySaved;
      assert.equal(configurations.length, 1);
      assert.equal(configurations[0].model, connected ? 'claude-fable-5-2' : agentId !== 'workbuddy' ? (theme === 'light' ? 'gpt-6.2-sol' : 'gpt-6.2-astra') : 'acu-auto');
      assert.equal(configurations[0].preserve === true, false);
      if (connected && agentId !== 'workbuddy') assert.equal(agent.model,'acu-runtime/claude-fable-5-2');
      assert.deepEqual(changes, [{ agent:agentId, hidden: ['acu-runtime/claude-fable-5-1','acu-runtime/gemini-4-carbon','acu-runtime/gemini-3.8-flash','acu-runtime/claude-fable-5','acu-runtime/claude-opus-4-8'] }]);
      assert.deepEqual(errors, []);
      await page.close();
    }
    console.log('PASS: GPT-6.2 recommendations, explicit model selection and preserved existing setup for four Agents across both themes and widths.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
