globalThis.createACUPreferences = ({ app, ui, request, navigation, escapeHTML: esc }) => {
  const state = { identity: '', generation: 0, data: {}, errors: {}, loaded: 0, busy: false, settingsSaving: false, settingsSaved: false, query: '', category: '', selected: '', controller: null, revoked: false, refreshTimer: null, retryDelay: 1500, retries: 0 };
  const settingKeys = ['learningEnabled', 'observerEnabled', 'advisorEnabled', 'injectionEnabled'];
  const graph = globalThis.createACUStarfish?.({ escapeHTML: esc, sourceList, onSelect: id => { state.selected = id; } });
  const preferenceCacheMs = 6 * 60 * 60 * 1000;
  const titles = { profile: '偏好概览', library: '偏好记录', activity: '近期摘要变化', settings: '学习与参考状态', controls: '管理偏好' };
  const date = value => value && Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString('zh-CN') : '时间未提供';
  const note = text => `<p class="preference-note" role="status">${esc(text)}</p>`;
  const drafts = new Map();
  const saving = new Set();
  let zoom=1;
  function clear() {
    state.settingsSaving = false; state.settingsSaved = false;
    drafts.clear(); saving.clear();
    zoom=1; graph?.reset();
    state.controller?.abort(); clearTimeout(state.refreshTimer); state.generation++; state.data = {}; state.errors = {}; state.loaded = 0; state.busy = false; state.query = ''; state.category = ''; state.selected = ''; state.revoked = false; state.retryDelay = 1500; state.retries = 0;
  }
  function syncAccount() {
    const identity = ui.status?.account?.loggedIn ? ui.status.account.identity : '';
    if (identity === state.identity) return;
    clear(); state.identity = identity;
    if (ui.page === 'preferences') { render(); load(); }
  }
  async function load(force = false) {
    if (!state.identity || state.revoked || state.busy || state.settingsSaving || (!force && Date.now() - state.loaded < preferenceCacheMs)) return;
    const generation = state.generation;
    let retry = false;
    const controller = new AbortController(); state.controller = controller; state.busy = true; state.errors = {}; update();
    await Promise.all(Object.keys(titles).map(async name => {
      try {
        const data = await request(`/runtime/preferences/${name}`, { signal: controller.signal, allowProfileStateError: name === 'profile' });
        if (generation !== state.generation) return;
        if ((name === 'library' && (data.version !== 1 || !Array.isArray(data.records))) || (name === 'activity' && (data.version !== 1 || !Array.isArray(data.events)))) throw new Error('偏好接口版本或数据格式暂不兼容。');
        if (name === 'settings' && state.settingsSaving) return;
        state.data[name] = name === 'profile' && !data.profile && ['queued','generating','error'].includes(data.status) && state.data.profile?.profile ? { ...data, profile: state.data.profile.profile, generatedAt: state.data.profile.generatedAt } : data;
      } catch (error) {
        if (generation !== state.generation || error.name === 'AbortError') return;
        if (error.status === 401) {
          clear(); state.revoked = true; state.errors.session = '登录会话已失效或无权限，请重新登录。'; update(); return;
        }
        state.errors[name] = error.status === 404 ? '当前服务暂不支持此视图。' : error.message;
        if (error.status === 503) retry = true;
      }
    }));
    if (generation === state.generation) {
      state.retries = retry ? state.retries + 1 : 0;
      const retrySoon = retry && state.retries <= 3;
      state.busy = false; state.loaded = retrySoon ? 0 : Date.now(); update();
      clearTimeout(state.refreshTimer);
      state.refreshTimer = setTimeout(() => { if (ui.page === 'preferences') load(true); }, retrySoon ? state.retryDelay : preferenceCacheMs + 10);
      state.retryDelay = retry ? Math.min(state.retryDelay * 2, 30000) : 1500;

    }
  }
  function sourceList(ray) {
    const records = state.data.library?.records || [];
    const sources = (ray.sourceIds || []).map(id => records.find(record => record.id === id));
    return `<details class="preference-sources"><summary>相关记录</summary>${state.errors.library ? note(state.errors.library) : sources.length ? sources.map(record => record?.summary ? `<p>${esc(record.summary)}</p>` : note('此条偏好记录暂不可用。')).join('') : note('暂无可读的偏好记录。')}</details>`;
  }
  function understanding(profile) {
    const items = (profile?.profile?.highlights || []).slice(0, 5);
    if (!items.length) return `<section class="preference-understanding"><h2>已记住的偏好</h2><p>${esc(profile?.profile?.summary || '暂无已整理的偏好。')}</p></section>`;
    return `<section class="preference-understanding" aria-label="已记住的偏好"><h2>已记住的偏好</h2><div class="preference-highlights">${items.map(item => `<article><h3>${esc(item.title)}</h3><p>${esc(item.summary)}</p><details><summary>相关记录</summary>${(item.sourceIds || []).map(id => { const record = state.data.library?.records?.find(record => record.id === id); return `<p>${esc(record?.summary || '相关记录暂不可用。')}</p>`; }).join('')}</details></article>`).join('')}</div></section>`;
  }
  function learningControls(settings) {
    if (!settingKeys.every(key => typeof settings?.[key] === 'boolean') || typeof settings.globalEnabled !== 'boolean') return '';
    const disabled = state.settingsSaving || state.busy || !settings.globalEnabled;
    const toggle = (key, label, checked) => `<label class="preference-setting"><span>${label}</span><input type="checkbox" role="switch" aria-label="${label}" data-pref-setting="${key}" ${checked ? 'checked' : ''} ${disabled ? 'disabled' : ''}></label>`;
    const interval = settings.observerInterval ?? settings.effectiveObserverInterval ?? settings.globalObserverInterval;
    return `<section class="preference-learning" aria-labelledby="pref-learning"><div class="preference-learning-heading"><h2 id="pref-learning">学习设置</h2><span role="status" class="preference-muted">${state.settingsSaving ? '保存中…' : state.settingsSaved ? '已保存' : ''}</span></div>${toggle('all', 'Private ACU', settingKeys.some(key => settings[key]))}<div class="preference-setting-grid">${[['learningEnabled','偏好学习'],['observerEnabled','任务观察'],['advisorEnabled','建议'],['injectionEnabled','偏好参考']].map(([key,label]) => toggle(key,label,settings[key])).join('')}</div><div class="preference-interval"><label for="pref-interval">观察间隔</label><input id="pref-interval" type="number" min="1" max="100000" step="1" value="${Number.isInteger(interval) ? interval : ''}" ${disabled ? 'disabled' : ''}><span class="preference-muted">次请求 · ${settings.observerInterval == null ? '默认' : '自定义'}</span>${settings.observerInterval != null ? `<button class="text-button" data-pref-interval-reset ${disabled ? 'disabled' : ''}>恢复默认</button>` : ''}</div>${!settings.globalEnabled ? '<p class="preference-muted">管理员已关闭学习服务</p>' : ''}${state.errors.settingsSave ? note(state.errors.settingsSave) : ''}</section>`;
  }
  async function saveSettings(patch) {
    const settings = state.data.settings;
    if (state.settingsSaving || state.busy || !settings?.globalEnabled || !settingKeys.every(key => typeof settings[key] === 'boolean')) return;
    const generation = state.generation;
    const body = Object.fromEntries(settingKeys.map(key => [key, settings[key]]));
    body.observerInterval = settings.observerInterval ?? null;
    Object.assign(body, patch);
    state.settingsSaving = true; state.settingsSaved = false; delete state.errors.settingsSave; update();
    try {
      const saved = await request('/runtime/preferences/settings', {method:'PUT', body:JSON.stringify(body)});
      if (generation !== state.generation) return;
      if (!settingKeys.every(key => typeof saved[key] === 'boolean') || typeof saved.globalEnabled !== 'boolean') throw new Error('设置保存结果暂不可用，请刷新。');
      state.data.settings = saved; state.settingsSaved = true;
    } catch (error) {
      if (generation !== state.generation) return;
      if (error.status === 401 || error.status === 403) { clear(); state.revoked = true; state.errors.session = '登录已失效，请重新登录。'; update(); }
      else state.errors.settingsSave = error.message;
    } finally { if (generation === state.generation) {state.settingsSaving = false; update();} }
  }
  function change(event) {
    const key = event.target.dataset?.prefSetting;
    if (key) {
      if (key === 'all') saveSettings(Object.fromEntries(settingKeys.map(name => [name,event.target.checked])));
      else if (settingKeys.includes(key)) saveSettings({[key]:event.target.checked});
      return true;
    }
    if (event.target.id === 'pref-interval') {
      const value = Number(event.target.value);
      if (Number.isInteger(value) && value >= 1 && value <= 100000 && value !== (state.data.settings?.observerInterval ?? state.data.settings?.effectiveObserverInterval)) saveSettings({observerInterval:value});
      else update();
      return true;
    }
    return false;
  }
  function content() {
    const account = ui.status?.account;
    if (!account?.loggedIn || state.revoked) return `<section class="panel preference-empty"><h2>登录后查看我的偏好</h2>${state.errors.session || account?.error ? note(state.errors.session || account.error) : ''}<button class="button primary" data-action="account-login" ${account?.pending || !ui.status?.desktop ? 'disabled' : ''}>${account?.pending ? '登录中…' : '登录 ACU'}</button></section>`;
    const profile = state.data.profile, library = state.data.library, activity = state.data.activity;
    const query = state.query.trim().toLocaleLowerCase();
    const match = text => !query || String(text || '').toLocaleLowerCase().includes(query);
    const rays = profile?.profile?.rays || [];
    const filtered = rays.filter(ray => (!state.category || ray.category === state.category) && match([ray.label, ray.summary, ray.scope, ...(ray.sourceIds || []).map(id => library?.records?.find(record => record.id === id)?.summary || '')].join(' ')));
    const categories = [...new Set(rays.map(ray => ray.category).filter(Boolean))];
    const records = (library?.records || []).filter(record => match(record.summary));
    const errors = Object.entries(state.errors).filter(([name]) => name !== 'session' && name !== 'settingsSave').map(([name, error]) => note(`${titles[name]}：${error}`)).join('');
    const settings = state.data.settings;
    const flags = learningControls(settings);
    const ready = !state.busy || Object.keys(state.data).length;
    return `<div class="preference-account"><div><strong>${esc(account.username || 'ACU 账户')}</strong></div><div class="preference-actions"><button class="button secondary" data-pref-refresh ${state.busy ? 'disabled' : ''}>${state.busy ? '读取中…' : '刷新'}</button><button class="text-button" data-action="account-login">切换账号</button><button class="text-button" data-pref-logout>退出账号</button></div></div>${errors}${state.busy ? note('正在读取个人偏好…') : ''}${graph?.render(profile) || ''}${flags}
      <div class="preference-search"><label for="preference-search">搜索偏好与近期变化</label><input id="preference-search" type="search" placeholder="搜索内容、领域或适用范围" value="${esc(state.query)}"><div class="preference-filters" role="group" aria-label="偏好领域筛选"><button class="button secondary" data-pref-category="" aria-pressed="${!state.category}">全部领域</button>${categories.map(category => `<button class="button secondary" data-pref-category="${esc(category)}" aria-pressed="${state.category === category}">${esc(category)}</button>`).join('')}</div></div>
      <section aria-labelledby="pref-overview"><h2 id="pref-overview">偏好概览</h2>${profile?.profile ? `<p>${esc(profile.profile.summary || '')}</p>` : ''}${!profile?.profile && profile?.status === 'error' ? note('偏好加载失败，请重试。') : !profile?.profile && ['queued','generating'].includes(profile?.status) ? note('正在整理偏好…') : ''}${ready && profile && !profile.profile && ['ready','empty'].includes(profile.status) ? note('暂无偏好。') : ''}
      <div class="preference-rays">${filtered.map(ray => `<article class="panel preference-ray"><div class="preference-ray-heading"><h3>${esc(ray.label || '偏好')}</h3><span>${esc(ray.category || '其他领域')}</span></div><p>${esc(ray.summary || '暂无摘要。')}</p>${ray.scope ? `<p class="preference-muted">适用范围：${esc(ray.scope)}</p>` : ''}${ray.bipolar?.origin === 'authorized_initialization' ? note('初始设置') : ''}${sourceList(ray)}</article>`).join('')}</div>${rays.length && !filtered.length ? note('没有匹配的偏好。') : ''}
      </section>
      <section aria-labelledby="pref-records"><h2 id="pref-records">偏好记录</h2>${library ? records.length ? records.map(record => `<article class="panel preference-record"><p>${esc(record.summary)}</p></article>`).join('') : note(query ? '没有匹配的偏好记录。' : '暂无可读的偏好记录。') : ''}</section>
      <section aria-labelledby="pref-activity"><h2 id="pref-activity">近期变化</h2>${activity?.partial ? note('部分更新加载失败，请刷新。') : ''}${activity?.moreAvailable ? '<p class="preference-muted">最近一批更新</p>' : ''}${activity ? activity.events.flatMap(event => (event.changes || []).filter(change => ['created','updated','deleted','rule_updated'].includes(change.kind) && (change.kind !== 'updated' || (change.before?.trim() && change.after?.trim() && change.before.trim() !== change.after.trim())) && (change.after || change.before) && match([change.after,change.before].join(' '))).map(change => `<article class="panel preference-change"><div class="preference-ray-heading"><strong>${({created:'新增摘要',updated:'摘要更新',deleted:'移除记录',rule_updated:'规则修订'})[change.kind]}</strong><span>${esc(date(event.recordedAt))}</span></div><p>${esc(change.kind === 'deleted' ? change.before : change.after)}</p>${change.kind === 'updated' ? `<details><summary>更新前</summary><p>${esc(change.before)}</p></details>` : ''}</article>`)).join('') || note(query ? '没有匹配的近期变化。' : '暂无近期变化。') : ''}</section>`;
  }
  function update() { if (ui.page === 'preferences') render(false); }
  async function changePreference(record, action, patch = {}) {
    if (saving.has(record.id)) return;
    const generation = state.generation;
    saving.add(record.id); update();
    try {
      await request(`/runtime/preferences/controls/${encodeURIComponent(record.id)}`, { method:'PUT', body:JSON.stringify({action,...patch,expected_hash:record.revision}) });
      if (generation !== state.generation) return;
      drafts.delete(record.id);
      state.loaded = 0; await load();
    } catch(error) { if (generation === state.generation) { state.errors.controls = error.message; update(); } }
    finally { if (generation === state.generation) {saving.delete(record.id); update();} }
  }
  function render(shouldLoad = true) {
    const focused=document.activeElement;
    const focusedSetting=focused?.dataset?.prefSetting;
    const focusedInterval=focused?.id === 'pref-interval';
    const focusedRecord=focused?.closest?.('[data-pref-edit]')?.dataset.prefEdit;
    const focusedName=focusedRecord ? focused.name : null;
    const selection=focusedRecord ? [focused.selectionStart,focused.selectionEnd] : null;
    const active = document.activeElement?.id === 'preference-search';
    const caret = active ? document.activeElement.selectionStart : null;
    const opened = new Set([...(app.querySelectorAll?.('details') || [])].filter(detail=>detail.open && !detail.dataset.autoEmpty).map(detail=>detail.querySelector('summary')?.textContent));
    app.innerHTML = `${navigation()}<div class="main-shell"><main class="preference-page"><header><div class="eyebrow">个人记录</div><h1>我的偏好</h1></header>${content()}</main></div>`;
    const chart = app.querySelector('.starfish-panel');
    const page = app.querySelector('.preference-page');
    if (chart) page.querySelector('header').after(chart);
    const settingsSection = app.querySelector('.preference-learning');
    if (settingsSection) (chart || page.querySelector('header')).after(settingsSection);
    if (ui.status?.account?.loggedIn && !state.revoked && state.data.profile?.profile?.highlights?.length) {
      const overview = document.createElement('div');
      overview.innerHTML = understanding(state.data.profile);
      (settingsSection || chart || page.querySelector('header')).after(overview);
    }
    // Keep the interactive graph first and always visible; only long catalogs collapse.
    for (const id of ['pref-overview', 'pref-records']) {
      const section = app.querySelector(`#${id}`)?.closest('section');
      if (!section) continue;
      const disclosure = document.createElement('details');
      disclosure.className = 'preference-catalog';
      const heading = document.createElement('summary');
      heading.textContent = id === 'pref-overview' ? '全部偏好与来源' : '保存的偏好记录';
      section.before(disclosure); disclosure.append(heading, section);
      const empty = !state.busy && state.data.profile && !state.data.profile.profile?.rays?.length;
      disclosure.open = opened.has(heading.textContent) || Boolean(state.query) || empty;
      if (empty) disclosure.dataset.autoEmpty = 'true';
    }
    const management = document.createElement('section');
    const selectedRay = state.data.profile?.profile?.rays?.find(ray=>ray.id===state.selected);
    const controls = (state.data.controls?.records || []).filter(record=>!state.query || [record.summary,record.scope,record.rule].join(' ').toLowerCase().includes(state.query.toLowerCase()));
    controls.sort((a,b)=>Number(selectedRay?.sourceIds?.includes(b.id))-Number(selectedRay?.sourceIds?.includes(a.id)));
    management.innerHTML = `<h2>管理偏好</h2>${controls.map(record=>{
      const draft=drafts.get(record.id) || record;
      return `<details class="preference-controls" data-pref-record="${esc(record.id)}" ${drafts.has(record.id)?'open':''}><summary>${esc(record.summary)}${record.disabled?' · 已停用':''}</summary><form data-pref-edit="${esc(record.id)}"><label>偏好摘要<textarea name="summary" required maxlength="400">${esc(draft.summary)}</textarea></label><label>适用范围<textarea name="scope" maxlength="30000">${esc(draft.scope || '')}</textarea></label><label>选择标准<textarea name="rule" maxlength="30000">${esc(draft.rule || '')}</textarea></label><label>例外情况<textarea name="exceptions" maxlength="30000">${esc(draft.exceptions || '')}</textarea></label><button class="button secondary" type="submit" ${saving.has(record.id)?'disabled':''}>${saving.has(record.id)?'保存中…':'保存修正'}</button><button class="text-button" type="button" data-pref-cancel="${esc(record.id)}">取消修改</button><button class="button secondary" type="button" data-pref-toggle="${esc(record.id)}" ${saving.has(record.id)?'disabled':''}>${record.disabled?'恢复':'停用'}</button></form></details>`;
    }).join('')}`;
    app.querySelector('.preference-page')?.append(management);
    const activity = app.querySelector('#pref-activity')?.closest('section');
    for (const [section, label] of [[activity, '近期变化'], [management, '管理偏好']]) {
      if (!section) continue;
      const disclosure = document.createElement('details');
      disclosure.className = 'preference-secondary';
      const heading = document.createElement('summary');
      heading.textContent = label;
      section.before(disclosure); disclosure.append(heading, section);
      disclosure.open = opened.has(label) || Boolean(state.query);
    }
    if (activity) {
      // Identical visible changes are one reading item, not multiple learning achievements.
      const seen = new Set();
      for (const item of activity.querySelectorAll('.preference-change')) {
        const key = item.textContent.replace(/明确偏好|反馈学习|偏好学习/g, '').trim();
        if (seen.has(key)) item.remove(); else seen.add(key);
      }
    }
    management.addEventListener('submit', event=>{event.preventDefault(); const record=state.data.controls?.records?.find(record=>record.id===event.target.dataset.prefEdit); if(record) {const draft=drafts.get(record.id) || record;const patch={};for(const key of ['summary','scope','rule','exceptions'])if((draft[key]||'')!==(draft.original?.[key]??record[key]??''))patch[key]=draft[key]||'';changePreference({...record,revision:draft.revision},'edit',patch);}});
    for (const detail of [...(app.querySelectorAll?.('details') || [])]) if (opened.has(detail.querySelector('summary')?.textContent)) detail.open = true;
    if (active) { const input = app.querySelector('#preference-search'); input?.focus(); if (caret !== null) input?.setSelectionRange?.(caret, caret); }
    if(focusedRecord&&focusedName){const input=app.querySelector(`[data-pref-edit="${CSS.escape(focusedRecord)}"] [name="${CSS.escape(focusedName)}"]`);input?.focus();if(selection)input?.setSelectionRange?.(...selection);}
    if (focusedSetting) app.querySelector(`[data-pref-setting="${CSS.escape(focusedSetting)}"]`)?.focus();
    if (focusedInterval) app.querySelector('#pref-interval')?.focus();
    graph?.bind(app);
    if (shouldLoad) { syncAccount(); load(); }
  }
  function click(event) {
    if (event.target.closest('[data-pref-interval-reset]')) { saveSettings({observerInterval:null}); return true; }
    const zoomButton=event.target.closest('[data-pref-zoom]');
    if(zoomButton){zoom=Math.max(1,Math.min(2.5,zoom+Number(zoomButton.dataset.prefZoom)*.5));update();return true;}
    if(event.target.closest('[data-pref-show-record]')){const rays=state.data.profile?.profile?.rays||[];const ray=rays.find(ray=>ray.id===state.selected)||rays[0];const id=ray?.sourceIds?.[0];const target=id&&app.querySelector(`[data-pref-record="${CSS.escape(id)}"]`);if(target){target.open=true;target.scrollIntoView({block:'center'});target.querySelector('textarea')?.focus();}return true;}
    const cancel = event.target.closest('[data-pref-cancel]');
    if(cancel) {drafts.delete(cancel.dataset.prefCancel);update();return true;}
    const toggle = event.target.closest('[data-pref-toggle]');
    if (toggle) { const record=state.data.controls?.records?.find(record=>record.id===toggle.dataset.prefToggle); if(record) changePreference(record,record.disabled?'restore':'disable'); return true; }
    const ray = event.target.closest('[data-pref-ray]');
    if (ray) { state.selected = ray.dataset.prefRay; update(); app.querySelector(`[data-pref-ray="${CSS.escape(state.selected)}"]`)?.focus(); return true; }
    if (event.target.closest('[data-pref-refresh]')) { load(true); return true; }
    const filter = event.target.closest('[data-pref-category]');
    if (filter) { state.category = filter.dataset.prefCategory; update(); return true; }
    if (event.target.closest('[data-pref-logout]')) {
      clear(); state.identity = ''; state.revoked = true; update();
      request('/runtime/account/logout', { method: 'POST' }).then(() => { ui.status.account = {}; state.revoked = false; update(); }).catch(error => { state.errors.session = error.message; update(); });
      return true;
    }
    return false;
  }
  function input(event) {
    const form=event.target.closest('[data-pref-edit]');
    if(form) {const record=state.data.controls?.records?.find(record=>record.id===form.dataset.prefEdit);if(record){const draft=drafts.get(record.id)||{...record,original:{...record}};draft[event.target.name]=event.target.value;drafts.set(record.id,draft);}return;}
    if (event.target.id === 'preference-search') { state.query = event.target.value; update(); }
  }
  function beginLogin() { clear(); state.revoked = true; update(); }
  return { render, syncAccount, click, input, change, beginLogin };
};
