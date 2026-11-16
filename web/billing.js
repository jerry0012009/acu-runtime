globalThis.createACUBilling = ({ app, ui, request, navigation, escapeHTML: esc, icon }) => {
  const state = { tab: 'local', days: 7, page: 1, account: null, prices: null, accountError: '', pricesError: '', accountBusy: false, pricesBusy: false, query: '', sort: 'capability_desc', identity: '', generation: 0, localQuery: '', localReady: false, accountAttempt: 0 };
  const money = value => value == null || value === '' || !Number.isFinite(Number(value)) ? '—' : '¥' + new Intl.NumberFormat('zh-CN', Number(value) !== 0 && Math.abs(Number(value)) < 0.000001 ? { maximumSignificantDigits: 4 } : { maximumFractionDigits: 6 }).format(Number(value));
  const priceMoney = value => '¥' + Number(value).toLocaleString('zh-CN', { maximumSignificantDigits: 10 });
  const date = value => value ? new Date(value * 1000).toLocaleString('zh-CN') : '尚未刷新';
  const quota = value => value == null || !state.account?.cnyPerQuota ? '—' : money(value * state.account.cnyPerQuota);
  const overviewMoney = value => value == null || value === '' || !Number.isFinite(Number(value)) ? '—' : Number(value) > 0 && Number(value) < .01 ? '< ¥0.01' : '¥' + Number(value).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  let modelOrder = 'charge', showAllModels = false;
  let renderedPriceRows = '';
  const note = message => `<p class="billing-note" role="status">${esc(message)}</p>`;
  const button = (label, attributes = '') => `<button class="button secondary" ${attributes}>${label}</button>`;
  const identity = () => `${ui.status?.account?.identity || ui.status?.account?.site || ''}/${ui.status?.account?.username || ''}/${!!ui.status?.account?.loggedIn}/${ui.status?.account?.selected || 0}`;
  const site = () => state.account?.site || ui.status?.account?.site || 'https://console.acucompute.com';
  const links = () => `<a class="button secondary" href="${esc(site())}/wallet" target="_blank" rel="noopener noreferrer">前往官网充值</a><a class="text-button" href="${esc(site())}/usage-logs" target="_blank" rel="noopener noreferrer">云端消费详情 ↗</a>`;

  function syncAccount() {
    const current = identity();
    if (current === state.identity) {
      if (ui.page === 'usage' && !document.hidden && !state.accountBusy && Date.now() - state.accountAttempt > 60000) loadAccount();
      return;
    }
    state.identity = current;
    state.generation++;
    state.account = null;
    state.accountError = '';
    state.accountBusy = false;
    state.page = 1;
    modelOrder = 'charge'; showAllModels = false;
    if (ui.page === 'usage') { render(); loadAccount(); }
  }

  async function loadAccount() {
    if (state.accountBusy || !ui.status?.account?.loggedIn) return;
    const generation = state.generation;
    const days = state.days, page = state.page;
    state.accountBusy = true;
    state.accountAttempt = Date.now();
    update('account');
    try {
      const result = await request(`/runtime/billing/account?days=${days}&page=${page}`);
      if (generation !== state.generation || days !== state.days || page !== state.page) return;
      const previous = state.account;
      if (previous && previous.days === days && previous.page === page) {
        if (result.warnings?.balance) result.balanceQuota = previous.balanceQuota;
        if (result.warnings?.currency) result.cnyPerQuota = previous.cnyPerQuota;
        if (result.warnings?.overview) result.overview = previous.overview;
        if (result.warnings?.summary) result.summary = previous.summary;
        if (result.warnings?.logs) { result.logs = previous.logs; result.total = previous.total; }
        if (Object.keys(result.warnings || {}).length) result.updatedAt = previous.updatedAt;
      }
      state.account = result;
      state.accountError = '';
    } catch (error) { if (generation === state.generation) state.accountError = error.message; }
    finally { if (generation === state.generation) { state.accountBusy = false; update('account'); } }
  }

  async function loadPrices(force = false) {
    if (state.pricesBusy) return;
    state.pricesBusy = true;
    state.pricesError = '';
    update();
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    try {
      if (!state.prices) {
        const cached = await request('/runtime/billing/prices?cached=1', { signal: controller.signal });
        if (Array.isArray(cached.models) && cached.models.length) { state.prices = cached; update('prices'); }
      }
      const result = await request('/runtime/billing/prices' + (force ? '?refresh=1' : ''), { signal: controller.signal });
      if (Array.isArray(result.models) && result.models.length) state.prices = result;
      state.pricesError = result.warning || (!result.models?.length ? '暂时没有可用的价格数据，请稍后重试。' : '');
    } catch (error) { state.pricesError = error.name === 'AbortError' ? '获取价格超时，请稍后重试。' : error.message; }
    finally { clearTimeout(timeout); state.pricesBusy = false; update('prices'); }
  }

  function summary() {
    const account = ui.status?.account;
    if (!account?.loggedIn) return `<span>${account?.restoring ? '正在恢复 ACU 登录…' : '登录后查看 ACU 账户余额'}</span>${button(account?.restoring ? '重新登录' : '登录 ACU', 'data-action="account-login"')}`;
    return `<span>ACU 账户余额 <strong>${quota(state.account?.balanceQuota)}</strong> · ${esc(account.username || '')}${state.account ? ` · ${esc(date(state.account.updatedAt))}` : ''}${state.accountError ? ' · 刷新失败' : ''}</span><button class="text-button" data-billing-tab="account">查看账户 →</button>`;
  }

  function accountPage() {
    const login = ui.status?.account;
    if (!login?.loggedIn) return `<section class="panel billing-empty"><h2>${login?.restoring ? '正在恢复登录' : '登录后查看余额与消费'}</h2>${login?.error ? note(login.error) : ''}${button(login?.restoring ? '手动登录' : '登录 ACU', 'data-action="account-login"')}</section>`;
    const data = state.account;
    const summary = data?.summary;
    const rows = data?.logs || [];
    const warnings = Object.values(data?.warnings || {});
    const overview = data?.overview;
    const comparison = overview?.comparison;
    const comparable = comparison?.savedPercent != null && Number.isFinite(Number(comparison.savedPercent)) && Number(comparison.referenceCny) > 0;
    const saved = Number(comparison?.savedCny), premium = saved < 0;
    const basis = comparison?.basis === 'official' ? '官方价' : comparison?.basis === 'openrouter' ? 'OpenRouter 参考价' : '平台参考价';
    const fullComparison = comparable && comparison.count === overview.count;
    const comparisonTitle = comparable ? `${fullComparison ? '' : '可比用量 · ' }较${basis}${premium ? '多付' : '节省'}` : '费用对比';
    const percent = comparable ? Math.abs(Number(comparison.savedPercent)).toLocaleString('zh-CN', { maximumFractionDigits: 1 }) : '';
    const range = `<select aria-label="消费时段" data-billing-days ${state.accountBusy ? 'disabled' : ''}>${[1,7,30].map(days => `<option value="${days}" ${days === state.days ? 'selected' : ''}>最近 ${days === 1 ? '24 小时' : days + ' 天'}</option>`).join('')}</select>`;
    return `<div class="billing-account" aria-label="ACU 账户概览"><div class="billing-account-heading"><div><span class="billing-eyebrow">ACU 账户</span><h2>${esc(login.username || 'ACU 账户')}</h2><p>账户全部 Key</p></div><div class="billing-account-actions"><a class="button primary" href="${esc(site())}/wallet" target="_blank" rel="noopener noreferrer">充值 ${icon?.('arrow') || ''}</a><a class="text-button" href="${esc(site())}/usage-logs" target="_blank" rel="noopener noreferrer">消费明细 ${icon?.('link') || ''}</a><details class="billing-account-menu"><summary aria-label="账户菜单" title="账户菜单">${icon?.('settings') || '账户'}</summary><div>${button('退出账号', 'data-billing-logout')}</div></details></div></div>
      <div class="billing-account-period">${range}<button class="billing-icon-button" title="刷新账户" aria-label="刷新账户" data-billing-refresh="account" ${state.accountBusy ? 'disabled' : ''}>${icon?.('refresh') || '刷新'}</button><span>${state.accountBusy ? '更新中…' : data ? esc(date(data.updatedAt)) + ' 更新' : ''}</span></div>
      ${state.accountError ? note('刷新失败：' + state.accountError + (data ? '。下方保留上次数据。' : '')) : ''}
      ${warnings.length ? note('部分数据暂不可用：' + [...new Set(warnings)].join('；')) : ''}
      ${data && data.loggedIn === false ? note(data.message) : ''}
      <div class="billing-account-metrics"><section><span>账户余额</span><strong title="${esc(quota(data?.balanceQuota))}">${overviewMoney(data?.balanceQuota != null && data?.cnyPerQuota ? data.balanceQuota * data.cnyPerQuota : null)}</strong><small>当前可用</small></section><section><span>最近 ${state.days === 1 ? '24 小时' : state.days + ' 天'}消费</span><strong>${overviewMoney(overview?.chargeCny ?? summary?.user_charge_cny)}</strong><small>${overview ? Number(overview.count).toLocaleString('zh-CN') + ' 笔消费' : '—'}</small></section><section class="billing-saving ${comparable && !premium ? 'is-saving' : ''}"><span>${esc(comparisonTitle)}</span><div class="billing-saving-value"><strong>${comparable ? overviewMoney(Math.abs(saved)) : '—'}</strong>${comparable ? `<span class="billing-saving-percent">${premium ? '+' : '−'}${esc(percent)}%</span>` : ''}</div>${comparable ? `<details class="billing-comparison"><summary>查看对照</summary><dl><div><dt>${basis}同用量费用</dt><dd>${overviewMoney(comparison.referenceCny)}</dd></div><div><dt>ACU 实际扣费</dt><dd>${overviewMoney(comparison.chargeCny)}</dd></div><div><dt>可比记录</dt><dd>${Number(comparison.count).toLocaleString('zh-CN')} / ${Number(overview.count).toLocaleString('zh-CN')}</dd></div></dl></details>` : '<small>暂无可比账单</small>'}</section></div>
      ${modelRanking(overview)}
      <section class="billing-account-history"><div class="billing-toolbar"><div><h2>消费记录</h2></div><div class="billing-actions">${summary?.legacy_charge_count ? `<span class="billing-muted">含 ${Number(summary.legacy_charge_count).toLocaleString('zh-CN')} 笔历史换算</span>` : ''}</div></div>
      ${state.accountBusy && !data ? note('正在读取云端账户与消费记录…') : ''}
      <div class="billing-table-scroll"><table><thead><tr><th>时间</th><th>模型 / Key</th><th>输入 / 输出 tokens</th><th>扣费</th></tr></thead><tbody>${rows.map(row => `<tr><td>${esc(date(row.created_at))}</td><td><strong>${esc(row.model_name)}</strong><small>${esc(row.token_name || '—')}</small></td><td>${Number(row.prompt_tokens || 0).toLocaleString()} / ${Number(row.completion_tokens || 0).toLocaleString()}</td><td><strong>${row.chargeCny != null ? money(row.chargeCny) : quota(row.quota)}</strong><small>${row.chargeCny != null ? '实际扣费' : '历史额度换算'}${row.priceVersion ? ' · ' + esc(row.priceVersion) : ''}</small></td></tr>`).join('')}</tbody></table></div>
      ${!rows.length && data && !data.warnings?.logs ? note('此范围暂无消费记录。') : ''}
      <div class="billing-toolbar billing-account-pagination"><span>第 ${state.page} 页${data?.total != null ? ` · 共 ${data.total} 条` : ''}</span><div class="billing-actions">${button('上一页', `data-billing-page="-1" ${state.page <= 1 || state.accountBusy ? 'disabled' : ''}`)}${button('下一页', `data-billing-page="1" ${!data || state.page*20 >= data.total || state.accountBusy ? 'disabled' : ''}`)}</div></div></section></div>`;
  }

  function modelRanking(overview) {
    const rows = Array.isArray(overview?.models) ? overview.models.filter(row => typeof row.model === 'string' && Number.isFinite(Number(row.chargeCny)) && Number(row.chargeCny) >= 0 && Number.isFinite(Number(row.count)) && row.count >= 0) : [];
    rows.sort((a,b) => (modelOrder === 'count' ? b.count - a.count : Number(b.chargeCny) - Number(a.chargeCny)) || a.model.localeCompare(b.model));
    const visible = showAllModels ? rows : rows.slice(0,5);
    const total = modelOrder === 'count' ? Number(overview?.count) : Number(overview?.chargeCny);
    return `<section class="billing-model-ranking" aria-label="主要使用模型"><div class="billing-toolbar"><h2>主要使用模型</h2><div class="billing-rank-modes" role="group" aria-label="模型排行排序"><button data-billing-rank="charge" aria-pressed="${modelOrder === 'charge'}">按消费</button><button data-billing-rank="count" aria-pressed="${modelOrder === 'count'}">按次数</button></div></div>${!rows.length ? note(state.accountBusy ? '正在读取模型用量…' : overview ? '此时段暂无模型消费。' : '模型用量暂不可用。') : `<div class="billing-rank-head"><span>模型</span><span>消费</span><span>次数</span><span>占比</span></div><ol>${visible.map((row,index) => {const share = total > 0 ? Math.max(0,Math.min(100,(modelOrder === 'count' ? row.count : Number(row.chargeCny)) / total * 100)) : 0;return `<li><span class="billing-rank-index">${String(index+1).padStart(2,'0')}</span><div class="billing-rank-model"><strong title="${esc(row.model)}">${esc(row.model)}</strong><div class="billing-rank-track"><span style="width:${share}%"></span></div></div><span>${overviewMoney(row.chargeCny)}</span><span class="billing-rank-count">${Number(row.count).toLocaleString('zh-CN')}</span><span class="billing-rank-share">${share.toLocaleString('zh-CN',{maximumFractionDigits:1})}%</span></li>`;}).join('')}</ol>${rows.length > 5 ? `<button class="text-button billing-model-more" data-billing-models>${showAllModels ? '收起' : '查看全部 ' + rows.length + ' 个模型'} ${icon?.('chevron') || ''}</button>` : ''}`}</section>`;
  }

  function priceRows() {
    const search = state.query.toLowerCase();
    const models = (state.prices?.models || []).filter(model => `${model.model_name} ${model.display_name || ''}`.toLowerCase().includes(search));
    const outputPrice = model => {
      const value = model.payable?.output_cny_per_million;
      return value != null && value !== '' && Number.isFinite(Number(value)) && Number(value) >= 0 ? Number(value) : null;
    };
    const capabilityScore = model => {
      const value = model.capability_benchmark?.score;
      return typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
    };
    models.sort((a,b) => {
      if (state.sort === 'capability_desc') {
        const left = capabilityScore(a), right = capabilityScore(b);
        if (left === null && right !== null) return 1;
        if (right === null && left !== null) return -1;
        if (left !== null && right !== null && left !== right) return right - left;
      } else if (state.sort !== 'name') {
        const left = outputPrice(a), right = outputPrice(b);
        if (left === null && right !== null) return 1;
        if (right === null && left !== null) return -1;
        if (left !== null && right !== null && left !== right) return state.sort === 'output_desc' ? right - left : left - right;
      }
      return a.model_name.localeCompare(b.model_name);
    });
    return models.map(model => {
      const payable = model.payable || {};
      const tiers = payable.context_tiers;
      const dynamic = model.model_name === 'acu-auto';
      const comparison = globalThis.acuPriceComparison(model);
      const multiplier = comparison.standard;
      const multiple = value => value === undefined ? '—' : Number(value).toLocaleString('zh-CN', { maximumSignificantDigits: 4 }) + '×';
      const comparisonReference = comparison.reference;
      let comparisonSource = '平台历史参考，非已核验官方价';
      if (comparisonReference?.source_type === 'official') {
        comparisonSource = '官方参考';
        if (model.model_name.startsWith('gpt-')) comparisonSource = model.model_name.endsWith('-fast') ? '官方 Fast 档参考 · 30 分钟缓存写入' : '官方 Standard 档参考 · 30 分钟缓存写入';
      }
      else if (comparisonReference?.source_type === 'public') comparisonSource = '厂商公开价参考（models.dev）';
      const comparisonDetails = comparisonReference ? `<details class="billing-price-reference"><summary>价格对照</summary><dl><div><dt>输入参考价</dt><dd>${money(comparisonReference.input_cny_per_million)}</dd></div><div><dt>输出参考价</dt><dd>${money(comparisonReference.output_cny_per_million)}</dd></div><div><dt>输入 / 输出倍率</dt><dd>${multiple(comparison.input)} / ${multiple(comparison.output)}</dd></div>${comparison.long !== undefined ? `<div><dt>长上下文倍率</dt><dd>${multiple(comparison.long)}</dd></div>` : ''}</dl><p>人民币 / 百万 tokens · 目录汇率 USD/CNY ${esc(comparisonReference.fx_cny_per_usd)}<br>${esc(comparisonSource)} · ${esc(comparisonReference.observed_at || '')}${comparisonReference.source_type === 'public' ? '<br><a href="https://models.dev" target="_blank" rel="noopener noreferrer">查看公开参考来源</a>' : ''}${comparisonReference.source_type === 'official' ? '<br><a href="https://developers.openai.com/api/docs/pricing" target="_blank" rel="noopener noreferrer">查看官方定价</a>' : ''}</p></details>` : '';
      const benchmark = model.capability_benchmark;
      const score = capabilityScore(model);
      const scoreText = value => Number(value).toLocaleString('zh-CN', { maximumFractionDigits: 1 });
      const source = benchmark?.source_url;
      const sourceLink = typeof source === 'string' && /^https:\/\/artificialanalysis\.ai\/models\/[a-z0-9-]+$/.test(source) ? `<a href="${esc(source)}" target="_blank" rel="noopener noreferrer">查看来源</a>` : '';
      const reference = score !== null ? `<details class="billing-benchmark"><summary>能力参考 · ${esc(scoreText(score))}</summary><p>${esc(benchmark.benchmark)}</p><ul>${(benchmark.variants || []).filter(item => typeof item.score === 'number' && Number.isFinite(item.score)).map(item => `<li><span>${esc(item.name)}</span><span>${esc(scoreText(item.score))}</span></li>`).join('')}</ul><p>排序取最高公开配置 · ${esc(benchmark.observed_at || '')} ${sourceLink}</p></details>` : state.sort === 'capability_desc' && !dynamic ? '<small>暂无同榜评分</small>' : '';
      const cells = [
        ['output', '输出价格', outputPrice(model), 'outputPricePerMillion'],
        ['input', '输入价格', payable.input_cny_per_million, 'inputPricePerMillion'],
        ['cached', '缓存命中', payable.cached_input_cny_per_million, 'cachedInputPricePerMillion'],
        ['write', '缓存写入', payable.cache_write_cny_per_million, 'cacheWritePricePerMillion'],
      ].map(([key, label, value, tierKey]) => {
        const valid = value != null && value !== '' && Number.isFinite(Number(value)) && Number(value) >= 0;
        const longValue = tiers?.longContext?.[tierKey];
        const hasTier = Number(tiers?.thresholdTokens) > 0 && longValue != null && longValue !== '' && Number.isFinite(Number(longValue)) && Number(longValue) >= 0;
        return `<td data-price="${key}" data-label="${label}"><span class="billing-price-value">${valid ? priceMoney(value) : '—'}</span>${hasTier ? `<small class="billing-price-long" aria-label="超过 ${Number(tiers.thresholdTokens).toLocaleString('zh-CN')} 输入 tokens">${priceMoney(longValue)}</small>` : ''}</td>`;
      }).join('');
      return `<tr><td class="billing-price-model"><strong>${esc(model.display_name || model.model_name)}</strong>${model.display_name && model.display_name !== model.model_name ? `<small>${esc(model.model_name)}</small>` : ''}${dynamic ? '<small>按实际选中模型计费</small>' : ''}${tiers?.thresholdTokens > 0 ? `<small class="billing-context-label">次行：&gt; ${Number(tiers.thresholdTokens).toLocaleString('zh-CN')} 输入 tokens</small>` : ''}<div class="billing-price-meta">${reference}${comparisonDetails}</div></td><td class="billing-price-multiplier" data-label="实付倍率"><span title="人民币实付价 ÷ 同档位公开参考价折合人民币；输入、输出折扣不同时分别列示">${multiplier !== undefined ? multiple(multiplier) : comparison.input !== undefined || comparison.output !== undefined ? `<small>输入 ${multiple(comparison.input)}<br>输出 ${multiple(comparison.output)}</small>` : '—'}</span></td>${cells}</tr>`;
    }).join('');
  }

  function pricesPage() {
    const rows = priceRows();
    renderedPriceRows = rows;
    return `<section class="billing-prices"><div class="billing-toolbar"><div><h2>模型价格</h2><p>人民币 / 百万 tokens · 实付单价</p></div><div class="billing-price-controls"><input type="search" id="billing-price-search" aria-label="搜索模型价格" placeholder="搜索模型名称" value="${esc(state.query)}"><label>排序 <select data-billing-sort><option value="name" ${state.sort === 'name' ? 'selected' : ''}>模型名称</option><option value="output_asc" ${state.sort === 'output_asc' ? 'selected' : ''}>输出价格从低到高</option><option value="output_desc" ${state.sort === 'output_desc' ? 'selected' : ''}>输出价格从高到低</option><option value="capability_desc" ${state.sort === 'capability_desc' ? 'selected' : ''}>模型能力从高到低</option></select></label>${button(state.pricesBusy ? '刷新中…' : '刷新价格', `data-billing-refresh="prices" ${state.pricesBusy ? 'disabled' : ''}`)}</div></div>
      <div id="billing-price-status">${priceStatus()}</div>
      <div class="billing-table-scroll"><table class="billing-price-table" aria-label="模型价格"><thead><tr><th scope="col">模型</th><th scope="col">实付倍率</th><th scope="col">输出价格</th><th scope="col">输入价格</th><th scope="col">缓存命中</th><th scope="col">缓存写入</th></tr></thead><tbody id="billing-price-rows">${rows}</tbody></table></div><p class="billing-note" id="billing-price-empty" ${rows || !state.prices ? 'hidden' : ''}>没有匹配的模型价格。</p><p class="billing-note">实付倍率 = 人民币实付价 ÷ 公开参考价折合人民币（输入 / 输出），不是渠道成本倍率。无可靠对照时不显示倍率。<br>— 表示暂无报价或可比参考 · 更新于 <span id="billing-price-updated">${esc(date(state.prices?.updatedAt))}</span></p></section>`;
  }

  function priceStatus() {
    return `${state.pricesError ? note('价格刷新失败：' + state.pricesError + (state.prices?.updatedAt ? ' 当前显示上次成功获取的价格。' : '')) : ''}${state.pricesBusy ? note(state.prices ? '正在更新价格，当前显示上次成功获取的价格。' : '正在获取模型价格…') : ''}`;
  }

  function updatePriceRows() {
    const rows = priceRows();
    if (rows !== renderedPriceRows) {
      document.getElementById('billing-price-rows').innerHTML = rows;
      renderedPriceRows = rows;
    }
    document.getElementById('billing-price-empty').hidden = !!rows || !state.prices;
  }

  function update(source = '') {
    if (ui.page !== 'usage') return;
    if (source === 'account' && state.tab === 'prices') return;
    if (source === 'prices' && state.tab !== 'prices') return;
    if (state.tab === 'local') {
      const strip = document.getElementById('billing-summary');
      if (strip) strip.innerHTML = summary();
      return;
    }
    const searchFocused = document.activeElement?.id === 'billing-price-search';
    render();
    if (searchFocused) document.getElementById('billing-price-search')?.focus();
  }

  function render() {
    if (ui.page !== 'usage') return;
    // Keep native controls mounted while async requests and status polls finish.
    if (state.tab === 'prices' && document.getElementById('billing-price-rows')) {
      updatePriceRows();
      const refresh = app.querySelector('[data-billing-refresh="prices"]');
      refresh.disabled = state.pricesBusy;
      refresh.textContent = state.pricesBusy ? '刷新中…' : '刷新价格';
      document.getElementById('billing-price-status').innerHTML = priceStatus();
      document.getElementById('billing-price-updated').textContent = date(state.prices?.updatedAt);
      return;
    }
    if (state.tab === 'local' && document.getElementById('billing-summary') && state.localReady === !!ui.status?.ready) { update(); return; }
    state.localReady = !!ui.status?.ready;
    const tabs = [['local','本机用量'],['account','ACU 账户'],['prices','模型价格']];
    app.innerHTML = `${navigation()}<div class="main-shell billing-shell"><header class="topbar"><span>ACU Runtime / 用量与费用</span></header><main class="content billing-content"><div class="page-heading"><div><h1>用量与费用</h1></div></div><nav class="billing-tabs" aria-label="用量与费用子页面">${tabs.map(([tab,label]) => `<button data-billing-tab="${tab}" ${state.tab === tab ? 'aria-current="page" class="active"' : ''}>${label}</button>`).join('')}</nav>${state.tab === 'local' ? `<div class="billing-summary" id="billing-summary">${summary()}</div><p class="billing-note">本机用量 · 预估费用</p>${ui.status?.ready ? `<iframe class="billing-local-frame" title="本机用量统计" src="/kernel/?view=usage&acu-view=usage${state.localQuery}" sandbox="allow-same-origin allow-scripts allow-forms allow-downloads allow-popups"></iframe>` : note('正在连接本机内核…')}` : state.tab === 'account' ? accountPage() : pricesPage()}</main></div>`;
  }

  function open(tab = state.tab, query) {
    ui.page = 'usage'; ui.frame = null;
    if (['local','account','prices'].includes(tab)) state.tab = tab;
    if (query) state.localQuery = '&' + query.toString();
    app.innerHTML = '';
    syncAccount();
    render();
    if (!state.account) loadAccount();
    if (state.tab === 'prices') loadPrices();
  }

  function click(event) {
    const target = event.target;
    const tab = target.closest('[data-billing-tab]');
    if (tab) { open(tab.dataset.billingTab); return true; }
    const refresh = target.closest('[data-billing-refresh]');
    if (refresh) { refresh.dataset.billingRefresh === 'prices' ? loadPrices(true) : loadAccount(); return true; }
    const page = target.closest('[data-billing-page]');
    if (page) { state.page = Math.max(1, state.page + Number(page.dataset.billingPage)); loadAccount(); return true; }
    const rank = target.closest('[data-billing-rank]');
    if (rank) { modelOrder = rank.dataset.billingRank === 'count' ? 'count' : 'charge'; render(); return true; }
    if (target.closest('[data-billing-models]')) { showAllModels = !showAllModels; render(); return true; }
    if (target.closest('[data-billing-logout]')) {
      request('/runtime/account/logout', { method: 'POST' }).then(() => { ui.status.account = {}; syncAccount(); }).catch(error => { state.accountError = error.message; update(); });
      return true;
    }
    return false;
  }

  function change(event) {
    if (event.target.matches?.('[data-billing-days]')) { state.days = Number(event.target.value); state.page = 1; state.account = null; loadAccount(); return true; }
    if (event.target.matches?.('[data-billing-sort]')) { state.sort = event.target.value; render(); return true; }
    return false;
  }
  function input(event) {
    if (event.target.id !== 'billing-price-search') return;
    state.query = event.target.value;
    updatePriceRows();
  }
  return { open, render, syncAccount, click, change, input };
};
