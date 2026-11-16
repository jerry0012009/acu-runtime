const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const workspace = process.env.ACU_TEST_WORKSPACE || path.resolve(__dirname, '../..');
const { chromium } = process.env.PLAYWRIGHT_MODULE ? require(process.env.PLAYWRIGHT_MODULE) : createRequire(path.join(workspace, '.build/package.json'))('playwright');
fs.mkdirSync(path.join(workspace, '.build'), { recursive: true });
const web = path.resolve(__dirname, '../web');

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 960 } });
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  let account = { loggedIn: true, username: '隔离测试账号', site: 'https://console.acucompute.com', selected: 9 };
  let failAccount = false, failPrices = false, localLoads = 0, coldPrices = false, delayPrices = false;
  const calls = [];
  const prices = [{ model_name: 'acu-auto', display_name: 'ACU Auto' }, { model_name: 'fixture-sol', display_name: '测试 Sol', payable: { model_multiplier: '0.06', price_version: 'test-static-v1', input_cny_per_million: 0.8088, output_cny_per_million: 4.044, cached_input_cny_per_million: 0.04044, cache_write_cny_per_million: 1.011, context_tiers: { thresholdTokens: 272000, longContext: { inputPricePerMillion: 1.6176, outputPricePerMillion: 6.066 } } }, reference: { source_type: 'platform' } }];
  prices.push({model_name:'cheap',payable:{output_cny_per_million:'0.25'}},{model_name:'premium',payable:{output_cny_per_million:25}},{model_name:'free',payable:{output_cny_per_million:0}},{model_name:'unknown',payable:{output_cny_per_million:'unavailable'}});
  prices[1].capability_benchmark = { benchmark:'Artificial Analysis Intelligence Index v5.0.1', score:52.5, observed_at:'2026-11-16', source_url:'https://artificialanalysis.ai/models/fixture-sol', variants:[{name:'Fixture Sol (High)',score:52.5},{name:'Fixture Sol (Medium)',score:48}] };
  prices.find(model=>model.model_name==='premium').capability_benchmark = { benchmark:'Artificial Analysis Intelligence Index v5.0.1', score:61.8, observed_at:'2026-11-16', source_url:'javascript:alert(1)', variants:[{name:'Premium <test>',score:61.8}] };
  prices.find(model=>model.model_name==='cheap').capability_benchmark = { score:null };
  await page.route('http://billing.test/**', async route => {
    const url = new URL(route.request().url());
    calls.push(url.pathname + url.search);
    const json = body => route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) });
    if (url.pathname === '/runtime/status') return json({ ready: true, desktop: false, account, agents: [], connection: { known: true, choices: [] } });
    if (url.pathname === '/runtime/billing/account') {
      if (failAccount) return route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"隔离网络失败"}' });
      return json({ loggedIn: true, site: account.site, username: account.username, balanceQuota: 10000000, cnyPerQuota: 0.00001348, updatedAt: Math.floor(Date.now()/1000), key: { name: '开发 Key', unlimited_quota: true, status: 1, expired_time: -1 }, summary: { user_charge_cny: '0.123456789', exact_charge_count: 2, legacy_charge_count: 1 }, logs: [{ model_name: 'fixture-sol', token_name: '开发 Key', created_at: 1794490000, prompt_tokens: 1000, completion_tokens: 300, chargeCny: '0.00000000123', priceVersion: 'old-price-v1' }, { model_name: 'legacy', token_name: '其他 Key', quota: 1000 }], total: 21, warnings: {} });
    }
    if (url.pathname === '/runtime/billing/prices') {
      if (delayPrices && !url.searchParams.has('cached')) await new Promise(resolve => setTimeout(resolve, 1000));
      return json({ models: coldPrices ? null : prices, updatedAt: coldPrices ? 0 : 1794490000, warning: failPrices ? '隔离价格刷新失败' : '' });
    }
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
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    await page.getByRole('searchbox').fill('sol');
    assert.equal(await page.locator('#billing-price-rows tr').count(), 1);
    delayPrices = true;
    await page.getByRole('button', { name: '刷新价格', exact: true }).click();
    assert.equal(await page.getByText(/价格刷新失败/).count(), 0, 'retry clears the previous failure');
    assert.equal(await page.locator('#billing-price-rows tr').count(), 1, 'pending refresh keeps cached rows');
    await page.getByRole('searchbox').fill('fixture');
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    assert.equal(await page.getByRole('searchbox').inputValue(), 'fixture');
    assert(await page.getByRole('searchbox').evaluate(node => node === document.activeElement), 'refresh preserves search focus');
    delayPrices = false;
    assert.deepEqual(await page.locator('.billing-prices th').allTextContents(),['模型','倍率','输出价格','输入价格','缓存命中','缓存写入']);
    assert(await page.getByText('0.06×', {exact:true}).isVisible());
    const solRow = page.locator('#billing-price-rows tr');
    assert((await solRow.locator('[data-price="output"]').innerText()).startsWith('¥4.044'));
    assert((await solRow.locator('[data-price="input"]').innerText()).startsWith('¥0.8088'));
    assert((await solRow.locator('[data-price="cached"]').innerText()).startsWith('¥0.04044'));
    assert((await solRow.locator('[data-price="write"]').innerText()).startsWith('¥1.011'));
    assert((await solRow.locator('[data-price="input"]').innerText()).includes('¥1.6176'));
    assert.equal(await solRow.getByText(/272,000/).count(), 2);
    await page.getByRole('searchbox').fill('');
    assert.equal(await page.locator('[data-billing-sort]').inputValue(), 'output_asc');
    await page.locator('[data-billing-sort]').selectOption('output_asc');
    assert.deepEqual(await page.locator('#billing-price-rows tr td:first-child strong').allTextContents(),['free','cheap','测试 Sol','premium','ACU Auto','unknown']);
    await page.locator('[data-billing-sort]').selectOption('output_desc');
    assert.deepEqual(await page.locator('#billing-price-rows tr td:first-child strong').allTextContents(),['premium','测试 Sol','cheap','free','ACU Auto','unknown']);
    await page.locator('[data-billing-sort]').selectOption('name');
    assert.deepEqual(await page.locator('#billing-price-rows tr td:first-child strong').allTextContents(),['ACU Auto','cheap','测试 Sol','free','premium','unknown']);
    await page.locator('[data-billing-sort]').selectOption('capability_desc');
    assert.deepEqual(await page.locator('#billing-price-rows tr td:first-child strong').allTextContents(),['premium','测试 Sol','ACU Auto','cheap','free','unknown']);
    const benchmarkDetails = page.locator('.billing-benchmark').filter({hasText:'Fixture Sol (High)'});
    assert.equal(await benchmarkDetails.getAttribute('open'),null);
    await benchmarkDetails.locator('summary').click();
    assert(await benchmarkDetails.getByText('Artificial Analysis Intelligence Index v5.0.1',{exact:true}).isVisible());
    assert(await benchmarkDetails.getByText('Fixture Sol (Medium)',{exact:true}).isVisible());
    assert.equal(await benchmarkDetails.getByRole('link',{name:'查看来源'}).getAttribute('href'),'https://artificialanalysis.ai/models/fixture-sol');
    assert.equal(await page.locator('.billing-benchmark a[href^="javascript:"]').count(),0);
    assert.equal(await page.locator('.billing-benchmark test').count(),0);
    await page.getByRole('button', { name: '刷新价格', exact: true }).click();
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    assert.equal(await page.locator('[data-billing-sort]').inputValue(),'capability_desc');
    await page.locator('[data-billing-sort]').selectOption('name');
    await page.getByRole('searchbox').fill('sol');
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
      await page.getByRole('heading', { name: '登录后查看余额与消费' }).waitFor();
    assert.equal(await page.getByText('¥134.8', { exact: true }).count(), 0);
    await page.getByRole('button', { name: '模型价格', exact: true }).click();
    await page.getByText('按实际选中模型计费', { exact: true }).waitFor();
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    coldPrices = true;
    await page.reload();
    await page.getByRole('button', { name: '用量与费用', exact: true }).click();
    await page.getByRole('button', { name: '模型价格', exact: true }).click();
    await page.getByText(/价格刷新失败/).waitFor();
    assert.equal(await page.getByText('没有匹配的模型价格。', { exact: true }).isVisible(), false, 'network failure is not an empty search');
    await page.getByRole('button', { name: 'ACU 账户', exact: true }).click();
    coldPrices = false; failPrices = false;
    await page.getByRole('button', { name: '模型价格', exact: true }).click();
    await page.getByText('按实际选中模型计费', { exact: true }).waitFor();
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    // A restarted UI receives disk cache before a slow upstream refresh completes.
    delayPrices = true;
    await page.reload();
    await page.getByRole('button', { name: '用量与费用', exact: true }).click();
    await page.getByRole('button', { name: '模型价格', exact: true }).click();
    await page.getByText('按实际选中模型计费', { exact: true }).waitFor();
    assert(await page.getByRole('button', { name: '刷新中…', exact: true }).isVisible());
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    prices.push({ model_name: 'fixture-model-with-a-very-long-unbroken-name-for-responsive-pricing', payable: { model_multiplier: 'invalid', input_cny_per_million: -1, output_cny_per_million: 3, cached_input_cny_per_million: 0.0003042, cache_write_cny_per_million: 0, context_tiers: { thresholdTokens: 100000, longContext: { inputPricePerMillion: 2, outputPricePerMillion: 6, cachedInputPricePerMillion: 0.0006084, cacheWritePricePerMillion: 0 } } } });
    await page.getByRole('button', { name: '刷新价格', exact: true }).click();
    await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
    const longRow = page.getByRole('row').filter({ hasText: 'fixture-model-with-a-very-long' });
    assert.equal(await longRow.locator('.billing-price-multiplier').innerText(), '—');
    assert.equal(await longRow.locator('[data-price="input"] .billing-price-value').innerText(), '—');
    assert.equal(await longRow.locator('[data-price="cached"] .billing-price-value').innerText(), '¥0.0003042');
    assert.equal(await longRow.locator('[data-price="write"] .billing-price-value').innerText(), '¥0');
    assert.equal(await longRow.getByText(/100,000/).count(), 4);
    for (const theme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: theme });
      for (const width of [1440, 1280, 920, 600, 390]) {
        await page.setViewportSize({ width, height: 960 });
        const reference = page.locator('.billing-benchmark').filter({hasText:'Fixture Sol (High)'});
        if (await reference.getAttribute('open') === null) await reference.locator('summary').click();
        const detailsBox = await reference.boundingBox();
        const modelBox = await reference.locator('..').boundingBox();
        assert(detailsBox.x + detailsBox.width <= modelBox.x + modelBox.width + 1, `benchmark fits model column at ${width}`);
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `no page overflow at ${width}`);
        assert.equal(await longRow.locator('[data-price="output"]').evaluate(node => getComputedStyle(node).textAlign), width > 1100 ? 'right' : 'left');
        const outputBox = await longRow.locator('[data-price="output"]').boundingBox();
        const inputBox = await longRow.locator('[data-price="input"]').boundingBox();
        assert(outputBox.x + outputBox.width <= inputBox.x + 1, `price columns do not overlap at ${width}`);
        assert(await longRow.locator('[data-price="write"]').isVisible());
        await page.screenshot({ path: path.join(workspace, `.build/billing-prices-${theme}-${width}.png`), fullPage: true });
      }
    }
    if (process.env.ACU_PUBLIC_PRICE_FIXTURE) {
      prices.splice(0, prices.length, ...JSON.parse(fs.readFileSync(process.env.ACU_PUBLIC_PRICE_FIXTURE, 'utf8')).data);
      await page.getByRole('button', { name: '刷新价格', exact: true }).click();
      await page.getByRole('button', { name: '刷新价格', exact: true }).waitFor();
      await page.locator('[data-billing-sort]').selectOption('capability_desc');
      const sorted = await page.locator('#billing-price-rows tr').evaluateAll(rows => rows.map(row=>row.querySelector('strong').textContent));
      assert(sorted[0].includes('Fable 5.2') || sorted[0].includes('claude-fable-5-2'));
      await page.locator('.billing-benchmark summary').first().click();
      for (const width of [1440, 920, 390]) {
        await page.setViewportSize({ width, height: 960 });
        await page.screenshot({ path: path.join(workspace, `.build/billing-public-prices-${width}.png`) });
      }
    }
    assert.deepEqual(errors, []);
    console.log('PASS: complete prices and multipliers, default output sort, zero and missing prices, tiers, search, refresh persistence, historical charges, failure recovery, logout isolation and responsive themes.');
  } finally { await browser.close(); }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
