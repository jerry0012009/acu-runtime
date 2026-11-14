globalThis.createACUWorkReview = ({ app, ui, request, navigation, escapeHTML: esc, icon }) => {
  const kinds = { private_advice: '纠偏参考', preference_learning: '偏好变化', image_assistance: '图片优化' };
  const labels = { advice_prepared: '参考已准备', advice_delivered: '已提供参考', advice_failed: '参考未送达', preferences_changed: '已更新', guidance_prepared: '建议已准备', guidance_applied: '已加入提示词' };
  const state = { identity: '', generation: 0, controller: null, data: null, busy: false, error: '', filter: '', loaded: 0, open: new Set() };
  const feedback = new Map();
  function syncAccount() {
    const identity = ui.status?.account?.loggedIn ? ui.status.account.identity : '';
    if (identity === state.identity) return;
    state.controller?.abort(); state.generation++; state.identity = identity; state.data = null; state.busy = false; state.error = ''; state.loaded = 0; state.open.clear(); state.filter = ''; feedback.clear();
    if (ui.page === 'work-review') render();
  }
  async function load(force = false) {
    if (!state.identity || state.busy || (!force && state.loaded)) return;
    const generation = state.generation; const controller = new AbortController(); state.controller = controller;
    state.busy = true; state.error = ''; paint();
    try {
      const data = await request('/runtime/preferences/work-review', {signal: controller.signal});
      if (generation !== state.generation) return;
      if (data.version !== 1 || !Array.isArray(data.items) || !Array.isArray(data.unavailable)) throw new Error('工作回顾数据暂不可用');
      state.data = data; state.loaded = Date.now();
    } catch (error) {
      if (generation !== state.generation || error.name === 'AbortError') return;
      state.error = error.status === 401 ? '请重新登录 ACU' : '暂时无法读取工作回顾';
      state.loaded = Date.now();
      if (error.status === 401) state.data = null;
    } finally { if (generation === state.generation) {state.busy = false; paint();} }
  }
  const date = value => Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleDateString('zh-CN', {month:'long',day:'numeric'}) : '时间未提供';
  const time = value => Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'}) : '';
  function record(item) {
    const changes = (item.changes || []).map(change => `<div class="review-change">${change.before ? `<div><span>更新前</span><p>${esc(change.before)}</p></div>` : ''}${change.after ? `<div><span>${change.kind === 'created' ? '新增记录' : '更新后'}</span><p>${esc(change.after)}</p></div>` : ''}${change.kind === 'deleted' ? '<p>已移除此项记录</p>' : ''}</div>`).join('');
    const sources = (item.sources || []).map(source => `<blockquote><span>${source.scope === 'team' ? '团队经验' : source.scope === 'personal' ? '个人偏好' : '历史参考'}</span><p>${esc(source.quote)}</p></blockquote>`).join('');
    return `<article class="review-item"><div class="review-kind">${icon(item.kind === 'image_assistance' ? 'image' : item.kind === 'preference_learning' ? 'shield' : 'eye')}</div><div class="review-content"><div class="review-meta"><span>${kinds[item.kind]}</span><time>${time(item.occurredAt)}</time><span class="review-state">${esc(labels[item.state] || '已记录')}</span></div><h2>${esc(item.title || kinds[item.kind])}</h2><p class="review-summary">${esc(item.summary)}</p>${item.basis ? `<p class="review-basis">${esc(item.basis)}</p>` : ''}<details data-review-detail="${esc(item.id)}" ${state.open.has(item.id) ? 'open' : ''}><summary>具体内容</summary>${item.detail ? `<p class="review-detail-text">${esc(item.detail)}</p>` : ''}${changes}${sources}${item.kind === 'preference_learning' ? '<button class="text-button" data-page="preferences">管理我的偏好</button>' : ''}${item.requestId ? `<div class="review-request">请求 ${esc(item.requestId)}</div>` : ''}</details></div></article>`;
  }
  function paint() {
    if (ui.page !== 'work-review') return;
    const items = (state.data?.items || []).filter(item => kinds[item.kind] && (!state.filter || item.kind === state.filter));
    let day = '';
    const content = items.map(item => {const current = date(item.occurredAt);const heading = current !== day ? `<h2 class="review-day">${esc(current)}</h2>` : '';day = current;return heading + record(item);}).join('');
    app.innerHTML = `${navigation()}<main class="main work-review" id="work-review"><header class="review-heading"><div><h1>工作回顾</h1>${state.loaded && state.data ? `<span>更新于 ${time(new Date(state.loaded).toISOString())}</span>` : ''}</div><button class="icon-button" data-review-refresh title="刷新工作回顾" aria-label="刷新工作回顾" ${state.busy ? 'disabled' : ''}>${icon('refresh', state.busy ? 'spinning' : '')}</button></header>${!state.identity ? '<div class="review-empty"><h2>登录后查看工作回顾</h2><button class="button primary" data-action="account-login">登录 ACU</button></div>' : `<nav class="review-filters" aria-label="记录类型">${[['','全部'], ...Object.entries(kinds)].map(([key,label]) => `<button data-review-filter="${key}" aria-pressed="${state.filter === key}">${label}</button>`).join('')}</nav>${state.error ? `<div class="review-message" role="alert">${esc(state.error)} <button class="text-button" data-review-refresh>重试</button></div>` : ''}${state.data?.unavailable?.length ? `<p class="review-message" role="status">${state.data.unavailable.map(key => kinds[key] || '部分记录').join('、')}暂未完整读取</p>` : ''}${state.busy && !state.data ? '<p class="review-empty" role="status">正在读取工作回顾…</p>' : content || (!state.error && !state.data?.unavailable?.length ? '<p class="review-empty">暂无工作记录</p>' : '')}${state.data?.moreAvailable ? '<p class="review-window">当前显示近期记录</p>' : ''}`}</main>`;
    app.querySelectorAll('[data-review-detail]').forEach(el => el.addEventListener('toggle', () => {if(el.open)state.open.add(el.dataset.reviewDetail);else state.open.delete(el.dataset.reviewDetail);}));
    for (const item of items.filter(item => item.advisorId)) {
      const el = [...app.querySelectorAll('[data-review-detail]')].find(el => el.dataset.reviewDetail === item.id);
      if (el) el.insertAdjacentHTML('beforeend', `<div class="review-feedback">${[['helpful','有帮助'],['inaccurate','不准确'],['ignored','忽略']].map(([value,label])=>`<button class="text-button" data-review-feedback="${value}" data-advisor="${esc(item.advisorId)}" ${feedback.has(item.advisorId) ? 'disabled' : ''}>${label}</button>`).join('')}<span role="status">${esc(feedback.get(item.advisorId)||'')}</span></div>`);
    }
  }
  function render() { if (!document.getElementById('work-review')) paint(); if (state.identity && !state.loaded && !state.busy) void load(); }
  function click(event) {
    const button = event.target.closest('[data-review-feedback]');
    if (button) {
      const id=button.dataset.advisor, generation=state.generation;
      feedback.set(id,'保存中…');paint();
      void request(`/runtime/preferences/feedback/${encodeURIComponent(id)}`, {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({feedback:button.dataset.reviewFeedback})}).then(()=>{
        if(generation===state.generation){feedback.set(id,'已记录');paint();}
      }).catch(()=>{if(generation===state.generation){feedback.delete(id);state.error='反馈未保存，请重试';paint();}});
      return true;
    }
    const filter = event.target.closest('[data-review-filter]'); if (filter) { state.filter = filter.dataset.reviewFilter; paint(); return true; }
    if (event.target.closest('[data-review-refresh]')) { void load(true); return true; }
    return false;
  }
  return {render, syncAccount, click};
};
