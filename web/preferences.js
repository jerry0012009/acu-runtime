globalThis.createACUPreferences = ({ app, ui, request, navigation, escapeHTML: esc }) => {
  const state = { identity: '', generation: 0, data: {}, errors: {}, loaded: 0, busy: false, query: '', category: '', controller: null, revoked: false, refreshTimer: null };
  const titles = { profile: '偏好概览', library: '偏好记录', activity: '近期摘要变化', settings: '学习与参考状态' };
  const date = value => value && Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString('zh-CN') : '时间未提供';
  const note = text => `<p class="preference-note" role="status">${esc(text)}</p>`;
  function clear() {
    state.controller?.abort(); clearTimeout(state.refreshTimer); state.generation++; state.data = {}; state.errors = {}; state.loaded = 0; state.busy = false; state.query = ''; state.category = ''; state.revoked = false;
  }
  function syncAccount() {
    const identity = ui.status?.account?.loggedIn ? ui.status.account.identity : '';
    if (identity === state.identity) return;
    clear(); state.identity = identity;
    if (ui.page === 'preferences') { render(); load(); }
  }
  async function load() {
    if (!state.identity || state.revoked || state.busy || Date.now() - state.loaded < 60000) return;
    const generation = state.generation;
    const controller = new AbortController(); state.controller = controller; state.busy = true; state.errors = {}; update();
    await Promise.all(Object.keys(titles).map(async name => {
      try {
        const data = await request(`/runtime/preferences/${name}`, { signal: controller.signal });
        if (generation !== state.generation) return;
        if ((name === 'library' && (data.version !== 1 || !Array.isArray(data.records))) || (name === 'activity' && (data.version !== 1 || !Array.isArray(data.events)))) throw new Error('偏好接口版本或数据格式暂不兼容。');
        state.data[name] = data;
      } catch (error) {
        if (generation !== state.generation || error.name === 'AbortError') return;
        if (error.status === 401) {
          clear(); state.revoked = true; state.errors.session = '登录会话已失效或无权限，请重新登录。'; update(); return;
        }
        state.errors[name] = error.status === 404 ? '当前服务暂不支持此视图。' : error.message;
      }
    }));
    if (generation === state.generation) { state.busy = false; state.loaded = Date.now(); update(); state.refreshTimer = setTimeout(update, 60010); }
  }
  function sourceList(ray) {
    const records = state.data.library?.records || [];
    const sources = (ray.sourceIds || []).map(id => records.find(record => record.id === id));
    return `<details class="preference-sources"><summary>偏好记录</summary><p class="preference-muted">保存的理解，不是用户原话。</p>${state.errors.library ? note(state.errors.library) : sources.length ? sources.map(record => record?.summary ? `<p>${esc(record.summary)}</p>` : note('此条偏好记录暂不可用。')).join('') : note('暂无可读的偏好记录。')}</details>`;
  }
  function content() {
    const account = ui.status?.account;
    if (!account?.loggedIn || state.revoked) return `<section class="panel preference-empty"><h2>登录后查看我的偏好</h2>${note(state.errors.session || account?.error || '使用 ACU 官网账号查看个人记录。API Key 连接仍可在工作台使用。')}<button class="button primary" data-action="account-login" ${account?.pending || !ui.status?.desktop ? 'disabled' : ''}>${account?.pending ? '登录中…' : '登录 ACU'}</button><p class="preference-muted">登录和查看页面不会开启学习或自动参考。</p></section>`;
    const profile = state.data.profile, library = state.data.library, activity = state.data.activity;
    const query = state.query.trim().toLocaleLowerCase();
    const match = text => !query || String(text || '').toLocaleLowerCase().includes(query);
    const rays = profile?.profile?.rays || [];
    const filtered = rays.filter(ray => (!state.category || ray.category === state.category) && match([ray.label, ray.summary, ray.scope, ...(ray.sourceIds || []).map(id => library?.records?.find(record => record.id === id)?.summary || '')].join(' ')));
    const categories = [...new Set(rays.map(ray => ray.category).filter(Boolean))];
    const records = (library?.records || []).filter(record => match(record.summary));
    const errors = Object.entries(state.errors).filter(([name]) => name !== 'session').map(([name, error]) => note(`${titles[name]}：${error}${state.data[name] ? ' 下方保留本次登录上次读取的数据。' : ''}`)).join('');
    const settings = state.data.settings;
    const flags = settings ? `<details class="preference-controls"><summary>学习与参考状态（只读）</summary>${[['learningEnabled','偏好学习'],['observerEnabled','观察'],['advisorEnabled','建议'],['injectionEnabled','参考注入']].map(([key,label]) => `<p>${label}：${typeof settings[key] === 'boolean' ? settings[key] ? '已开启' : '已关闭' : '服务未提供'}</p>`).join('')}${settings.globalEnabled === false ? note('全局控制当前关闭。') : ''}<p class="preference-muted">设置状态不代表参考已被实际采纳。</p></details>` : '';
    const ready = !state.busy || Object.keys(state.data).length;
    return `<div class="preference-account"><div><strong>${esc(account.username || 'ACU 账户')}</strong><p>当前官网登录账户 · 与手动连接的 API Key 分别管理</p></div><div class="preference-actions"><button class="button secondary" data-pref-refresh ${state.busy || Date.now()-state.loaded < 60000 ? 'disabled' : ''}>${state.busy ? '读取中…' : '刷新'}</button><button class="text-button" data-action="account-login">切换账号</button><button class="text-button" data-pref-logout>退出账号</button></div></div>${errors}${state.busy ? note('正在读取个人偏好…') : ''}${state.loaded ? `<p class="preference-muted">读取于 ${esc(new Date(state.loaded).toLocaleString('zh-CN'))} · 一分钟内复用本次登录的读取结果</p>` : ''}${flags}
      <div class="preference-search"><label for="preference-search">搜索偏好与近期变化</label><input id="preference-search" type="search" placeholder="搜索内容、领域或适用范围" value="${esc(state.query)}"><div class="preference-filters" role="group" aria-label="偏好领域筛选"><button class="button secondary" data-pref-category="" aria-pressed="${!state.category}">全部领域</button>${categories.map(category => `<button class="button secondary" data-pref-category="${esc(category)}" aria-pressed="${state.category === category}">${esc(category)}</button>`).join('')}</div></div>
      <section aria-labelledby="pref-overview"><h2 id="pref-overview">偏好概览</h2>${profile?.profile ? `<p>${esc(profile.profile.summary || '')}</p><p class="preference-muted">已保存图谱 · ${esc(date(profile.generatedAt))}</p>` : ''}${profile?.status === 'error' ? note('图谱生成失败；如有已保存内容，仍在下方展示。') : ['queued','generating'].includes(profile?.status) ? note('图谱正在生成；此处展示已保存内容。') : ''}${ready && profile && !profile.profile ? note('暂无已保存的偏好图谱。') : ''}
      <div class="preference-rays">${filtered.map(ray => `<article class="panel preference-ray"><div class="preference-ray-heading"><h3>${esc(ray.label || '偏好')}</h3><span>${esc(ray.category || '其他领域')}</span></div><p>${esc(ray.summary || '暂无可读摘要。')}</p>${ray.scope ? `<p class="preference-muted">适用范围：${esc(ray.scope)}</p>` : ''}${ray.bipolar?.origin === 'authorized_initialization' ? note('授权初始化；不代表已学习的偏好。') : ''}${sourceList(ray)}</article>`).join('')}</div>${rays.length && !filtered.length ? note('没有匹配的偏好，可调整搜索或领域筛选。') : ''}
      ${rays.length ? `<details class="preference-graph"><summary>查看领域图谱</summary><p class="preference-muted">图谱按领域整理已保存偏好；排列与大小不代表准确率。</p><div class="preference-map">${categories.map(category => `<button class="button secondary" data-pref-category="${esc(category)}">${esc(category)}</button>`).join('')}</div></details>` : ''}</section>
      <section aria-labelledby="pref-records"><h2 id="pref-records">偏好记录</h2><p class="preference-muted">保存的理解，不是用户原话。部分记录可能没有可展示的摘要。</p>${library ? records.length ? records.map(record => `<article class="panel preference-record"><p>${esc(record.summary)}</p></article>`).join('') : note(query ? '没有匹配的偏好记录。' : '暂无可读的偏好记录。') : ''}</section>
      <section aria-labelledby="pref-activity"><h2 id="pref-activity">近期摘要变化</h2><p class="preference-muted">显示近期窗口中的摘要文字变化，不是完整历史；时间为学习完成时间。</p>${activity?.partial ? note('部分变化读取失败，当前结果不完整。') : ''}${activity?.moreAvailable ? note('已达到近期窗口的读取上限，可能还有其他变化。') : ''}${activity ? activity.events.flatMap(event => (event.changes || []).filter(change => ['created','updated','deleted'].includes(change.kind) && (change.kind !== 'updated' || (change.before?.trim() && change.after?.trim() && change.before.trim() !== change.after.trim())) && (change.after || change.before) && match([change.after,change.before].join(' '))).map(change => `<article class="panel preference-change"><div class="preference-ray-heading"><strong>${({created:'新增摘要',updated:'摘要更新',deleted:'移除记录'})[change.kind]}</strong><span>${event.trigger === 'explicit_user_preference' ? '明确偏好' : event.trigger === 'user_dissatisfaction' ? '反馈学习' : '偏好学习'} · ${esc(date(event.recordedAt))}</span></div><p>${esc(change.kind === 'deleted' ? change.before : change.after)}</p>${change.kind === 'updated' ? `<details><summary>更新前</summary><p>${esc(change.before)}</p></details>` : ''}</article>`)).join('') || note(query ? '没有匹配的近期变化。' : '近期窗口内没有可展示的摘要变化；这不代表没有发生学习。') : ''}</section>`;
  }
  function update() { if (ui.page === 'preferences') render(false); }
  function render(shouldLoad = true) {
    const active = document.activeElement?.id === 'preference-search';
    const caret = active ? document.activeElement.selectionStart : null;
    const opened = [...(app.querySelectorAll?.('details') || [])].map((detail, index) => detail.open ? index : -1);
    app.innerHTML = `${navigation()}<div class="main-shell"><main class="preference-page"><header><div class="eyebrow">个人记录</div><h1>我的偏好</h1><p>查看已保存的理解，核对近期摘要变化。</p></header>${content()}</main></div>`;
    for (const [index, detail] of [...(app.querySelectorAll?.('details') || [])].entries()) if (opened.includes(index)) detail.open = true;
    if (active) { const input = app.querySelector('#preference-search'); input?.focus(); if (caret !== null) input?.setSelectionRange?.(caret, caret); }
    if (shouldLoad) { syncAccount(); load(); }
  }
  function click(event) {
    if (event.target.closest('[data-pref-refresh]')) { load(); return true; }
    const filter = event.target.closest('[data-pref-category]');
    if (filter) { state.category = filter.dataset.prefCategory; update(); return true; }
    if (event.target.closest('[data-pref-logout]')) {
      clear(); state.identity = ''; state.revoked = true; update();
      request('/runtime/account/logout', { method: 'POST' }).then(() => { ui.status.account = {}; state.revoked = false; update(); }).catch(error => { state.errors.session = error.message; update(); });
      return true;
    }
    return false;
  }
  function input(event) { if (event.target.id === 'preference-search') { state.query = event.target.value; update(); } }
  function beginLogin() { clear(); state.revoked = true; update(); }
  return { render, syncAccount, click, input, beginLogin };
};
