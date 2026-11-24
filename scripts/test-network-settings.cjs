const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);

const web = path.resolve(__dirname, '../web');
const best = 'https://acu.best';
const compute = 'https://console.acucompute.com';

async function checkSettings(browser, width, colorScheme) {
  const page = await browser.newPage({ viewport: { width, height: 950 }, colorScheme });
  const errors = [];
  const writes = [];
  let networkReads = 0;
  let view = { mode: 'auto', route: { base: compute, proxy: 'direct' }, checked: true, managed: true };
  let reject = 0;
  page.on('pageerror', error => errors.push(error.message));
  await page.route('http://network-settings.test/**', async route => {
    const { pathname } = new URL(route.request().url());
    if (pathname === '/runtime/status') return route.fulfill({ json: { desktop: true, ready: true, agents: [] } });
    if (pathname === '/runtime/network') {
      if (route.request().method() === 'POST') {
        const input = route.request().postDataJSON();
        writes.push(input);
        if (reject) return route.fulfill({ status: reject, json: { error: reject === 409 ? '正在检查线路，请稍后重试' : '未测到可用线路，已保留当前连接' } });
        view = { ...view, mode: input.mode, route: input.route || { base: compute, proxy: 'direct' }, message: '连接设置已保存' };
        return route.fulfill({ json: view });
      }
      return route.fulfill({ json: ++networkReads === 1 ? { checking: true, route: { base: '', proxy: '' }, managed: false } : view });
    }
    if (pathname.startsWith('/runtime/')) return route.fulfill({ json: {} });
    const file = path.join(web, pathname === '/' ? 'index.html' : pathname.replace('/ui/', ''));
    if (!fs.existsSync(file) || !fs.statSync(file).isFile()) return route.fulfill({ status: 404 });
    const type = file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html';
    return route.fulfill({ contentType: type, body: fs.readFileSync(file, 'utf8').replace('__ACU_TOKEN__', 'fixture') });
  });
  const settings = page.getByRole('region', { name: '网络', exact: true });
  const button = name => settings.getByRole('button', { name, exact: true });
  const mode = settings.getByLabel('连接方式', { exact: true });
  const endpoint = settings.getByLabel('入口', { exact: true });
  const proxy = settings.getByLabel('代理方式', { exact: true });
  const noOverflow = () => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth);
  const networkRoute = (domain, path) => settings.getByText(`${domain} · ${path}`, { exact: true });
  try {
    await page.goto('http://network-settings.test/');
    assert.equal(await settings.count(), 0);
    assert.equal(networkReads, 0);
    await page.getByRole('button', { name: '设置', exact: true }).click();
    await settings.getByText('正在比较可用线路…', { exact: true }).waitFor();
    await networkRoute('console.acucompute.com', '直连').waitFor();
    assert.equal(await settings.getByRole('combobox').count(), 0);
    assert.deepEqual(writes, []);

    reject = 400;
    await button('重新测速').click();
    await settings.getByText('未测到可用线路，已保留当前连接', { exact: true }).waitFor();
    assert.equal(await networkRoute('console.acucompute.com', '直连').count(), 1);
    reject = 409;
    await button('重新测速').click();
    await settings.getByText('正在检查线路，请稍后重试', { exact: true }).waitFor();
    assert.equal(await networkRoute('console.acucompute.com', '直连').count(), 1);
    await settings.getByText('正在检查线路，请稍后重试', { exact: true }).waitFor({ state: 'hidden' });
    reject = 0;

    await button('连接选项').click();
    assert.equal(await mode.inputValue(), 'auto');
    await mode.selectOption('manual');
    await endpoint.selectOption(best);
    await proxy.selectOption('');
    assert(await noOverflow());
    await button('取消').click();
    assert.deepEqual(writes, [{ mode: 'auto' }, { mode: 'auto' }]);

    await button('连接选项').click();
    await mode.selectOption('manual');
    await endpoint.selectOption(best);
    await proxy.selectOption('');
    await button('保存').click();
    await networkRoute('acu.best', '跟随全局网络').waitFor();
    assert.equal(await button('重新测速').count(), 0);
    assert.deepEqual(writes.at(-1), { mode: 'manual', route: { base: best, proxy: '' } });

    await button('连接选项').click();
    assert.equal(await mode.inputValue(), 'manual');
    await mode.selectOption('auto');
    await button('保存').click();
    await networkRoute('console.acucompute.com', '直连').waitFor();
    assert.deepEqual(writes.at(-1), { mode: 'auto' });
    assert(await noOverflow());
    assert.deepEqual(errors, []);
  } finally {
    await page.close();
  }
}

(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.ACU_TEST_CHROMIUM });
  try {
    await checkSettings(browser, 390, 'dark');
    await checkSettings(browser, 1440, 'light');
  } finally {
    await browser.close();
  }
  console.log('PASS: network settings startup progress, recheck failure, manual overrides, automatic restore, and desktop/mobile layout');
})().catch(error => { console.error(error); process.exitCode = 1; });
