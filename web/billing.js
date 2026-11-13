globalThis.createACUBilling = ({ app, ui, request, navigation, escapeHTML: esc }) => {
  const state = { tab: 'local', days: 7, page: 1, account: null, prices: null, accountError: '', pricesError: '', accountBusy: false, pricesBusy: false, query: '', sort: 'name', identity: '', generation: 0, localQuery: '', localReady: false, accountAttempt: 0 };
  const money = value => value == null || value === '' || !Number.isFinite(Number(value)) ? '—' : '¥' + new Intl.NumberFormat('zh-CN', Number(value) !== 0 && Math.abs(Number(value)) < 0.000001 ? { maximumSignificantDigits: 4 } : { maximumFractionDigits: 6 }).format(Number(value));
  const date = value => value ? new Date(value * 1000).toLocaleString('zh-CN') : '尚未刷新';
  const quota = value => value == null || !state.account?.cnyPerQuota ? '—' : money(value * state.account.cnyPerQuota);
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
        if (result.warnings?.key) result.key = previous.key;
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
    const key = data?.key;
    const summary = data?.summary;
    const rows = data?.logs || [];
    const warnings = Object.values(data?.warnings || {});
    const range = `<label>消费范围 <select data-billing-days ${state.accountBusy ? 'disabled' : ''}>${[1,7,30].map(days => `<option value="${days}" ${days === state.days ? 'selected' : ''}>最近 ${days === 1 ? '24 小时' : days + ' 天'}</option>`).join('')}</select></label>`;
    return `<div class="billing-toolbar"><div><strong>${esc(login.username || 'ACU 账户')}</strong><p>云端账户全部 Key · 包含其他设备与客户端的消费</p></div><div class="billing-actions">${links()}${button('退出账号', 'data-billing-logout')}</div></div>
      ${state.accountError ? note('刷新失败：' + state.accountError + (data ? '。下方保留上次数据。' : '')) : ''}
      ${warnings.length ? note('部分数据暂不可用：' + [...new Set(warnings)].join('；')) : ''}
      ${data && data.loggedIn === false ? note(data.message) : ''}
      <div class="billing-metrics"><section class="panel"><span>账户余额</span><strong>${quota(data?.balanceQuota)}</strong><small>账户可用额度</small></section><section class="panel"><span>${key ? esc(key.name || '上次连接的 Key') : '上次连接的 Key'}</span><strong>${key?.unlimited_quota ? '未设额度上限' : quota(key?.remain_quota)}</strong><small>${key ? `${key.status === 1 ? '已启用' : '已停用'} · ${key.expired_time > 0 ? '到期 ' + esc(date(key.expired_time)) : '无到期限制'}` : '额度暂不可用'} · 与账户余额分别计算</small></section><section class="panel"><span>最近 ${state.days === 1 ? '24 小时' : state.days + ' 天'}云端消费</span><strong>${money(summary?.user_charge_cny)}</strong><small>${summary ? `${summary.exact_charge_count || 0} 条精确扣费 · ${summary.legacy_charge_count || 0} 条历史额度换算` : '从 ACU 云端消费记录读取'}</small></section></div>
      <section class="panel billing-history"><div class="billing-toolbar"><div><h2>云端消费记录</h2></div><div class="billing-actions">${range}${button(state.accountBusy ? '刷新中…' : '刷新', `data-billing-refresh="account" ${state.accountBusy ? 'disabled' : ''}`)}</div></div>
      ${state.accountBusy && !data ? note('正在读取云端账户与消费记录…') : ''}
      <div class="billing-table-scroll"><table><thead><tr><th>时间</th><th>模型 / Key</th><th>输入 / 输出 tokens</th><th>扣费</th></tr></thead><tbody>${rows.map(row => `<tr><td>${esc(date(row.created_at))}</td><td><strong>${esc(row.model_name)}</strong><small>${esc(row.token_name || '—')}</small></td><td>${Number(row.prompt_tokens || 0).toLocaleString()} / ${Number(row.completion_tokens || 0).toLocaleString()}</td><td><strong>${row.chargeCny != null ? money(row.chargeCny) : quota(row.quota)}</strong><small>${row.chargeCny != null ? '实际扣费' : '历史额度换算'}${row.priceVersion ? ' · ' + esc(row.priceVersion) : ''}</small></td></tr>`).join('')}</tbody></table></div>
      ${!rows.length && data && !data.warnings?.logs ? note('此范围暂无消费记录。') : ''}
      <div class="billing-toolbar"><span>第 ${state.page} 页${data?.total != null ? ` · 共 ${data.total} 条` : ''} · ${data ? esc(date(data.updatedAt)) : '尚未刷新'}</span><div class="billing-actions">${button('上一页', `data-billing-page="-1" ${state.page <= 1 || state.accountBusy ? 'disabled' : ''}`)}${button('下一页', `data-billing-page="1" ${!data || state.page*20 >= data.total || state.accountBusy ? 'disabled' : ''}`)}</div></div></section>`;
  }

  function priceRows() {
    const search = state.query.toLowerCase();
    const models = (state.prices?.models || []).filter(model => `${model.model_name} ${model.display_name || ''}`.toLowerCase().includes(search));
    const outputPrice = model => {
      const value = model.payable?.output_cny_per_million;
      return value != null && value !== '' && Number.isFinite(Number(value)) && Number(value) >= 0 ? Number(value) : null;
    };
    models.sort((a,b) => {
      if (state.sort !== 'name') {
        const left = outputPrice(a), right = outputPrice(b);
        if (left === null && right !== null) return 1;
        if (right === null && left !== null) return -1;
        if (left !== null && right !== null && left !== right) return state.sort === 'output_desc' ? right - left : left - right;
      }
      return a.model_name.localeCompare(b.model_name);
    });
    return models.map(model => {
      const tiers = model.payable?.context_tiers;
      const longOutput = tiers?.longContext?.outputPricePerMillion;
      const tier = tiers && longOutput != null ? `<small>超过 ${Number(tiers.thresholdTokens).toLocaleString()} tokens：${money(longOutput)}</small>` : '';
      return `<tr><td><strong>${esc(model.display_name || model.model_name)}</strong>${model.display_name && model.display_name !== model.model_name ? `<small>${esc(model.model_name)}</small>` : ''}</td><td>${money(outputPrice(model))}${model.model_name === 'acu-auto' ? '<small>按实际选中模型计费</small>' : ''}${tier}</td></tr>`;
    }).join('');
  }

  function pricesPage() {
    const rows = priceRows();
    return `<section class="panel billing-prices"><div class="billing-toolbar"><div><h2>模型价格</h2><p>人民币 / 百万 tokens</p></div>${button(state.pricesBusy ? '刷新中…' : '刷新价格', `data-billing-refresh="prices" ${state.pricesBusy ? 'disabled' : ''}`)}</div><div class="billing-toolbar"><input type="search" id="billing-price-search" aria-label="搜索模型价格" placeholder="搜索模型名称" value="${esc(state.query)}"><label>排序 <select data-billing-sort><option value="name" ${state.sort === 'name' ? 'selected' : ''}>模型名称</option><option value="output_asc" ${state.sort === 'output_asc' ? 'selected' : ''}>输出价格从低到高</option><option value="output_desc" ${state.sort === 'output_desc' ? 'selected' : ''}>输出价格从高到低</option></select></label></div>
      ${state.pricesError ? note('价格刷新失败：' + state.pricesError + (state.prices?.updatedAt ? ' 当前显示上次成功获取的价格。' : '')) : ''}${state.pricesBusy ? note(state.prices ? '正在更新价格，当前显示上次成功获取的价格。' : '正在获取模型价格…') : ''}
      <div class="billing-table-scroll"><table><thead><tr><th>模型</th><th>输出价格</th></tr></thead><tbody id="billing-price-rows">${rows}</tbody></table></div><p class="billing-note" id="billing-price-empty" ${rows || !state.prices ? 'hidden' : ''}>没有匹配的模型价格。</p><p class="billing-note">更新于 ${esc(date(state.prices?.updatedAt))}</p></section>`;
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
    const rows = priceRows();
    document.getElementById('billing-price-rows').innerHTML = rows;
    document.getElementById('billing-price-empty').hidden = !!rows || !state.prices;
  }
  return { open, render, syncAccount, click, change, input };
};
