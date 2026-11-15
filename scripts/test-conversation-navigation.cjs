const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const web = path.resolve(__dirname, '../web');
(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    for (const width of [390, 1440]) {
      const page = await browser.newPage({ viewport: { width, height: 1000 } });
      const actions = [];
      await page.route('http://conversation.test/**', async route => {
        const request = route.request(), url = new URL(request.url());
        if (url.pathname === '/runtime/status') return route.fulfill({ json: { desktop: true, ready: true, agents: [], account: {}, connection: { choices: [] } } });
        if (url.pathname === '/runtime/desktop/action') { actions.push(request.postDataJSON()); return route.fulfill({ json: { ok: true } }); }
        if (url.pathname.startsWith('/runtime/')) return route.fulfill({ json: {} });
        const file = path.join(web, url.pathname === '/' ? 'index.html' : url.pathname.replace('/ui/', ''));
        if (!fs.existsSync(file)) return route.fulfill({ status: 404 });
        return route.fulfill({ contentType: file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html', body: fs.readFileSync(file, 'utf8').replace('__ACU_TOKEN__', 'fixture') });
      });
      await page.goto('http://conversation.test/');
      const nav = page.getByRole('navigation', { name: '主导航' });
      const parent = nav.getByRole('button', { name: 'ACU对话', exact: true });
      await Promise.all([page.waitForResponse(response => response.url().endsWith('/runtime/desktop/action')), parent.click()]);
      await page.waitForFunction(() => document.querySelector('[data-workbench="playground"]'));
      assert.equal(await nav.getByRole('button', { name: '图片生成', exact: true }).count(), 0);
      assert.equal(await nav.getByRole('button', { name: '操练场', exact: true }).count(), 0);
      const toggle = nav.getByRole('button', { name: '展开图片生成和视频生成' });
      await toggle.focus(); await page.keyboard.press('Enter');
      assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      const children = nav.getByRole('list', { name: 'ACU对话子栏目' });
      await Promise.all([page.waitForResponse(response => response.url().endsWith('/runtime/desktop/action')), children.getByRole('button', { name: '图片生成', exact: true }).click()]);
      await Promise.all([page.waitForResponse(response => response.url().endsWith('/runtime/desktop/action')), children.getByRole('button', { name: '视频生成', exact: true }).click()]);
      assert.deepEqual(actions.map(action => action.page), ['playground', 'image-playground', 'video-playground']);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      await page.screenshot({ path: `/tmp/acu-conversation-navigation-${width}.png`, fullPage: true });
      await toggle.click();
      assert.equal(await toggle.getAttribute('aria-expanded'), 'false');
      assert.equal(await nav.getByRole('button', { name: '图片生成', exact: true }).count(), 0);
      await page.close();
    }
    console.log('PASS: one ACU conversation parent, keyboard-expandable media children, correct actions at desktop/mobile widths');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
