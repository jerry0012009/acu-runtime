const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const workspace = path.resolve(__dirname, '../..');
const { chromium } = createRequire(path.join(workspace, '.build/package.json'))('playwright');
const web = path.resolve(__dirname, '../web');

async function main() {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1280, height: 960 } });
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  let account = { loggedIn: true, username: '隔离测试账号', site: 'https://console.acucompute.com', selected: 9 };
  let failAccount = false, failPrices = false, localLoads = 0;
  const calls = [];
  const prices = [{ model_name: 'acu-auto', display_name: 'ACU Auto' }, { model_name: 'fixture-sol', display_name: '测试 Sol', payable: { model_multiplier: '0.06', price_version: 'test-static-v1', input_cny_per_million: 0.8088, output_cny_per_million: 4.044, cached_input_cny_per_million: 0.04044, cache_write_cny_per_million: 1.011, context_tiers: { thresholdTokens: 272000, longContext: { inputPricePerMillion: 1.6176, outputPricePerMillion: 6.066 } } }, reference: { source_type: 'platform' } }];
  await page.route('http://billing.test/**', async route => {
    const url = new URL(route.request().url());
    calls.push(url.pathname + url.search);
    const json = body => route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) });
    if (url.pathname === '/runtime/status') return json({ ready: true, desktop: false, account, agents: [], connection: { known: true, choices: [] } });
    if (url.pathname === '/runtime/billing/account') {
      if (failAccount) return route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"隔离网络失败"}' });
      return json({ loggedIn: true, site: account.site, username: account.username, balanceQuota: 10000000, cnyPerQuota: 0.00001348, updatedAt: Math.floor(Date.now()/1000), key: { name: '开发 Key', unlimited_quota: true, status: 1, expired_time: -1 }, summary: { user_charge_cny: '0.123456789', exact_charge_count: 2, legacy_charge_count: 1 }, logs: [{ model_name: 'fixture-sol', token_name: '开发 Key', created_at: 1794490000, prompt_tokens: 1000, completion_tokens: 300, chargeCny: '0.00000000123', priceVersion: 'old-price-v1' }, { model_name: 'legacy', token_name: '其他 Key', quota: 1000 }], total: 21, warnings: {} });
    }
    if (url.pathname === '/runtime/billing/prices') return json({ models: prices, updatedAt: 1794490000, warning: failPrices ? '隔离价格刷新失败' : '' });
    if (url.pathname === '/runtime/account/logout') { account = {}; return json({ ok: true }); }
    if (url.pathname === '/kernel/') { localLoads++; return route.fulfill({ contentType: 'text/html', body: '<h1>原有本机用量统计</h1><input aria-label="原有筛选" value="今天">' }); }
    const file = url.pathname === '/' ? path.join(web, 'index.html') : path.join(web, url.pathname.replace('/ui/', ''));
    if (!fs.existsSync(file)) return json({});
    return route.fulfill({ contentType: file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html', body: fs.readFileSync(file,'utf8').replace('__ACU_TOKEN__','fixture-token') });
  });
  try {
    await page.goto('http://billing.test/');
    await page.getByRole('button', { name: '用量与费用', exact: true }).click();
    await page.frameLocator('iframe').getByRole('heading', { name: '原有本机用量统计' }).waitFor();
    await page.getByText('¥134.8', { exact: true }).waitFor();
    const before = localLoads;
    await page.waitForResponse(response => response.url().endsWith('/runtime/status'));
    assert.equal(localLoads, before, 'status polling must preserve local iframe');
    await page.getByRole('button', { name: 'ACU 账户', exact: true }).click();
    await page.getByText('未设额度上限', { exact: true }).waitFor();
    assert(await page.getByText('实际扣费 · old-price-v1', { exact: true }).isVisible());
    assert(await page.getByText('¥0.00000000123', { exact: true }).isVisible());
    await page.getByRole('combobox').selectOption('30');
    await page.getByText('最近 30 天云端消费', { exact: true }).waitFor();
    await page.getByRole('button', { name: '下一页' }).click();
    await page.getByText(/第 2 页/).waitFor();
    assert(calls.some(call => call.includes('days=30&page=2')));
    failAccount = true;
    await page.getByRole('button', { name: '刷新', exact: true }).click();
    await page.getByText(/刷新失败：隔离网络失败/).waitFor();
    assert(await page.getByText('¥134.8', { exact: true }).isVisible());
    await page.screenshot({ path: path.join(workspace,'.build/billing-account-preview.png'), fullPage: true });
    await page.getByRole('button', { name: '模型价格', exact: true }).click();
    await page.getByText('按实际选中模型计费', { exact: true }).waitFor();
    await page.getByRole('searchbox').fill('sol');
    assert.equal(await page.locator('#billing-price-rows tr').count(), 1);
    assert(await page.getByRole('columnheader', {name:'倍率',exact:true}).isVisible());
    assert(await page.getByText('0.06×', {exact:true}).isVisible());
    await page.getByText('计费详情', { exact: true }).click();
    await page.getByText(/272,000/).waitFor();
    failPrices = true;
    await page.getByRole('button', { name: '刷新价格' }).click();
    await page.getByText(/价格刷新失败/).waitFor();
    assert.equal(await page.locator('#billing-price-rows tr').count(), 1);
    await page.getByRole('searchbox').fill('没有这个模型');
    await page.getByText('没有匹配的模型价格。', { exact: true }).waitFor();
    await page.getByRole('searchbox').fill('');
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--acu-text').trim() === '#f3f3f1');
    await page.evaluate(() => Promise.all(document.getAnimations().map(animation => animation.finished.catch(() => {}))));
    assert.equal(await page.locator('.billing-tabs .active').evaluate(node => getComputedStyle(node).color), 'rgb(243, 243, 241)');
    await page.screenshot({ path: path.join(workspace,'.build/billing-prices-dark.png'), fullPage: true });
    await page.setViewportSize({ width: 920, height: 700 });
    await page.screenshot({ path: path.join(workspace,'.build/billing-prices-narrow.png'), fullPage: true });
    await page.getByRole('button', { name: 'ACU 账户', exact: true }).click();
    await page.getByRole('button', { name: '退出账号' }).click();
    await page.getByRole('heading', { name: '连接 ACU 账户' }).waitFor();
    assert.equal(await page.getByText('¥134.8', { exact: true }).count(), 0);
    await page.getByRole('button', { name: '模型价格', exact: true }).click();
    await page.getByText('按实际选中模型计费', { exact: true }).waitFor();
    assert.deepEqual(errors, []);
    console.log('PASS: three tabs, preserved local statistics, cloud historical charges, pagination, search, tiered prices, failure recovery, logout isolation, light/dark and narrow layouts.');
  } finally { await browser.close(); }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
