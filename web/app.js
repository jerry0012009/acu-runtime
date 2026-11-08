const app = document.getElementById('app');
const overlay = document.getElementById('overlay');
const token = document.querySelector('meta[name="acu-token"]').content;
const names = { codex: 'Codex', claude: 'Claude Code', workbuddy: 'WorkBuddy', hermes: 'Hermes' };
const icons = {
  grid: '<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
  link: '<path d="M10 13a5 5 0 0 0 7 .2l3-3a5 5 0 0 0-7-7l-2 2M14 11a5 5 0 0 0-7-.2l-3 3a5 5 0 0 0 7 7l2-2"/>',
  models: '<path d="m12 3 9 5-9 5-9-5 9-5ZM3 12l9 5 9-5M3 16l9 5 9-5"/>',
  settings: '<path d="M4 7h16M4 17h16"/><circle cx="8" cy="7" r="3"/><circle cx="16" cy="17" r="3"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  refresh: '<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6 7a7 7 0 0 1 12-1l2 3M4 15l2 3a7 7 0 0 0 12-1"/>',
  check: '<path d="m5 12 4 4 10-10"/>',
  close: '<path d="m6 6 12 12M6 18 18 6"/>',
  shield: '<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Z"/><path d="m8 12 3 3 5-6"/>',
  terminal: '<path d="m4 6 5 6-5 6M13 18h7"/>',
  image: '<rect x="3" y="3" width="18" height="18" rx="3"/><circle cx="8" cy="8" r="2"/><path d="m21 15-5-5L5 21"/>',
  eye: '<path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c5 5 5 13 0 18-5-5-5-13 0-18Z"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  cpu: '<rect x="6" y="6" width="12" height="12" rx="3"/><path d="M9 2v4m6-4v4M9 18v4m6-4v4M2 9h4m-4 6h4m12-6h4m-4 6h4"/>',
  chevron: '<path d="m9 5 7 7-7 7"/>',
};
const icon = (name, className = '') => `<svg class="icon ${className}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name] || icons.grid}</svg>`;
const escapeHTML = (value = '') => String(value).replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
const ui = { page: 'overview', status: null, busy: false, frame: null, error: '', autoStarted: false, selected: null, applying: false, results: [], progress: '', accountBusy: false, keySelection: null, providerSelection: null, setup: null, disconnect: null, inventory: null, inventoryTab: 'all', inventoryBulk: null, inventorySaving: false, polling: false, connectionOpen: false };
const launchQuery = new URLSearchParams(globalThis.location?.search || '');
if (launchQuery.get('page') === 'overview' || launchQuery.has('native')) ui.page = 'overview';
let pendingNative = launchQuery.get('native');
const nativeViews = new Set(['agents', 'providers', 'settings', 'library', 'usage', 'routing', 'plugins', 'masking']);

async function request(path, options = {}) {
  const response = await fetch(path, { ...options, headers: { 'X-ACU-Token': token, ...options.headers } });
  const result = await response.json();
  if (!response.ok || result.error) throw new Error(result.error || `请求失败（${response.status}）`);
  return result;
}

async function requestKernel(path, options = {}) {
  const response = await fetch(path, { ...options, headers: { 'X-ACU-Token': token, ...options.headers } });
  const payload = typeof response.text === 'function' ? await response.text() : JSON.stringify(await response.json());
  let result = {};
  try { result = payload ? JSON.parse(payload) : {}; } catch { /* the proxy may return plain text for a transport error */ }
  if (!response.ok || result.error) throw new Error(result.error || `内核请求失败（${response.status}）`);
  return result;
}

function toast(message) {
  const target = document.getElementById('toast');
  target.textContent = message;
  target.classList.add('show');
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => target.classList.remove('show'), 4000);
}

async function refresh(rescan = false, quiet = false) {
  if (ui.applying || ui.accountBusy) return;
  if (ui.polling) return;
  ui.polling = true;
  clearTimeout(refresh.timer);
  if (!quiet) { ui.busy = true; render(); }
  try {
    if (rescan && ui.status?.ready) {
      const response = await fetch('/api/agents/rescan', { method: 'POST', headers: { 'X-ACU-Token': token } });
      if (!response.ok) throw new Error(`内核扫描失败（${response.status}）`);
    }
    const previousAccount = ui.status?.account;
    const previousState = JSON.stringify(ui.status);
    ui.status = await request('/runtime/status');
    if (ui.providerSelection !== null && !(ui.status.connection?.choices || []).some(choice => choice.id === ui.providerSelection)) ui.providerSelection = '';
    if (previousAccount?.username !== ui.status.account?.username || previousAccount?.site !== ui.status.account?.site) ui.keySelection = null;
    if (ui.status.ready && ui.selected === null) ui.selected = new Set((ui.status.agents || []).filter(agent => agent.detected && !agent.connected).map(agent => agent.id));
    if (ui.status.desktop && !ui.autoStarted) {
      ui.autoStarted = true;
      if (!ui.status.ready && !ui.status.starting) ui.status = await request('/runtime/kernel/start', { method: 'POST' });
    }
    ui.error = ui.status.error || '';
    if (ui.inventory && previousState !== JSON.stringify(ui.status)) {
      const scroll = overlay.querySelector('.recommendation-body')?.scrollTop || 0;
      const expanded = new Set([...(overlay.querySelectorAll?.('[data-flow-key][open]') || [])].map(detail => detail.dataset.flowKey));
      const searching = document.activeElement?.id === 'inventory-search';
      const caret = searching ? document.activeElement.selectionStart : null;
      openModelInventory(ui.inventory);
      for (const detail of overlay.querySelectorAll?.('[data-flow-key]') || []) detail.open = expanded.has(detail.dataset.flowKey);
      const body = overlay.querySelector('.recommendation-body');
      if (body) body.scrollTop = scroll;
      if (searching) {
        const search = overlay.querySelector('#inventory-search');
        search?.focus({ preventScroll: true });
        if (caret !== null) search?.setSelectionRange?.(caret, caret);
      }
    }
    if (quiet && previousState !== JSON.stringify(ui.status) && ['agents', 'overview', 'settings'].includes(ui.page) && !ui.setup) render();
    if (ui.status.ready && pendingNative) {
      const view = pendingNative;
      pendingNative = null;
      if (nativeViews.has(view)) openNative(view, '', false, launchQuery.get('tab') || '', '', '', launchQuery);
    }
  } catch (error) { ui.error = error.message; if (ui.status) ui.status.connection = { known: false, configured: false, choices: [] }; if (quiet && ['agents', 'overview'].includes(ui.page) && !ui.setup) render(); }
  finally { ui.polling = false; ui.busy = false; if (!quiet) render(); refresh.timer = setTimeout(() => refresh(false, true), ui.status?.starting || ui.status?.account?.pending ? 1500 : 4000); }
}

function navigation() {
  return `<aside class="sidebar"><a class="brand" href="#overview" data-page="overview" aria-label="ACU Runtime 首页"><span class="brand-symbol">A</span><span>ACU<span class="brand-sub">RUNTIME</span></span></a>
    <nav aria-label="主导航">${[['overview', 'grid', '工作台'], ['usage', 'clock', '用量'], ['routing', 'link', '请求记录'], ['settings', 'settings', '设置']].map(([page, glyph, label]) => `<button class="nav-item ${ui.page === page ? 'active' : ''}" data-page="${page}" ${ui.page === page ? 'aria-current="page"' : ''}>${icon(glyph)}<span>${label}</span></button>`).join('')}</nav>
    <div class="sidebar-bottom"><span class="sidebar-signature">LESS COST.<br>MORE POSSIBILITY.</span></div></aside>`;
}

function header(eyebrow, title, subtitle, action = '') {
  return `<div class="page-heading"><div><div class="eyebrow">${eyebrow}</div><h1>${title}</h1><p>${subtitle}</p></div>${action}</div>`;
}

function kernelBanner() {
  if (ui.status?.ready) return '';
  const status = ui.status;
  const preparing = !status || status.starting;
  const percent = status?.downloadTotal > 0 ? ` · ${Math.min(100, Math.round(status.downloaded / status.downloadTotal * 100))}%` : '';
  return `<section class="kernel-banner" role="status"><div class="kernel-glyph">${icon('refresh', preparing ? 'spinning' : '')}</div><div class="banner-copy"><strong>${escapeHTML(preparing ? '正在准备 ACU' + percent : 'ACU 暂时无法连接')}</strong><p>${preparing ? '首次启动可能需要片刻，请稍候。' : '请检查网络后重试，已有 Agent 配置不会被删除。'}</p></div><button class="button secondary" data-action="connect" ${preparing || ui.busy ? 'disabled' : ''}>${preparing ? '准备中' : '重试'}</button></section>`;
}

function activeChoice() {
  const connection = ui.status?.connection || {};
  const provider = ui.providerSelection === null ? connection.provider : ui.providerSelection;
  if (provider === connection.provider && !connection.configured) return;
  return connection.choices?.find(choice => choice.id === provider);
}

// Curated text-model candidates, smoke-tested through Magpie on 2026-11-06.
// This is a recommendation list, not a permanent availability guarantee.
const recommendedACUModels = ['acu-auto', 'gpt-6.1-sol', 'gpt-6-astra', 'gpt-6-luna', 'glm-5.3-flash', 'glm-5.3', 'deepseek-v4-pro', 'deepseek-v4.1-flash', 'deepseek-v4.1-flash-1017', 'kimi-k3', 'claude-opus-5-5', 'claude-sonnet-5', 'claude-fable-5'];
function recommendedModels(choice) {
  return recommendedACUModels.filter(model => choice?.models?.includes(model));
}

function agentRecommendation(agent, choice, selectedModel, settings) {
  const options = agent?.modelOptions || [];
  const nativeModels = [...new Set(options.filter(option => !option.ref && option.value).map(option => option.label || option.value))];
  const availableModels = recommendedModels(choice).filter(model => agent.id === 'workbuddy' || options.some(option => option.ref === `${choice.id}/${model}`));
  const recommendedModel = availableModels.includes('acu-auto') ? 'acu-auto' : availableModels[0] || '';
  const model = availableModels.includes(selectedModel) ? selectedModel : recommendedModel;
  const namesByAgent = {
    codex: { summary: '设置 ACU 默认模型，并同步可选模型。你仍然在 Codex 中使用。', change: '不把所有 GPT 模型统一改走 ACU。原有 ChatGPT 模型按原来源处理；实际可选列表随登录方式而定。', retained: '不主动切换登录方式；继续沿用现有工具权限。' },
    claude: { summary: '将主模型接入 ACU，继续使用 Claude Code 的工具与权限确认。', change: '只指定主模型，不统一替换快速任务、子任务等模型；这些请求按现有路由处理。', retained: '保留现有工具权限和其他角色的选模设置。' },
    workbuddy: { summary: '把 ACU 模型准备到列表，仍由你在 WorkBuddy 任务内选择。', change: '选择 ACU 模型的任务才走对应 ACU 来源，不改变其他任务的选择。', retained: '保留应用内选模与权限确认；同时同步其他已启用的模型来源。' },
    hermes: { summary: '设置 ACU 默认模型，保留 Hermes 的 bot 内部选模。', change: '更改默认模型；bot 已有的模型覆盖设置不在本次统一替换。', retained: '保留其他角色的选模和现有工具权限。' },
  };
  const copy = namesByAgent[agent.id];
  const named = (reference, models = []) => {
    const entry = models.find(item => item.id === reference);
    return entry ? `${entry.name || entry.id}${entry.pname ? ` · ${entry.pname}` : ''}` : reference;
  };
  const fallback = (field, automatic, missing, models) => {
    if (!settings) return ['暂未读取到配置', '待确认'];
    if (settings[field] === 'off') return ['补充服务已关闭', '未启用'];
    const reference = settings[field] && settings[field] !== settings[missing] ? settings[field] : settings[automatic];
    return reference ? [named(reference, settings[models]), '已有配置'] : ['未配置补充服务', '待补齐'];
  };
  const vision = fallback('vision', 'visionAuto', 'visionMissing', 'visionModels');
  const imageGen = fallback('imageGen', 'imageGenAuto', 'imageGenMissing', 'imageGenModels');
  const searchAPI = settings?.searchAPIs?.find(api => api.ready && !api.off);
  const searcher = settings?.searchProvider;
  const search = !settings ? ['暂未读取到配置', '待确认'] : searcher ? [named(searcher, (settings.searchChoices || []).flatMap(item => item.models || [])), settings.searchFailed || settings.searchUnused ? '需检查' : '已有配置'] : searchAPI ? [searchAPI.name || searchAPI.vendor, '已有配置'] : ['未配置补充搜索来源', '待补齐'];
  const routeModel = agent.id === 'workbuddy' ? '在任务中选择的 ACU 模型' : model;
  const capabilities = [
    ['看图', `${routeModel || '所选模型'} 原生识图优先；补充识图：${vision[0]}。`, vision[1]],
    ['读文档', '继续使用应用的文件工具；扫描 PDF 还需要识图或 OCR，不能仅凭接入保证可读。', '依文件而定'],
    ['联网搜索', `应用发起搜索后，沿用现有处理规则。补充来源：${search[0]}。`, search[1]],
    ['生图', `沿用现有生图工具；补充生图：${imageGen[0]}。本次不新增 MCP。`, imageGen[1]],
    ['生视频', '本次不配置异步视频生成。', '暂不提供'],
  ];
  return { nativeModels, availableModels, recommendedModel, model, capabilities, ...copy };
}

function compactConnection() {
  const account = ui.status?.account || {};
  const choice = activeChoice();
  const choices = ui.status?.connection?.choices || [];
  return `<section class="panel quick-connection ${choice ? 'is-connected' : 'needs-connection'}" aria-label="ACU 连接"><div class="connection-identity"><span class="quick-check ${choice ? 'complete' : ''}" aria-label="${choice ? '已连接 ACU' : '尚未连接 ACU'}">${icon(choice ? 'check' : 'link')}</span><div><h2>${choice ? '已连接 ACU' : '连接 ACU'}</h2><p>${choice ? '连接配置已完成' : '登录后自动连接，也可以使用已有 Key。'}</p></div></div>
    <div class="connection-controls">${account.loggedIn ? `<span class="account-name">${escapeHTML(account.username || 'ACU 账号')}</span>` : `<button class="button primary login-button" data-action="account-login" ${ui.accountBusy || account.pending || !ui.status?.desktop ? 'disabled' : ''}>${icon('arrow')}${account.pending ? '登录中…' : '登录 ACU'}</button>`}
    ${choices.length || account.loggedIn ? `<div class="connection-key-group"><select id="quick-key" aria-label="启用的 API Key" ${ui.applying || ui.accountBusy ? 'disabled' : ''}><option value="" ${!choice ? 'selected' : ''}>选择 API Key</option>${choices.map(current => `<option value="local:${escapeHTML(current.id)}" ${choice?.id === current.id ? 'selected' : ''}>${escapeHTML(current.name || 'ACU')} · ${escapeHTML(current.masked || '已保存 Key')}</option>`).join('')}${account.loggedIn ? (account.keys || []).filter(key => key.available).map(key => `<option value="account:${key.id}">${escapeHTML(key.name || `Key #${key.id}`)}</option>`).join('') : ''}</select><button class="text-button connection-manage" data-action="manage-connection" aria-expanded="${ui.connectionOpen}">${choice ? '更换 Key' : '使用 API Key'}</button></div>` : '<button class="text-button connection-manage" data-action="manage-connection" aria-expanded="false">使用 API Key</button>'}</div>
    ${account.pending ? '<span class="login-progress" role="status">请在登录窗口完成登录，成功后自动返回。 <button class="text-button" data-action="account-cancel">取消</button></span>' : ''}
    ${ui.status?.connection?.error || account.error ? `<span class="quick-error" role="alert">${escapeHTML(ui.status.connection?.error || account.error)}</span>` : ''}</section>`;
}

function usageGuide() {
  const connected = activeChoice();
  const configured = connected && (ui.status?.agents || []).some(agent => agent.connected && !agent.needsRepair);
  const steps = `<ol class="welcome-steps" aria-label="使用指南"><li><span class="flow-number ${connected ? 'complete' : ''}" aria-label="步骤 1">${connected ? icon('check') : '1'}</span><div><strong>连接 ACU</strong><p>登录或粘贴 API Key</p></div></li><li><span class="flow-number ${configured ? 'complete' : ''}" aria-label="步骤 2">${configured ? icon('check') : '2'}</span><div><strong>接入常用 Agent</strong><p>选择应用，确认一键配置</p></div></li><li><span class="flow-number" aria-label="步骤 3">3</span><div><strong>回到 Agent 使用</strong><p>按提示重开，照常选模型</p></div></li></ol>`;
  return !connected || !configured ? steps : `<details class="welcome-help"><summary>如何开始使用？</summary>${steps}</details>`;
}

function overviewPage() {
  return `${header('', '你的 Agent，一处管理。', '连接 ACU，继续用你熟悉的工具。', ui.status?.desktop ? '<button class="button secondary" data-action="show-panel">快捷面板</button>' : '')}
    <div id="home-preparation">${kernelBanner()}</div><div id="home-connection">${compactConnection()}</div>
    <div id="connection-manager" ${ui.connectionOpen ? '' : 'hidden'}>${ui.connectionOpen ? `<section class="panel setup-stage"><div class="manager-heading"><h2>管理连接</h2><button class="text-button" data-action="manage-connection">收起</button></div>${connectionContent()}</section>` : ''}</div>
    <div id="home-workbench">${usageGuide()}${agentsPage()}</div>`;
}

function agentsPage() {
  const agents = ui.status?.agents || Object.entries(names).map(([id, name]) => ({ id, name }));
  const detected = agents.filter(agent => agent.detected);
  const restarting = agents.filter(agent => agent.connected && agent.restartRequired);
  return `<section class="agent-stage" aria-label="我的 Agent"><div class="workbench-heading"><div><h2>我的 Agent <span class="count-label">${detected.length}</span></h2><p>${activeChoice() ? '一键接入，随时调整模型与工具。' : '先连接 ACU，再选择要接入的应用。'}</p></div><div class="workbench-actions"><button class="button secondary" data-action="capabilities" ${!ui.status?.ready || !activeChoice() || ui.applying ? 'disabled' : ''}>图片与搜索</button><button class="icon-button" title="重新检测 Agent" aria-label="重新检测 Agent" data-action="refresh" ${ui.busy || ui.applying ? 'disabled' : ''}>${icon('refresh', ui.busy ? 'spinning' : '')}</button><button class="button secondary batch-configure" data-action="apply-agents" ${!ui.status?.ready || !activeChoice() || !detected.length || ui.busy || ui.applying || ui.accountBusy ? 'disabled' : ''}>${ui.applying ? escapeHTML(ui.progress) : '一键配置'}${icon('arrow')}</button></div></div>
    <div class="agent-grid">${agents.map(agentCard).join('')}</div>
    ${ui.results.length ? `<div class="setup-results" aria-live="polite">${ui.results.map(result => `<div class="setup-result ${result.ok ? '' : 'failed'}"><strong>${escapeHTML(names[result.id] || (result.id === 'tools' ? '图片与搜索' : result.id))}</strong><span>${escapeHTML(result.message)}</span></div>`).join('')}</div>` : ''}
    ${ui.results.some(result => !result.ok && result.id !== 'tools') ? '<button class="button secondary" data-action="retry-setup">重试未完成应用</button>' : ''}${ui.results.some(result => !result.ok && result.id === 'tools') ? '<button class="button secondary" data-action="capabilities">处理未完成工具</button>' : ''}
    ${restarting.length ? `<div class="restart-summary" role="status">${icon('refresh')}<span>${restarting.map(agent => escapeHTML(agent.name)).join('、')}：保存工作后重新打开，让配置生效。</span></div>` : ''}</section>`;
}

function connectionContent() {
  const connection = ui.status?.connection || {};
  const account = ui.status?.account || {};
  const disabled = ui.accountBusy || ui.applying || account.pending;
  return `<div class="login-options"><span class="field-label">登录站点</span><strong class="login-site">acucompute.com</strong><button class="button secondary" data-action="account-login" ${disabled || !ui.status?.desktop ? 'disabled' : ''}>${account.loggedIn ? '切换账号' : '登录 ACU'}</button><button class="text-button" data-action="account-register" ${disabled || !ui.status?.desktop ? 'disabled' : ''}>注册账号</button></div>
    ${account.loggedIn && !(account.keys || []).some(key => key.available) ? '<p class="connection-hint">账号暂无可用 Key，可以在官网创建后重新连接，或填写已有 Key。</p>' : ''}
    <form class="manual-connect" data-connection-form><label class="field-label" for="acu-key">ACU API Key</label><div class="key-picker"><input id="acu-key" name="key" type="password" autocomplete="off" placeholder="粘贴已有 Key" required ${disabled ? 'disabled' : ''}><button type="submit" class="button secondary" ${disabled || !ui.status?.ready ? 'disabled' : ''}>${ui.accountBusy ? '正在连接…' : '连接'}</button></div><details class="base-options"><summary>API 地址</summary><label class="field-label" for="acu-base">Base URL</label><input id="acu-base" name="base" type="url" value="${escapeHTML(connection.base || 'https://acu.best/v1')}" required ${disabled ? 'disabled' : ''}></details></form>
    <button class="text-button advanced-connection" data-action="native-providers" ${!ui.status?.ready || disabled ? 'disabled' : ''}>管理已保存的 Key 与模型来源 ${icon('arrow')}</button>`;
}

function modelFacts(agent) {
  const choices = ui.status?.connection?.choices || [];
  const catalog = agent.catalog;
  const models = catalog?.models || [];
  const isACU = reference => choices.some(choice => reference?.startsWith(choice.id + '/'));
  const acu = models.filter(model => isACU(model.id) && !model.own);
  const current = (agent.modelOptions || []).find(option => option.value === agent.model);
  const roles = agent.roles || (agent.model ? [{ key: 'model', value: agent.model, ref: current?.ref, name: current?.label, group: current?.group }] : []);
  const routeName = reference => {
    if (!reference) return '沿用当前来源';
    const choice = choices.find(item => reference.startsWith(item.id + '/'));
    if (choice) return choice.name || 'ACU';
    const group = (ui.status?.routingGroups || []).find(item => item.ref === reference);
    if (group || isRoutingGroup({ id: reference })) return '高级配置';
    return models.find(item => item.id === reference)?.group || '其他来源';
  };
  const label = role => ({ model: '默认模型', small: '快速任务', opus: 'Opus', sonnet: 'Sonnet', haiku: 'Haiku', fable: 'Fable', subagent: '子任务模型', subagent_model: '子任务模型', memories: '记忆任务' })[role.key] || role.key;
  const roleRows = roles.map(role => ({ key: role.key, value: role.value, name: role.name, label: label(role), model: role.name || role.ref?.split('/').slice(1).join('/') || role.value, destination: role.group && !role.ref ? role.group : routeName(role.ref), ref: role.ref }));
  const configured = agent.connected && !agent.needsRepair;
  const enabled = acu.filter(model => !model.hidden && !(model.own && catalog?.ownHidden));
  const top = [...enabled].sort((left, right) => Number(right.id.endsWith('/acu-auto')) - Number(left.id.endsWith('/acu-auto'))).slice(0, 3);
  const nativeEntries = [...new Map((agent.modelOptions || []).filter(option => !option.ref && option.value).map(option => [option.value, { id: option.value, name: option.label || option.value, group: `${agent.name} 原生`, own: true, nativeOption: true, inUse: option.value === agent.model }])).values()];
  return { catalog, roleRows, configured, top, nativeEntries, routeName };
}

function modelIsHidden(agent, model) {
  return Boolean(model.hidden || (model.own && agent.catalog?.ownHidden));
}

function isRoutingGroup(model) {
  const ref = model.ref || model.id || model.value || '';
  return model.group === 'Routing groups' || ref.startsWith('auto/') || (ui.status?.routingGroups || []).some(group => group.ref === ref);
}

function modelOptionLabel(option, agent) {
  const source = (ui.status?.connection?.choices || []).find(choice => (option.ref || option.value || '').startsWith(choice.id + '/'));
  const suffix = isRoutingGroup(option) ? '当前高级配置' : source ? source.name || 'ACU' : !option.ref ? `${agent.name} 原生` : option.group;
  return `${option.label || option.value || '默认'}${suffix ? ` · ${suffix}` : ''}`.replace(/\bmagpie\b/gi, '本机网关');
}

function modelOptionGroup(option, agent) {
  if (isRoutingGroup(option)) return '高级配置';
  const ref = option.ref || option.value || '';
  const source = (ui.status?.connection?.choices || []).find(choice => ref.startsWith(choice.id + '/'));
  if (source) {
    const model = ref.slice(source.id.length + 1);
    if (recommendedModels(source).includes(model)) return '推荐模型 · ACU';
    return '其他来源';
  }
  if (!option.ref) return `${agent.name} 原生`;
  return '其他来源';
}

function agentModelSelect(agent, role = null) {
  const current = role?.value || agent.model;
  const field = role?.key || 'model';
  let options = [...(agent.modelOptions || [])];
  if (!options.length && agent.catalog?.models?.length) options.push(...agent.catalog.models.filter(model => !model.own).map(model => ({ value: model.id, label: model.name || model.id, ref: model.id, group: model.group })));
  if (current && !options.some(option => option.value === current)) options.unshift({ value: current, label: role?.name || current, ref: role?.ref, group: role?.group });
  // Keep the workbench focused on native and direct provider choices; preserve an already-selected route.
  options = options.filter(option => {
    if (option.value === current) return true;
    if (isRoutingGroup(option)) return false;
    const ref = option.ref || option.value || '';
    const choice = (ui.status?.connection?.choices || []).find(item => ref.startsWith(item.id + '/'));
    return !choice || recommendedModels(choice).includes(ref.slice(choice.id.length + 1));
  });
  const groupRank = { '推荐模型 · ACU': 0, [`${agent.name} 原生`]: 1, '其他来源': 2, '高级配置': 3 };
  const acuRank = new Map(recommendedACUModels.map((model, index) => [model, index]));
  options.sort((left, right) => {
    const leftGroup = modelOptionGroup(left, agent);
    const rightGroup = modelOptionGroup(right, agent);
    if (leftGroup !== rightGroup) return (groupRank[leftGroup] ?? 9) - (groupRank[rightGroup] ?? 9);
    const leftRef = left.ref || left.value || '';
    const rightRef = right.ref || right.value || '';
    const leftModel = leftRef.includes('/') ? leftRef.split('/').slice(1).join('/') : leftRef;
    const rightModel = rightRef.includes('/') ? rightRef.split('/').slice(1).join('/') : rightRef;
    return (acuRank.get(leftModel) ?? 999) - (acuRank.get(rightModel) ?? 999) || String(left.label || left.value).localeCompare(String(right.label || right.value));
  });
  const optionHTML = group => options.filter(option => modelOptionGroup(option, agent) === group).map(option => `<option value="${escapeHTML(option.value)}" ${option.value === current ? 'selected' : ''}>${escapeHTML(modelOptionLabel(option, agent))}</option>`).join('');
  const groups = ['推荐模型 · ACU', `${agent.name} 原生`, '其他来源', '高级配置'].map(group => {
    const content = optionHTML(group);
    return content ? `<optgroup label="${escapeHTML(group)}">${content}</optgroup>` : '';
  }).join('');
  const disabled = !ui.status?.ready || !agent.detected || ui.applying || ui.accountBusy;
  const roleAttribute = role ? ` data-agent-role="${escapeHTML(field)}"` : '';
  const inputID = role ? `agent-${field}-${agent.id}` : `agent-model-${agent.id}`;
  return `<select id="${inputID}" data-agent-model="${agent.id}"${roleAttribute} aria-label="${escapeHTML(agent.name)} ${escapeHTML(role?.label || '默认模型')}" ${disabled ? 'disabled' : ''}>${groups}</select>`;
}

function modelFlow(agent, compact = false, inventory = null, summary = false) {
  const facts = modelFacts(agent);
  const entries = [];
  const add = (key, kind, name, target, badge, detail = '', muted = false, control = '', trailing = '', subtitle = '') => {
    const search = [name, target, badge, detail].join(' ').toLocaleLowerCase();
    const hidden = !compact && ui.inventoryQuery && !search.includes(ui.inventoryQuery.trim().toLocaleLowerCase());
    const mark = kind === 'route' ? icon('link') : kind === 'add' ? '+' : icon('shield');
    const line = `<span class="flow-mark ${kind}" aria-label="${kind === 'route' ? '路由' : kind === 'add' ? '新增' : '保留'}">${mark}</span><span class="flow-path"><span class="flow-name" title="${escapeHTML(name)}">${escapeHTML(name)}${subtitle ? `<small class="flow-provider">${escapeHTML(subtitle)}</small>` : ''}</span>${control ? `<span class="flow-arrow" aria-hidden="true">→</span>${control}` : target ? `<span class="flow-arrow" aria-hidden="true">→</span><strong title="${escapeHTML(target)}">${escapeHTML(target)}</strong>` : ''}</span>${badge ? `<span class="flow-badge">${escapeHTML(badge)}</span>` : ''}${trailing}`;
    entries.push(`<li class="flow-row ${muted ? 'is-muted' : ''}" data-flow-search="${escapeHTML(search)}" ${hidden ? 'hidden' : ''}>${!compact && detail && inventory?.tab !== 'order' ? `<details data-flow-key="${escapeHTML(key)}"><summary class="flow-line">${line}${icon('chevron', 'flow-chevron')}</summary><div class="flow-detail">${escapeHTML(detail)}</div></details>` : `<div class="flow-line">${line}</div>`}</li>`);
  };
  if ((!inventory || inventory.tab !== 'order') && facts.configured && agent.id !== 'workbuddy') {
    const roles = compact ? facts.roleRows.slice(0, 2) : facts.roleRows;
    const hasModelChoices = agent.modelOptions?.length || agent.catalog?.models?.length;
    roles.forEach(role => add('role:' + role.label, 'route', role.label, role.model, role.ref ? facts.routeName(role.ref) : '原来源', '', false, compact && ['默认模型', '子任务模型'].includes(role.label) && hasModelChoices ? agentModelSelect(agent, role) : ''));
    if (compact && !roles.some(role => role.label === '默认模型') && hasModelChoices) add('role:default', 'route', '默认模型', '', '', '', false, agentModelSelect(agent));
  }
  const previewed = new Set(facts.configured && agent.id !== 'workbuddy' ? facts.roleRows.slice(0, 2).map(role => role.ref) : []);
  const catalogModels = (facts.catalog?.models || []).filter(model => !isRoutingGroup(model));
  const catalogIDs = new Set(catalogModels.map(model => model.id));
  const fallbackNative = facts.nativeEntries.filter(model => !catalogIDs.has(model.id));
  const models = compact ? [...facts.top.filter(model => !previewed.has(model.id)), ...fallbackNative.filter(model => !modelIsHidden(agent, model)).slice(0, 1)] : inventory?.tab === 'order' ? catalogModels.filter(model => !modelIsHidden(agent, model)) : [...catalogModels, ...fallbackNative].sort((left, right) => Number(right.id.endsWith('/acu-auto')) - Number(left.id.endsWith('/acu-auto')));
  if (summary) models.length = 0;
  models.forEach(model => {
    if (inventory?.tab === 'shown' && modelIsHidden(agent, model)) return;
    const group = (ui.status?.routingGroups || []).find(item => item.ref === model.id);
    const source = group ? '自动路由' : facts.routeName(model.id);
    const hidden = modelIsHidden(agent, model);
    const mode = hidden ? '已隐藏' : model.own ? '原生' : agent.needsRepair ? '待核对' : !agent.connected ? '可添加' : source;
    const detail = [group?.members?.length ? `候选来源：${group.members.map(member => member.split('/')[0]).filter((value, index, all) => all.indexOf(value) === index).join('、')}` : '', group?.patterns?.length ? `适用模型：${group.patterns.join('、')}` : ''].filter(Boolean).join(' · ');
    const provider = !model.own && !group && model.id.includes('/') ? model.id.split('/')[0] : '';
    const visibility = inventory && inventory.tab !== 'order' && model.inUse ? '<span class="model-in-use" title="请先切换默认模型或子任务模型，再隐藏此模型">使用中 · 保留显示</span>' : inventory && inventory.tab !== 'order' ? `<button type="button" class="model-visibility ${hidden ? '' : 'is-on'}" data-model-visibility data-agent-id="${escapeHTML(agent.id)}" data-model-id="${escapeHTML(model.id)}" role="switch" aria-checked="${!hidden}" ${model.inUse || model.own || ui.inventorySaving ? 'disabled' : ''} title="${escapeHTML(model.inUse ? '当前模型会始终保留' : model.own ? '原生模型由上方开关控制' : hidden ? '显示此模型' : '隐藏此模型')}"><span aria-hidden="true"></span><span class="sr-only">${hidden ? '隐藏' : '显示'}</span></button>` : '';
    const order = inventory?.tab === 'order' ? `<span class="model-order-actions"><button type="button" data-model-move="up" data-agent-id="${escapeHTML(agent.id)}" data-model-id="${escapeHTML(model.id)}" ${models.indexOf(model) === 0 || ui.inventorySaving ? 'disabled' : ''} aria-label="上移">↑</button><button type="button" data-model-move="down" data-agent-id="${escapeHTML(agent.id)}" data-model-id="${escapeHTML(model.id)}" ${models.indexOf(model) === models.length - 1 || ui.inventorySaving ? 'disabled' : ''} aria-label="下移">↓</button></span>` : '';
    add('model:' + model.id, model.own ? 'native' : 'add', model.name || model.id, '', mode, detail, hidden, '', visibility || order, provider);
  });
  if ((!inventory || inventory.tab !== 'order') && facts.configured && agent.id === 'workbuddy') {
    add('native:workbuddy', 'native', 'WorkBuddy 原生模型', '', '保留');
  } else if ((!inventory || inventory.tab !== 'order') && !agent.connected && agent.detected) {
    add('native:unchanged', 'native', '当前配置', '', '未改动');
  } else if ((!inventory || inventory.tab !== 'order') && compact && facts.configured) {
    if (!facts.nativeEntries.length) facts.catalog?.models.filter(model => model.own && !model.hidden).slice(0, 1).forEach(model => add('native:' + model.id, 'native', model.name || model.id, '', '原生'));
  }
  return `<ul class="model-flow ${compact ? 'compact-flow' : ''}">${entries.join('')}</ul>`;
}

function agentModelSummary(agent, advanced = '') {
  const catalog = agent.catalog;
  const models = (catalog?.models || []).filter(model => !isRoutingGroup(model));
  const facts = modelFacts(agent);
  const allModels = [...models, ...facts.nativeEntries.filter(model => !models.some(current => current.id === model.id))];
  const shown = allModels.filter(model => !modelIsHidden(agent, model));
  const visibilityCount = catalog ? `${shown.length}/${allModels.length}` : '';
  const acu = shown.filter(model => !model.own && model.id.startsWith('acu-runtime/')).length;
  const native = shown.filter(model => model.own).length || (agent.modelOptions || []).filter(option => !option.ref || option.ref.startsWith('native/')).length;
  const other = shown.filter(model => !model.own && !model.id.startsWith('acu-runtime/')).length;
  const exampleRanks = {
    codex: ['gpt-6-astra', 'gpt-6.1-sol', 'acu-auto'],
    claude: ['claude-opus-5-5', 'claude-sonnet-5', 'acu-auto'],
    workbuddy: ['gpt-6-luna', 'glm-5.3-flash', 'acu-auto'],
    hermes: ['deepseek-v4-pro', 'deepseek-v4.1-flash', 'acu-auto'],
  };
  const rank = exampleRanks[agent.id] || ['acu-auto', 'gpt-6.1-sol', 'gpt-6-astra'];
  const examples = shown.filter(model => !model.own && rank.includes(model.id.split('/').slice(1).join('/'))).sort((a, b) => rank.indexOf(a.id.split('/').slice(1).join('/')) - rank.indexOf(b.id.split('/').slice(1).join('/'))).slice(0, 3).map(model => `<span class="model-example">${escapeHTML(model.name || model.id.split('/').pop())}</span>`).join('');
  const stateHint = !agent.connected ? '<span class="model-example muted">可添加</span>' : agent.needsRepair ? '<span class="model-example muted">待核对</span>' : '';
  const nativeHint = agent.id === 'workbuddy' ? '<span class="model-example muted">WorkBuddy 原生模型 · 保留</span>' : '';
  const hiddenCompatibility = shown.filter(model => !model.own).slice(0, 4).map(model => `<span class="sr-only">${escapeHTML(model.name || model.id)}</span>`).join('');
  const summary = catalog ? `<div class="model-counts"><span class="model-visibility-count"><strong>${visibilityCount}</strong> 个模型已显示</span>${acu ? `<span><strong>${acu}</strong> 个 ACU</span>` : ''}${native ? `<span><strong>${native}</strong> 个原生</span>` : ''}${other ? `<span><strong>${other}</strong> 个其他来源</span>` : ''}<span class="model-examples">${nativeHint}${stateHint}${examples || '<span class="model-example muted">可在全部模型中选择</span>'}${hiddenCompatibility}</span></div>` : '';
  const modelTotal = visibilityCount;
  return `<section class="model-summary" aria-label="${escapeHTML(agent.name)} 模型清单"><div class="model-flow-compact-data">${modelFlow(agent, true, null, true)}</div>${summary}${!catalog && agent.detected ? `<p class="model-empty">${escapeHTML(agent.catalogError || '正在读取模型…')}</p>` : ''}<div class="model-summary-actions">${advanced}<button class="text-button model-list-link" data-action="model-inventory" data-agent-id="${agent.id}" ${!ui.status?.ready || !agent.detected ? 'disabled' : ''}><span>查看全部模型${modelTotal ? ` <span class="model-total">${modelTotal}</span>` : ''}</span>${icon('arrow')}</button></div></section>`;
}

function filterModelInventory() {
  const query = (ui.inventoryQuery || '').trim().toLocaleLowerCase();
  let count = 0;
  for (const row of overlay.querySelectorAll?.('[data-flow-search]') || []) {
    row.hidden = !row.dataset.flowSearch.includes(query);
    if (!row.hidden) count += 1;
  }
  const empty = overlay.querySelector('[data-flow-empty]');
  if (empty) empty.hidden = count > 0 || !query;
}

function inventorySnapshot(agent) {
  return agent?.catalog ? JSON.parse(JSON.stringify(agent.catalog)) : null;
}

function redrawInventory() {
  if (!ui.inventory) return;
  const body = overlay.querySelector?.('.recommendation-body');
  const scroll = body?.scrollTop || 0;
  const expanded = new Set([...(overlay.querySelectorAll?.('[data-flow-key][open]') || [])].map(detail => detail.dataset.flowKey));
  openModelInventory(ui.inventory);
  for (const detail of overlay.querySelectorAll?.('[data-flow-key]') || []) detail.open = expanded.has(detail.dataset.flowKey);
  const nextBody = overlay.querySelector?.('.recommendation-body');
  if (nextBody) nextBody.scrollTop = scroll;
}

async function saveInventoryChange(agent, body, before) {
  if (!agent?.catalog || ui.inventorySaving) return;
  ui.inventorySaving = true;
  redrawInventory();
  try {
    const result = await requestKernel(`/kernel/api/agent-models/${encodeURIComponent(agent.id)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (result?.models) agent.catalog = { ...agent.catalog, ...result, models: result.models };
    else if (result && typeof result === 'object') agent.catalog = { ...agent.catalog, ...result };
    toast('模型显示设置已保存');
  } catch (error) {
    agent.catalog = before;
    toast(error.message);
  } finally {
    ui.inventorySaving = false;
    redrawInventory();
  }
}

function visibilityPayload(agent) {
  const models = agent.catalog?.models || [];
  if (agent.catalog?.only) return { shown: models.filter(model => !modelIsHidden(agent, model)).map(model => model.id) };
  return { hidden: models.filter(model => !model.own && model.hidden).map(model => model.id) };
}

function toggleModelVisibility(agentId, modelId) {
  const agent = ui.status?.agents?.find(item => item.id === agentId);
  const model = agent?.catalog?.models?.find(item => item.id === modelId);
  if (!agent?.catalog || !model || model.inUse || model.own || ui.inventorySaving) return;
  const before = inventorySnapshot(agent);
  model.hidden = !model.hidden;
  saveInventoryChange(agent, visibilityPayload(agent), before);
}

function toggleInventoryFlag(agentId, flag) {
  const agent = ui.status?.agents?.find(item => item.id === agentId);
  if (!agent?.catalog || ui.inventorySaving) return;
  const before = inventorySnapshot(agent);
  if (flag === 'only') agent.catalog.only = !agent.catalog.only;
  if (flag === 'own') agent.catalog.ownHidden = !agent.catalog.ownHidden;
  const body = flag === 'only' ? { only: agent.catalog.only } : { ownHidden: agent.catalog.ownHidden };
  saveInventoryChange(agent, body, before);
}

function bulkInventory(agentId, action) {
  const agent = ui.status?.agents?.find(item => item.id === agentId);
  const models = agent?.catalog?.models || [];
  if (!agent?.catalog || ui.inventorySaving) return;
  if (ui.inventoryBulk !== action) {
    ui.inventoryBulk = action;
    redrawInventory();
    return;
  }
  ui.inventoryBulk = null;
  const before = inventorySnapshot(agent);
  for (const model of models) {
    if (model.inUse || model.own || isRoutingGroup(model)) continue;
    model.hidden = action === 'hide';
  }
  saveInventoryChange(agent, visibilityPayload(agent), before);
}

function moveInventoryModel(agentId, modelId, direction) {
  const agent = ui.status?.agents?.find(item => item.id === agentId);
  const models = agent?.catalog?.models || [];
  const shown = models.filter(model => !modelIsHidden(agent, model) && !isRoutingGroup(model));
  const index = shown.findIndex(model => model.id === modelId);
  if (!agent?.catalog || index < 0 || ui.inventorySaving) return;
  const target = index + (direction === 'up' ? -1 : 1);
  if (target < 0 || target >= shown.length) return;
  const before = inventorySnapshot(agent);
  [shown[index], shown[target]] = [shown[target], shown[index]];
  // Swap only ordinary model slots, retaining advanced entries and hidden entries in place.
  let position = 0;
  models.splice(0, models.length, ...models.map(model => !modelIsHidden(agent, model) && !isRoutingGroup(model) ? shown[position++] : model));
  saveInventoryChange(agent, { order: models.filter(model => !modelIsHidden(agent, model)).map(model => model.id) }, before);
}

function openModelInventory(id) {
  const agent = ui.status?.agents?.find(item => item.id === id);
  if (!agent || !ui.status?.ready) return;
  if (ui.inventory !== id) { ui.inventoryQuery = ''; ui.inventoryTab = 'all'; ui.inventoryBulk = null; }
  ui.inventory = id;
  const warning = agent.needsRepair ? '配置已变化，请重新核对' : !agent.connected ? '接入后可添加以下模型' : '';
  const catalog = agent.catalog;
  const facts = modelFacts(agent);
  const models = (catalog?.models || []).filter(model => !isRoutingGroup(model));
  const displayModels = [...models, ...facts.nativeEntries.filter(model => !models.some(current => current.id === model.id))];
  const shown = displayModels.filter(model => !modelIsHidden(agent, model)).length;
  const tab = ui.inventoryTab || 'all';
  const ownSwitch = (catalog?.own || facts.nativeEntries.length) ? `<div class="inventory-switch-row"><button type="button" class="model-visibility ${catalog?.ownHidden ? '' : 'is-on'}" data-inventory-flag="own" data-agent-id="${escapeHTML(agent.id)}" role="switch" aria-checked="${!catalog?.ownHidden}" ${ui.inventorySaving ? 'disabled' : ''}><span aria-hidden="true"></span></button><span><strong>显示 ${escapeHTML(agent.name)} 原生模型</strong><small>由应用当前登录方式提供</small></span></div>` : '';
  const onlySwitch = catalog ? `<div class="inventory-switch-row"><button type="button" class="model-visibility ${catalog.only ? 'is-on' : ''}" data-inventory-flag="only" data-agent-id="${escapeHTML(agent.id)}" role="switch" aria-checked="${!!catalog.only}" ${ui.inventorySaving ? 'disabled' : ''}><span aria-hidden="true"></span></button><span><strong>只显示我勾选的模型</strong><small>${catalog.only ? '新模型默认隐藏，勾选后才会出现' : '新模型默认显示'}</small></span></div>` : '';
  const tabs = catalog ? `<div class="inventory-toolbar"><div class="inventory-tabs" role="tablist" aria-label="模型视图"><button type="button" class="${tab === 'all' ? 'is-active' : ''}" data-inventory-tab="all" role="tab" aria-selected="${tab === 'all'}">全部</button><button type="button" class="${tab === 'shown' ? 'is-active' : ''}" data-inventory-tab="shown" role="tab" aria-selected="${tab === 'shown'}">已显示</button><button type="button" class="${tab === 'order' ? 'is-active' : ''}" data-inventory-tab="order" role="tab" aria-selected="${tab === 'order'}">排序</button></div><span class="inventory-count">${shown} / ${displayModels.length} 个模型已显示</span></div>` : '';
  const bulkLabel = action => ui.inventoryBulk === action ? (action === 'hide' ? '再次点击隐藏未使用' : '再次点击显示全部') : (action === 'hide' ? '隐藏未使用' : '显示全部');
  overlay.innerHTML = `<div class="scrim"><section class="modal inventory-modal" role="dialog" aria-modal="true" aria-labelledby="inventory-title"><div class="recommendation-header"><button class="icon-button modal-close" data-close aria-label="关闭">${icon('close')}</button><h2 id="inventory-title">${escapeHTML(agent.name)} · 模型</h2><p class="inventory-intro">在这里管理 Agent 模型列表；切换只影响显示，不改变模型路由。</p>${warning ? `<p class="inventory-warning">${warning}</p>` : ''}${ownSwitch}${onlySwitch}<input id="inventory-search" type="search" aria-label="搜索模型" placeholder="搜索模型" value="${escapeHTML(ui.inventoryQuery || '')}">${tabs}</div><div class="recommendation-body">${modelFlow(agent, false, { tab })}${!agent.catalog ? `<p class="model-empty" role="status">${escapeHTML(agent.catalogError || '正在读取模型…')}</p>` : ''}<p class="model-empty" data-flow-empty hidden>没有找到匹配的模型</p>${id === 'codex' && agent.loginMode === 'api' ? '<details class="inventory-note"><summary>原生模型说明</summary><p>Codex 原生模型由当前登录方式提供；API 接入时，具体可用模型以当前账号返回的列表为准。</p></details>' : ''}</div><div class="recommendation-footer"><div class="inventory-bulk-actions"><button type="button" class="text-button" data-inventory-bulk="hide" data-agent-id="${escapeHTML(agent.id)}" ${ui.inventorySaving ? 'disabled' : ''}>${bulkLabel('hide')}</button><span>·</span><button type="button" class="text-button" data-inventory-bulk="show" data-agent-id="${escapeHTML(agent.id)}" ${ui.inventorySaving ? 'disabled' : ''}>${bulkLabel('show')}</button></div><div class="modal-actions"><button class="button secondary" data-close>返回</button></div></div></section></div>`;
  document.body.classList.add('overlay-open');
  filterModelInventory();
}

function agentCard(agent) {
  const ready = ui.status?.ready;
  const options = agent.modelOptions || [];
  const selectedModel = options.find(option => option.value === agent.model);
  const inApp = agent.id === 'workbuddy';
  const routed = agent.connected && (selectedModel?.ref || agent.model === 'magpie');
  const acuRoute = routed && (ui.status?.connection?.choices || []).some(choice => selectedModel?.ref?.startsWith(choice.id + '/'));
  const state = !ready ? '检测中' : !agent.detected ? '未检测到' : agent.needsRepair ? '配置有变化' : !agent.connected ? '原生配置' : inApp ? '模型列表已配置' : acuRoute ? '已接入 ACU' : routed ? '本机路由已配置' : selectedModel ? '原来源已选' : '已接入';
  const disabled = !ready || !agent.detected || ui.applying || ui.accountBusy;
  const route = !ready ? '正在检测应用' : !agent.detected ? '安装并打开一次应用后，重新检测。' : agent.needsRepair ? '应用配置已变化，请检查后重新接入。' : !agent.connected ? '未接入 ACU · 保留应用当前设置' : inApp ? '已准备模型 · 在应用内选择通道' : acuRoute ? '所选默认模型 → ACU' : routed ? '所选默认模型 → 本机网关' : selectedModel ? '当前模型使用原生通道' : '已保存接入配置 · 在应用内选择';
  const routeButton = !agent.detected ? `<button class="route-toggle route-toggle-disabled" disabled title="等待检测此 Agent">${icon('link')}<span><strong>等待检测</strong><small>打开 Agent 后重新检测</small></span><span class="route-switch" aria-hidden="true"><span></span></span></button>` : agent.connected ? `<button class="route-toggle route-toggle-on" data-action="agent-toggle" data-agent-id="${agent.id}" aria-pressed="true" title="点击关闭${inApp ? '附加模型列表' : '本地接入'}" ${disabled ? 'disabled' : ''}><span class="route-toggle-icon">${icon('check')}</span><span><strong>${inApp ? '附加模型已开启' : 'ACU 接入已开启'}</strong><small>点击关闭${inApp ? '并预览移除项' : '并还原原配置'}</small></span><span class="route-switch" aria-hidden="true"><span></span></span></button>` : `<button class="route-toggle route-toggle-off" data-action="agent-toggle" data-agent-id="${agent.id}" aria-pressed="false" title="点击开启 ACU 接入" ${disabled || !activeChoice() ? 'disabled' : ''}><span class="route-toggle-icon">${icon('link')}</span><span><strong>ACU 接入未开启</strong><small>${!activeChoice() ? '先连接 ACU' : '点击查看推荐配置并开启'}</small></span><span class="route-switch" aria-hidden="true"><span></span></span></button>`;
  const restartNotice = ready && agent.id === 'workbuddy' && agent.restartRequired ? `<div class="agent-restart-notice"><span>重启后生效</span><button class="agent-restart-button" data-action="agent-restart" data-agent-id="${agent.id}">重启</button></div>` : '';
  const headerState = agent.id === 'workbuddy' && agent.connected ? `<div class="agent-header-actions"><span class="agent-state ${ready && !agent.needsRepair ? 'route-on' : ''}">已连接</span>${restartNotice}</div>` : `<span class="agent-state ${ready && agent.connected && !agent.needsRepair ? 'route-on' : ''}">${state}</span>`;
  return `<article class="agent-card compact-card routing-card ${agent.connected ? 'is-configured' : ''}" data-agent="${agent.id}"><div class="agent-card-top"><div class="agent-identity"><span class="agent-monogram ${agent.id}" aria-hidden="true">${({ codex: 'C', claude: '✳', workbuddy: 'W', hermes: 'H' })[agent.id] || 'A'}</span><h3>${escapeHTML(agent.name)}</h3></div>${headerState}</div>${routeButton}
    ${!ready || !agent.detected || agent.needsRepair ? `<p class="agent-route">${route}</p>` : ''}
    ${agentModelSummary(agent, agent.connected ? `<button class="button secondary advanced-link" data-action="agent" data-agent-id="${agent.id}" ${disabled ? 'disabled' : ''}>高级配置</button>` : '')}<div class="agent-routing-actions">${ready && agent.restartRequired && agent.id !== 'workbuddy' ? `<button class="agent-restart-button" data-action="agent-restart" data-agent-id="${agent.id}">重新打开 ${escapeHTML(agent.name)}</button>` : ''}</div></article>`;
}

async function setAgentModel(id, value, field = 'model') {
  if (ui.applying || ui.accountBusy || ui.busy || !ui.status?.ready) return;
  const agent = ui.status.agents?.find(current => current.id === id);
  const role = agent?.roles?.find(current => current.key === field);
  const allowed = agent?.modelOptions?.some(option => option.value === value) || agent?.catalog?.models?.some(model => model.id === value) || role?.value === value;
  if (!agent?.connected || !names[id] || !allowed) { toast('模型列表已变化，请刷新后重试'); return; }
  if ((field === 'model' ? agent.model : role?.value) === value) return;
  ui.applying = true;
  ui.progress = '正在切换模型…';
  render();
  try {
    const result = await request('/api/set', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ agent: id, field, value }) });
    const updated = result.agents?.find(current => current.id === id);
    if (!updated?.wired || updated.drift || !updated.fields?.some(current => current.key === field && current.value === value)) throw new Error('未确认模型已生效，请打开高级配置检查');
    toast(`${names[id]} ${field === 'model' ? '默认模型' : '子任务模型'}已保存；如有重启提示，请重新打开 Agent。`);
  } catch (error) { toast(error.message); }
  finally { ui.applying = false; await refresh(); }
}

async function checkForUpdate() {
  if (!ui.status?.update?.supported || ui.applying) return;
  ui.applying = true;
  try { await request('/runtime/update/check', { method: 'POST' }); }
  catch (error) { toast(error.message); }
  finally { ui.applying = false; await refresh(); }
}

function installRuntimeUpdate() {
  const release = ui.status?.update?.release;
  if (!release || ui.applying) return;
  const notes = escapeHTML(release.notes || '修复问题并更新兼容的内置内核。').replace(/\n/g, '<br>');
  showModal(`更新到 ACU Runtime ${escapeHTML(release.version)}`, `<p>将一并更新 Runtime 和随包提供的兼容内核。已保存的 Key、Agent 与本地配置会保留。</p><p>${notes}</p><p>更新完成后 Runtime 将自动重启，可能短暂中断正在处理的请求。</p>`, '立即更新', async () => {
    ui.applying = true;
    try { await request('/runtime/update/install', { method: 'POST' }); ui.applying = false; await refresh(); }
    catch (error) { ui.applying = false; toast(error.message); await refresh(); }
  });
}

async function reopenAgent(id) {
  const agent = ui.status?.agents?.find(item => item.id === id);
  if (!agent || ui.applying || ui.accountBusy) return;
  ui.applying = true;
  render();
  try {
    await requestKernel(`/kernel/api/agents/restart/${encodeURIComponent(id)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
    // Magpie restarts the app-server asynchronously. Read the Runtime status
    // directly while waiting; refresh() intentionally ignores requests while
    // an apply operation is active.
    let latest = null;
    for (let attempt = 0; attempt < 8; attempt += 1) {
      await new Promise(resolve => setTimeout(resolve, attempt ? 750 : 350));
      latest = await request('/runtime/status');
      ui.status = latest;
      const current = latest.agents?.find(item => item.id === id);
      if (!current?.restartRequired) break;
    }
    const current = latest?.agents?.find(item => item.id === id);
    if (current?.restartRequired) throw new Error(`${agent.name} 尚未完成重启，请保存工作后手动重新打开`);
    toast(`${agent.name} 已重启`);
  } catch (error) {
    toast(error.message || `无法重启 ${agent.name}，请关闭后重新打开`);
  } finally {
    ui.applying = false;
    render();
  }
}

function settingsPage() {
  const update = ui.status?.update || {};
  const updateLabel = ({ idle: `当前版本 ${update.current || ui.status?.version || ''}`, checking: '正在检查更新…', current: '已是最新版本', available: `发现 ACU Runtime ${update.release?.version || ''}`, downloading: update.total ? `正在下载 ${Math.round(update.downloaded / update.total * 100)}%` : '正在下载更新…', restarting: '正在更新并重新启动…', error: update.message || '检查更新失败' })[update.phase || 'idle'];
  return `${header('', '设置', '按你的习惯使用 ACU。')}
    <section class="panel settings-panel"><h2>图片与搜索</h2><div class="setting-row"><div><strong>常用能力</strong><p>为 Agent 准备识图、生图与搜索工具，随时更换对应模型。</p></div><button class="button secondary" data-action="capabilities" ${!ui.status?.ready || !activeChoice() ? 'disabled' : ''}>管理能力</button></div></section>
    <section class="panel settings-panel"><h2>外观</h2><div class="setting-row"><div><strong>跟随系统</strong><p>自动适配浅色与深色模式。</p></div><span class="theme-readout"><span class="theme-swatch" aria-hidden="true"></span><span data-system-theme>跟随系统</span></span></div></section>
    <section class="panel settings-panel"><h2>窗口与托盘</h2><div class="setting-row"><div><strong>关闭窗口后继续运行</strong><p>点击托盘图标打开快捷面板，随时查看或调整 Agent。</p></div>${ui.status?.desktop ? '<button class="button secondary" data-action="show-panel">打开快捷面板</button>' : ''}</div></section>
    ${ui.status?.desktop ? `<section class="panel settings-panel"><h2>应用更新</h2><div class="setting-row"><div><strong>ACU Runtime ${escapeHTML(update.current || ui.status?.version || '')}</strong><p role="status">${escapeHTML(update.supported ? updateLabel : '此平台暂未提供应用内更新安装包')}${update.kernel ? ` · 内置内核 ${escapeHTML(update.kernel)}` : ''}</p>${update.phase === 'available' ? '<p>Runtime 与兼容内核将一起更新；本机 Key 和 Agent 配置会保留。</p>' : ''}</div><div class="update-actions">${update.phase === 'available' ? '<button class="button primary" data-action="install-update">更新并重启</button>' : ''}<button class="button secondary" data-action="check-update" ${(!update.supported || ['checking', 'downloading', 'restarting'].includes(update.phase)) ? 'disabled' : ''}>${update.phase === 'checking' ? '检查中…' : '检查更新'}</button></div></div>${update.phase === 'error' ? '<p class="inventory-warning">更新失败；当前安装未变更。</p>' : ''}</section>` : ''}
    ${ui.status?.desktop ? '<div class="desktop-exit"><span>退出应用可能中断正在经 ACU 处理的请求。</span><button class="button secondary" data-action="quit">退出 ACU Runtime</button></div>' : ''}`;
}

function render() {
  if (ui.frame) return;
  if (['agents', 'connection', 'tools'].includes(ui.page)) {
    ui.connectionOpen ||= ui.page === 'connection';
    ui.page = 'overview';
  }
  if (ui.page === 'overview' && document.getElementById('home-workbench')) {
    const connection = document.getElementById('home-connection');
    const focusedID = document.activeElement?.id;
    if (connection) connection.innerHTML = compactConnection();
    document.getElementById('home-preparation').innerHTML = kernelBanner();
    const workbench = document.getElementById('home-workbench');
    {
      const guideOpen = workbench.querySelector('.welcome-help')?.open;
      workbench.innerHTML = usageGuide() + agentsPage();
      const guide = workbench.querySelector('.welcome-help');
      if (guide && guideOpen) guide.open = true;
    }
    if (focusedID) document.getElementById(focusedID)?.focus({ preventScroll: true });
    const error = document.getElementById('home-error');
    if (error) { error.textContent = ui.error; error.hidden = !ui.error; }
    return;
  }
  queueMicrotask(() => window.dispatchEvent(new Event('acu:render')));
  app.innerHTML = `${navigation()}<div class="main-shell"><header class="topbar"><span>ACU Runtime <span class="top-divider">/</span> ${ui.page === 'settings' ? '设置' : '工作台'}</span><span class="top-status">${icon('shield')}本机配置</span></header><main class="content"><div id="home-error" role="alert" class="error-banner" ${ui.error ? '': 'hidden'}>${escapeHTML(ui.error)}</div>${ui.page === 'settings' ? settingsPage() : overviewPage()}</main></div>`;
}

function closeOverlay() {
  if (ui.capabilities?.saving) return;
  ui.capabilities = null;
  ui.inventory = null;
  ui.inventoryQuery = '';
  ui.setup = null;
  ui.disconnect = null;
  overlay.innerHTML = '';
  document.body.classList.remove('overlay-open');
}

function setupToolsControl(setup) {
  return `<div class="setup-tools"><label class="switch-row"><span>同时配置图片与搜索工具</span><input id="setup-tools" type="checkbox" ${setup.tools ? 'checked' : ''}></label><p>自动为所选应用准备识图、生图和搜索。保留已有模型和工具设置，首次配置可能需要几分钟。</p></div>`;
}

async function openCapabilities() {
  const choice = activeChoice();
  if (!ui.status?.ready || !choice || ui.applying) return;
  const current = { provider: choice.id, loading: true, draft: {}, error: '' };
  ui.capabilities = current;
  renderCapabilities();
  try {
    current.view = await request(`/runtime/capabilities?provider=${encodeURIComponent(choice.id)}`);
  } catch (error) { current.error = error.message; }
  if (ui.capabilities !== current) return;
  current.loading = false;
  renderCapabilities();
}

function capabilityAgentState(view, id) {
  const tools = view.tools.filter(tool => tool.agents?.includes(id));
  const count = ['acu-vision', 'acu-image', 'search'].filter(name => tools.some(tool => name === 'search' ? ['acu-search', 'acu-free-search'].includes(tool.name) : tool.name === name)).length;
  return { count, custom: tools.some(tool => tool.custom) };
}

function renderCapabilities() {
  const current = ui.capabilities;
  if (!current) return;
  const view = current.view;
  const select = (field, label, models) => {
    const value = current.draft[field] ?? view[field];
    const options = [...models];
    if (value && value !== 'off' && !options.some(model => model.id === value)) options.unshift({ id: value, name: `${value}（当前来源）` });
    return `<label class="field-label" for="capability-${field}">${label}</label><select id="capability-${field}" data-capability="${field}" ${current.saving ? 'disabled' : ''}><option value="" disabled ${!value ? 'selected' : ''}>由一键配置选择推荐模型</option>${options.map(model => `<option value="${escapeHTML(model.id)}" ${value === model.id ? 'selected' : ''}>${escapeHTML(model.name || model.id)}</option>`).join('')}<option value="off" ${value === 'off' ? 'selected' : ''}>关闭此补充能力</option></select>`;
  };
  const agents = (ui.status?.agents || []).filter(agent => agent.detected);
  const missing = view ? agents.filter(agent => capabilityAgentState(view, agent.id).count < 3) : [];
  const body = view ? `<p class="capability-intro">选择识图和生图模型，各应用共用。保存模型设置不会重新配置工具。</p><div class="capability-fields">${select('vision', '识图模型', view.visionModels)}${select('imageGen', '生图模型', view.imageModels)}</div><div class="capability-search-status"><strong>联网搜索</strong><span>${view.searchReady ? '使用已设置的搜索服务' : view.freeSearchConfigured ? '使用免费搜索' : '待配置'}</span></div><div class="capability-agent-list"><div class="capability-list-heading"><strong>应用工具</strong>${missing.length ? `<button class="text-button" data-configure-capabilities="missing" ${current.saving ? 'disabled' : ''}>${current.retry ? '重试未完成项' : '配置未完成项'}</button>` : ''}</div>${agents.map(agent => {
    const state = capabilityAgentState(view, agent.id);
    return `<div class="capability-agent-row"><div><strong>${escapeHTML(agent.name)}</strong><span class="capability-status ${state.count === 3 ? 'complete' : ''}">${state.count === 3 ? '已配置 ✓' : state.count ? '待补齐' : '待配置'}${state.custom ? ' · 自定义设置' : ''}</span></div><button class="text-button" data-configure-capabilities="${escapeHTML(agent.id)}" ${current.saving ? 'disabled' : ''}>${state.count === 3 ? '重新配置' : '配置'}</button></div>`;
  }).join('') || '<p>暂未检测到应用，安装后返回工作台重新检测。</p>'}</div><p class="capability-intro capability-help">包含识图、生图和搜索工具。新增工具后请重新打开应用；首次使用按应用提示确认。已配置表示配置已写入。</p><details class="capability-advanced"><summary>高级设置</summary><p>模型来源、搜索服务和工具权限由内核统一管理。</p><div class="capability-advanced-actions"><button class="button secondary" data-capability-native="images" ${current.saving ? 'disabled' : ''}>模型与图片设置</button><button class="button secondary" data-capability-native="search" ${current.saving ? 'disabled' : ''}>搜索服务</button><button class="button secondary" data-capability-native="library" ${current.saving ? 'disabled' : ''}>工具与应用分配</button></div></details>` : `<p>${current.loading ? '正在读取配置…' : '配置暂未读取，请关闭后重试。'}</p>`;
  overlay.innerHTML = `<div class="scrim"><section class="modal capability-modal recommendation-modal" role="dialog" aria-modal="true" aria-labelledby="capability-title"><div class="recommendation-header"><button class="icon-button modal-close" data-close aria-label="关闭" ${current.saving ? 'disabled' : ''}>${icon('close')}</button><h2 id="capability-title">图片与搜索</h2></div><div class="recommendation-body">${current.error ? `<p class="error-banner" role="alert">${escapeHTML(current.error)}</p>` : ''}${current.progress ? `<p class="capability-progress" role="status">${escapeHTML(current.progress)}</p>` : ''}${body}</div><div class="recommendation-footer"><div class="modal-actions"><button class="button secondary" data-close ${current.saving ? 'disabled' : ''}>完成</button><button class="button primary" data-save-capabilities ${!view || current.saving || !Object.keys(current.draft).length ? 'disabled' : ''}>${current.saving === 'models' ? '保存中…' : '保存模型设置'}</button></div></div></section></div>`;
  document.body.classList.add('overlay-open');
}

async function saveCapabilities() {
  const current = ui.capabilities;
  if (!current?.view || current.saving || !Object.keys(current.draft).length) return;
  current.saving = 'models';
  current.error = '';
  renderCapabilities();
  try {
    const body = { provider: current.provider, modelsOnly: true, ...current.draft };
    current.view = await request('/runtime/capabilities', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    current.draft = {};
    toast('模型设置已保存');
  } catch (error) { current.error = error.message; }
  finally { current.saving = false; if (ui.capabilities === current) renderCapabilities(); }
}

async function configureCapabilities(target) {
  const current = ui.capabilities;
  if (!current?.view || current.saving) return;
  const detected = (ui.status?.agents || []).filter(agent => agent.detected);
  const agents = target === 'missing' ? detected.filter(agent => capabilityAgentState(current.view, agent.id).count < 3).map(agent => agent.id) : detected.filter(agent => agent.id === target).map(agent => agent.id);
  if (!agents.length) return;
  current.saving = 'tools'; current.error = ''; current.retry = false;
  current.progress = '正在准备图片与搜索工具，首次配置可能需要几分钟…';
  renderCapabilities();
  try {
    current.view = await request('/runtime/capabilities', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ provider: current.provider, agents }) });
    toast('工具已配置，请重新打开应用');
  } catch (error) {
    current.error = error.message; current.retry = true;
    try { current.view = await request(`/runtime/capabilities?provider=${encodeURIComponent(current.provider)}`); } catch { /* keep the last known state and allow retry */ }
  } finally { current.saving = false; current.progress = ''; if (ui.capabilities === current) renderCapabilities(); }
}

function disconnectLabel(id) {
  return id === 'workbuddy' ? '移除附加模型' : '关闭 ACU 接入';
}

function renderDisconnect() {
  const current = ui.disconnect;
  if (!current) return;
  const agent = (ui.status?.agents || []).find(item => item.id === current.agentId);
  const changes = current.preview?.changes || [];
  const previewLines = changes.flatMap(change => Array.isArray(change.lines) ? change.lines.map(line => ({ ...line, path: change.path })) : [change]);
  const preview = previewLines.length ? previewLines.slice(0, 36).map(line => `<li><span class="preview-op">${escapeHTML(line.op || '·')}</span><code>${escapeHTML(line.path ? `${line.path}: ` : '')}${escapeHTML(line.text || line.was || '')}</code></li>`).join('') : '<li class="preview-empty">将由内核还原此 Agent 之前保存的配置。</li>';
  const more = previewLines.length > 36 ? `<p class="preview-more">还有 ${previewLines.length - 36} 项变化，完整内容由内核确认。</p>` : '';
  const error = current.error ? `<div class="disconnect-error" role="alert">${escapeHTML(current.error)}<button class="text-button" data-retry-disconnect>重试</button></div>` : '';
  overlay.innerHTML = `<div class="scrim"><section class="modal disconnect-modal" role="dialog" aria-modal="true" aria-labelledby="disconnect-title"><button class="icon-button modal-close" data-close aria-label="关闭">${icon('close')}</button><div class="recommendation-kicker">${agent?.id === 'workbuddy' ? '模型列表' : '路由开关'}</div><h2 id="disconnect-title">${escapeHTML(agent?.name || names[current.agentId] || 'Agent')} ${disconnectLabel(current.agentId)}</h2><p class="disconnect-summary">${agent?.id === 'workbuddy' ? '移除本地接入管理的附加模型（含 ACU 与其他来源）；实际移除项以下方预览为准。WorkBuddy 内建模型仍在原应用管理。' : '关闭此 Agent 的 ACU 接入，并按内核预览还原接入前配置。ACU、其他 Agent 和 API Key 不受影响。'}</p>${error}<div class="disconnect-preview"><div class="preview-heading"><strong>将要还原的配置</strong>${current.loading ? '<span>读取中…</span>' : `<span>${previewLines.length} 项</span>`}</div><ul>${current.loading ? '<li class="preview-empty">正在读取内核还原预览…</li>' : preview}</ul>${more}</div><div class="modal-actions"><button class="button secondary" data-close>取消</button><button class="button primary danger-button" data-confirm-disconnect ${current.loading || current.failed ? 'disabled' : ''}>${current.loading ? '读取中…' : disconnectLabel(current.agentId)} ${icon('arrow')}</button></div></section></div>`;
  document.body.classList.add('overlay-open');
}

async function openDisconnectDialog(agentId) {
  if (ui.applying || ui.accountBusy || !ui.status?.ready) return;
  ui.disconnect = { agentId, loading: true, failed: false, preview: null, error: '' };
  renderDisconnect();
  try {
    const preview = await request(`/kernel/api/agents/preview/${encodeURIComponent(agentId)}`);
    if (!ui.disconnect || ui.disconnect.agentId !== agentId) return;
    if (preview.error) throw new Error(preview.error);
    ui.disconnect.preview = preview;
  } catch (error) {
    if (!ui.disconnect || ui.disconnect.agentId !== agentId) return;
    ui.disconnect.error = `无法读取还原预览：${error.message}`;
    ui.disconnect.failed = true;
  } finally {
    if (ui.disconnect?.agentId === agentId) { ui.disconnect.loading = false; renderDisconnect(); }
  }
}

async function confirmDisconnect() {
  const current = ui.disconnect;
  if (!current || current.loading || current.failed || ui.applying) return;
  ui.applying = true;
  renderDisconnect();
  try {
    const result = await request(`/kernel/api/agents/disconnect/${encodeURIComponent(current.agentId)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
    const disconnected = Array.isArray(result.agents) ? result.agents.find(agent => agent.id === current.agentId) : null;
    if (!disconnected || disconnected.wired) throw new Error('内核未确认已关闭，请重新检测');
    closeOverlay();
    toast(`${names[current.agentId] || 'Agent'} 已${disconnectLabel(current.agentId)}`);
  } catch (error) {
    if (ui.disconnect) { ui.disconnect.error = `操作未完成：${error.message}`; ui.disconnect.failed = true; }
  } finally { ui.applying = false; if (ui.disconnect) renderDisconnect(); await refresh(); }
}

function showModal(title, body, button, onConfirm) {
  overlay.innerHTML = `<div class="scrim"><section class="modal confirmation-modal" role="dialog" aria-modal="true" aria-labelledby="dialog-title"><button class="icon-button modal-close" data-close aria-label="关闭">${icon('close')}</button><div class="modal-glyph">${icon('shield')}</div><h2 id="dialog-title">${title}</h2><div class="modal-body">${body}</div><div class="modal-actions"><button class="button secondary" data-close>取消</button><button class="button primary" id="confirm-action">${button}</button></div></section></div>`;
  document.body.classList.add('overlay-open');
  document.getElementById('confirm-action').onclick = async () => { closeOverlay(); await onConfirm(); };
  overlay.querySelector('[data-close]').focus();
}

async function connectKernel() {
  if (ui.status?.starting || ui.busy) return;
  ui.autoStarted = true;
  ui.busy = true;
  render();
  try {
    ui.status = await request('/runtime/kernel/start', { method: 'POST' });
    ui.error = ui.status.error || '';
  } catch (error) { ui.error = error.message; }
  finally {
    ui.busy = false;
    render();
    if (ui.status?.starting) refresh.timer = setTimeout(() => refresh(), 1000);
  }
}

async function applyAgents() {
  if (ui.applying || ui.accountBusy || ui.busy || !ui.status?.ready) return;
  await refresh();
  const choice = activeChoice();
  const agents = (ui.status?.agents || []).filter(agent => ui.selected?.has(agent.id) && agent.detected && names[agent.id]);
  if (!choice || !choice.models?.length || !agents.length) { toast('请先选择可用的 ACU Key'); return; }
  ui.setup = { provider: choice.id, model: recommendedModels(choice)[0] || '', auxiliary: false, tools: true, agents: new Set(agents.map(agent => agent.id)) };
  renderSetup();
}

async function configureAgent(id) {
  if (ui.applying || ui.accountBusy || ui.busy || !ui.status?.ready) return;
  await refresh();
  const agent = (ui.status?.agents || []).find(current => current.id === id);
  const choice = activeChoice();
  if (!agent?.detected || !choice?.models?.length) { toast('请先连接可用的 ACU Key'); return; }
  const recommendation = agentRecommendation(agent, choice);
  ui.setup = { mode: 'single', provider: choice.id, model: recommendation.recommendedModel, auxiliary: false, tools: true, agents: new Set([id]), agentId: id, loading: true };
  const setup = ui.setup;
  renderSetup();
  try { setup.settings = await request('/api/settings'); }
  catch { setup.settings = null; }
  if (ui.setup !== setup) return;
  setup.loading = false;
  renderSetup();
}

function renderSetup() {
  const setup = ui.setup;
  if (!setup) return;
  const choice = ui.status?.connection?.choices?.find(current => current.id === setup.provider);
  if (!choice) { closeOverlay(); toast('所选 Key 已删除，请重新选择'); return; }
  const agents = (ui.status.agents || []).filter(agent => agent.detected && names[agent.id]);
  if (setup.mode === 'single') {
    const agent = agents.find(current => current.id === setup.agentId) || (ui.status.agents || []).find(current => current.id === setup.agentId);
    const recommendation = agentRecommendation(agent, choice, setup.model, setup.settings);
    const currentModels = recommendation.nativeModels.length ? recommendation.nativeModels.map(escapeHTML).join('、') : '按应用当前登录方式与配置保留';
    const addedModels = recommendation.availableModels.length ? recommendation.availableModels.map(escapeHTML).join('、') : '此 Agent 暂无可选的 ACU 模型';
    const rows = recommendation.capabilities.map(row => [...row]);
    if (setup.tools) {
      rows[0][1] = `${setup.model} 的识图取决于模型能力；同时添加 Magpie 识图 MCP，保留已选模型。`;
      rows[2][1] += ' 添加 Magpie 搜索 MCP；未连接搜索来源时仍需配置。';
      rows[3][1] = rows[3][1].replace('本次不新增 MCP。', '同时添加 Magpie 生图 MCP，首次使用推荐 ACU 生图模型。');
    }
    const capabilityRows = rows.map(([label, route, change]) => `<tr><th>${escapeHTML(label)}</th><td>${escapeHTML(route)}</td><td>${escapeHTML(change)}</td></tr>`).join('');
    const inApp = setup.agentId === 'workbuddy';
    const modelControl = inApp ? '<strong>在 WorkBuddy 任务内选择</strong><p>不替你设置统一默认模型</p>' : `<label class="field-label" for="setup-model">默认模型 · 推荐已选好</label><select id="setup-model" ${!recommendation.availableModels.length ? 'disabled' : ''}>${!recommendation.availableModels.length ? '<option>暂无兼容模型</option>' : recommendation.availableModels.map(model => `<option value="${escapeHTML(model)}" ${setup.model === model ? 'selected' : ''}>${escapeHTML(model)}</option>`).join('')}</select><p>${escapeHTML(agent.name)} → ${escapeHTML(setup.model || '待选择')} → ACU</p>`;
    overlay.innerHTML = `<div class="scrim"><section class="modal setup-modal recommendation-modal" role="dialog" aria-modal="true" aria-labelledby="setup-title"><div class="recommendation-header"><button class="icon-button modal-close" data-close aria-label="关闭">${icon('close')}</button><div class="recommendation-kicker">推荐配置</div><h2 id="setup-title">${escapeHTML(agent.name)} 接入 ACU</h2><p class="setup-key-label">${escapeHTML(choice.name || 'ACU')} · ${escapeHTML(choice.masked || '已选 Key')}</p></div><div class="recommendation-body"><p class="recommendation-summary">${escapeHTML(recommendation.summary)}</p><div class="recommendation-route">${modelControl}</div><p class="recommendation-impact">${escapeHTML(recommendation.change)}</p><details class="recommendation-models"><summary>查看原有与 ACU 模型 · ${recommendation.availableModels.length} 个可选</summary><dl class="recommendation-list"><div><dt>原有模型</dt><dd>${currentModels}</dd></div><div><dt>ACU 模型</dt><dd>${addedModels}</dd></div><div><dt>保持不变</dt><dd>${escapeHTML(recommendation.retained)}</dd></div></dl><p>${inApp ? '确认后显示推荐模型并收起其他 ACU 模型，当前使用项保留；可在全部模型中恢复。' : '确认后显示推荐模型，收起其他 ACU 模型；当前使用的模型保留。可在全部模型中恢复显示。'}</p></details><div class="recommendation-block"><h3>接入后，常用能力怎么用</h3>${setupToolsControl(setup)}<p class="capability-caption">${setup.loading ? '正在读取现有配置…' : '已有配置 ≠ 已验证可用；保留显式模型选择，新增工具由内核写入。'}</p><table class="capability-table"><thead><tr><th>能力</th><th>处理方式 / 来源</th><th>配置状态</th></tr></thead><tbody>${capabilityRows}</tbody></table></div></div><div class="recommendation-footer"><p class="setup-footnote">仅配置 ${escapeHTML(agent.name)}；需要重新打开时会提示。</p><div class="modal-actions"><button class="button secondary" data-close>取消</button><button class="button primary" data-confirm-setup ${setup.loading || !recommendation.availableModels.length ? 'disabled' : ''}>${inApp ? '同意并添加模型' : '同意并接入'} ${icon('arrow')}</button></div></div></section></div>`;
  } else {
    overlay.innerHTML = `<div class="scrim"><section class="modal setup-modal recommendation-modal" role="dialog" aria-modal="true" aria-labelledby="setup-title"><div class="recommendation-header"><button class="icon-button modal-close" data-close aria-label="关闭">${icon('close')}</button><h2 id="setup-title">一键配置</h2><p class="setup-key-label">${escapeHTML(choice.name || 'ACU')} · ${escapeHTML(choice.masked)}</p></div><div class="recommendation-body"><p class="recommendation-summary">为所选应用接入 ACU，准备识图、生图和免费搜索。已接入应用保留现有模型与工具设置。</p><div class="setup-app-options">${agents.map(agent => `<label class="setup-app-option"><input type="checkbox" data-setup-agent="${agent.id}" ${setup.agents.has(agent.id) ? 'checked' : ''}><span>${escapeHTML(agent.name)}</span><small>${agent.connected ? '保留现有模型' : '首次接入'}</small></label>`).join('')}</div><details class="setup-adjustments"><summary>调整推荐模型与配置范围</summary><label class="field-label" for="setup-model">首次接入的主模型</label><select id="setup-model">${recommendedModels(choice).map(model => `<option value="${escapeHTML(model)}" ${setup.model === model ? 'selected' : ''}>${escapeHTML(model)}</option>`).join('')}</select><label class="setup-app-option"><input id="setup-replace" type="checkbox" ${setup.replace ? 'checked' : ''}><span>同时替换已接入应用的主模型</span></label>${setupToolsControl(setup)}</details><p class="setup-footnote">首次准备工具可能需要几分钟，完成后按提示重新打开应用。</p></div><div class="recommendation-footer"><div class="modal-actions"><button class="button secondary" data-close>取消</button><button class="button primary" data-confirm-setup ${!setup.agents.size || !setup.model ? 'disabled' : ''}>确认配置 ${setup.agents.size ? `(${setup.agents.size})` : ''}</button></div></div></section></div>`;
  }
  document.body.classList.add('overlay-open');
}

async function applyRecommendedVisibility(agentId, provider, selectedModel) {
  const catalog = await requestKernel(`/kernel/api/agent-models/${encodeURIComponent(agentId)}`);
  if (!Array.isArray(catalog?.models)) throw new Error('接入已保存，但无法读取模型清单，请重试推荐配置');
  const prefix = provider + '/';
  const models = catalog.models.map(model => {
    if (!model.id.startsWith(prefix) || model.own || model.inUse || model.id === prefix + selectedModel) return model;
    return { ...model, hidden: !recommendedACUModels.includes(model.id.slice(prefix.length)) };
  });
  const body = catalog.only
    ? { shown: models.filter(model => !model.hidden && !(model.own && catalog.ownHidden)).map(model => model.id) }
    : { hidden: models.filter(model => !model.own && model.hidden).map(model => model.id) };
  await requestKernel(`/kernel/api/agent-models/${encodeURIComponent(agentId)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
}

async function confirmSetup() {
  if (ui.applying || !ui.setup || ui.setup.loading || !ui.setup.model) return;
  const setup = { ...ui.setup, agents: [...ui.setup.agents] };
  closeOverlay();
  clearTimeout(refresh.timer);
  ui.applying = true;
  ui.results = [];
  try {
    const connectedAgents = [];
    for (const [index, id] of setup.agents.entries()) {
      ui.progress = `配置中 ${index + 1}/${setup.agents.length}`;
      render();
      try {
        const preserve = setup.mode !== 'single' && !setup.replace && ui.status?.agents?.some(agent => agent.id === id && agent.connected);
        const result = await request('/runtime/agents/configure', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ agent: id, provider: setup.provider, model: setup.model, auxiliary: setup.auxiliary, ...(preserve ? { preserve: true } : {}) }) });
        const configured = result.agents?.find(current => current.id === id);
        if (!configured?.wired || configured.drift) throw new Error('未确认配置成功，请检查设置');
        connectedAgents.push(id);
        if (!preserve) await applyRecommendedVisibility(id, setup.provider, setup.model);
        ui.selected?.delete(id);
        ui.results.push({ id, ok: true, message: preserve ? '已接入 · 保留现有模型' : setup.auxiliary && !result.runtimeSetup?.auxiliaryApplied ? '已配置 · 无可同步的辅助项' : '已配置' });
      } catch (error) { ui.results.push({ id, ok: false, message: error.message }); }
    }
    if (setup.tools && connectedAgents.length) {
      ui.progress = '正在准备图片与搜索…'; render();
      try {
        const capabilities = await request('/runtime/capabilities', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ provider: setup.provider, agents: connectedAgents }) });
        ui.results.push({ id: 'tools', ok: true, message: `工具已写入 · ${capabilities.searchReady ? '搜索来源已配置' : capabilities.freeSearchConfigured ? '免费搜索已配置' : '搜索待配置'} · 重新打开 Agent 后使用` });
      } catch (error) { ui.results.push({ id: 'tools', ok: false, message: `模型接入已完成，工具需重试：${error.message}` }); }
    }
  } catch (error) { toast(`模型目录刷新失败：${error.message}`); }
  finally { ui.applying = false; await refresh(); }
}

async function startAccountLogin(register) {
  if (ui.applying || ui.accountBusy || ui.status?.account?.pending || !ui.status?.desktop) return;
  const site = 'acucompute.com';
  ui.connectionOpen = false;
  app.innerHTML = '';
  ui.accountBusy = true;
  ui.error = '';
  render();
  try {
    await request('/runtime/account/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ site, register }) });
    ui.keySelection = null;
  } catch (error) { toast(error.message); }
  finally { ui.accountBusy = false; await refresh(); }
}

async function cancelAccountLogin() {
  try { await request('/runtime/account/cancel', { method: 'POST' }); }
  catch (error) { toast(error.message); }
  await refresh();
}

async function configureConnection(input) {
  if (ui.applying || ui.accountBusy || !ui.status?.ready) return;
  ui.accountBusy = true;
  ui.error = '';
  render();
  try {
    await request('/runtime/connection', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    ui.selected = null;
    ui.providerSelection = null;
    ui.page = 'overview';
    ui.connectionOpen = false;
    app.innerHTML = '';
    toast('ACU 已连接，模型已准备好。接下来配置常用 Agent。');
  } catch (error) { toast(error.message); }
  finally { ui.accountBusy = false; await refresh(); }
}

function openNative(view, agent = '', developer = false, tab = '', focus = '', action = '', extra = new URLSearchParams()) {
  if (!ui.status?.ready) { connectKernel(); return; }
    ui.frame = { view, agent, developer };
    const query = new URLSearchParams({ view, 'acu-view': view, 'acu-agent': agent, 'acu-developer': developer ? '1' : '0' });
    for (const key of ['req', 'provider', 'computer', 'period', 'day', 'newgroup', 'q']) if (extra.get(key)) query.set(key, extra.get(key));
    if (tab) query.set('tab', tab);
    if (focus) query.set('acu-focus', focus);
    if (action === 'disconnect' || action === 'models') query.set('acu-action', action);
    const capabilityLinks = view === 'agents' && agent ? `<nav class="native-context-tabs" aria-label="${escapeHTML(names[agent])} 功能配置"><span class="context-caption">${escapeHTML(names[agent])}</span><button class="button secondary" data-native-view="settings" data-native-tab="models" data-native-focus="imagesList">图片服务（共享）</button><button class="button secondary" data-native-view="settings" data-native-tab="models" data-native-focus="searchList">搜索服务（共享）</button><button class="button secondary" data-native-view="library">添加工具 / MCP</button></nav>` : '';
    app.innerHTML = `${navigation()}<div class="main-shell"><header class="topbar"><button class="text-button" data-action="back">← 返回工作台</button><span class="build-tag">配置保存在本机</span></header><div class="native-heading"><div><h1>${agent ? escapeHTML(names[agent]) + ' 配置' : ({ usage: '用量详情', routing: '路由详情', plugins: '插件与工具', masking: '内容保护', agents: 'Agent 模型配置', providers: '高级连接设置', settings: '共享能力服务', library: '工具目录' })[view] || '配置'}</h1><p>${['usage', 'routing'].includes(view) ? '本机请求记录；费用为估算，不代表 ACU 账单。' : action === 'disconnect' ? '确认前会显示要还原的配置；仅停止此 Agent 接入，不退出 ACU，也不删除已保存的 Key。' : view === 'providers' ? '管理其他模型来源或高级连接选项；ACU 登录与快速连接请使用首页。' : ['settings', 'library'].includes(view) ? '此处是共享服务与工具目录；按页面中的 Agent 选项配置，不会自动为所有应用启用。' : '调整此 Agent 的高级模型与工具配置；保存后按提示重新打开应用。'}</p></div><button class="button secondary" data-action="back">${view === 'providers' ? '返回首页，继续配置' : '返回并检查状态'} ${icon('arrow')}</button></div>${capabilityLinks}<iframe class="native-frame" title="${view === 'providers' ? '高级连接配置' : '高级配置'}" src="/kernel/?${query}" sandbox="allow-same-origin allow-scripts allow-forms allow-downloads allow-popups"></iframe></div>`;
}



app.addEventListener('click', event => {
  if (ui.applying || ui.accountBusy) return;
  const page = event.target.closest('[data-page]');
  if (page) { ui.frame = null; ui.page = page.dataset.page; if (['usage', 'routing'].includes(ui.page)) openNative(ui.page); else { app.innerHTML = ''; render(); } refresh(false, true); return; }
  const native = event.target.closest('[data-native-view]');
  if (native) { openNative(native.dataset.nativeView, '', false, native.dataset.nativeTab, native.dataset.nativeFocus); return; }
  const control = event.target.closest('[data-action]');
  if (!control) return;
  const actions = {
    'show-panel': () => request('/runtime/desktop/action', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'panel' }) }).catch(error => toast(error.message)),
    refresh: () => refresh(true), connect: connectKernel,
    'check-update': checkForUpdate, 'install-update': installRuntimeUpdate, 'agent-restart': () => reopenAgent(control.dataset.agentId),
    'manage-connection': () => { ui.connectionOpen = !ui.connectionOpen; app.innerHTML = ''; render(); },
    capabilities: openCapabilities,
    'configure-agent': () => configureAgent(control.dataset.agentId),
    'retry-setup': () => { ui.selected = new Set(ui.results.filter(result => !result.ok && names[result.id]).map(result => result.id)); applyAgents(); },
    'apply-agents': () => { if (!ui.selected?.size) ui.selected = new Set((ui.status?.agents || []).filter(agent => agent.detected).map(agent => agent.id)); applyAgents(); },
    'account-login': () => startAccountLogin(false), 'account-register': () => startAccountLogin(true),
    'account-cancel': cancelAccountLogin,
    'use-account-key': () => configureConnection({ id: Number(ui.keySelection ?? ui.status?.account?.selected) }),
    agent: () => openNative('agents', control.dataset.agentId),
    'model-inventory': () => openModelInventory(control.dataset.agentId),
    'agent-disconnect': () => openDisconnectDialog(control.dataset.agentId),
    'agent-toggle': () => { const id = control.dataset.agentId; const agent = ui.status?.agents?.find(item => item.id === id); if (agent?.connected) openDisconnectDialog(id); else configureAgent(id); },
    'native-models': () => openNative('agents'), 'native-providers': () => openNative('providers'),
    back: () => { ui.page = 'overview'; ui.frame = null; app.innerHTML = ''; refresh(); },
    quit: () => showModal('退出 ACU Runtime', '<p>可能中断正在经 ACU 处理的请求。只想收起窗口，请点击窗口关闭按钮。</p>', '退出应用', () => request('/runtime/desktop/quit', { method: 'POST' })),
  };
  actions[control.dataset.action]?.();
});
app.addEventListener('change', event => {
  if (event.target.dataset?.agentModel) { setAgentModel(event.target.dataset.agentModel, event.target.value, event.target.dataset.agentRole || 'model'); return; }
  if (event.target.id === 'quick-key') {
    const [kind, ...parts] = event.target.value.split(':');
    if (kind === 'account') { configureConnection({ id: Number(parts.join(':')) }); return; }
    ui.providerSelection = kind === 'local' ? parts.join(':') : '';
    render(); return;
  }
  if (event.target.id === 'account-key') { ui.keySelection = Number(event.target.value); render(); return; }
  const id = event.target.dataset?.selectAgent;
  if (!id || ui.applying || !names[id]) return;
  const agent = ui.status?.agents?.find(current => current.id === id);
  if (!activeChoice() || ui.accountBusy || !agent?.detected) return;
  if (!ui.selected) ui.selected = new Set();
  if (event.target.checked) ui.selected.add(id);
  else ui.selected.delete(id);
  render();
  document.querySelector(`[data-select-agent="${id}"]`)?.focus();
});
app.addEventListener('submit', event => {
  if (!event.target.matches('[data-connection-form]')) return;
  event.preventDefault();
  const values = new FormData(event.target);
  configureConnection({ base: values.get('base'), key: values.get('key') });
});
overlay.addEventListener('click', event => {
  const visibility = event.target.closest('[data-model-visibility]');
  if (visibility) { event.preventDefault?.(); toggleModelVisibility(visibility.dataset.agentId, visibility.dataset.modelId); return; }
  const move = event.target.closest('[data-model-move]');
  if (move) { event.preventDefault?.(); moveInventoryModel(move.dataset.agentId, move.dataset.modelId, move.dataset.modelMove); return; }
  const inventoryFlag = event.target.closest('[data-inventory-flag]');
  if (inventoryFlag) { toggleInventoryFlag(inventoryFlag.dataset.agentId, inventoryFlag.dataset.inventoryFlag); return; }
  const inventoryTab = event.target.closest('[data-inventory-tab]');
  if (inventoryTab) { ui.inventoryTab = inventoryTab.dataset.inventoryTab; ui.inventoryBulk = null; redrawInventory(); return; }
  const inventoryBulk = event.target.closest('[data-inventory-bulk]');
  if (inventoryBulk) { bulkInventory(inventoryBulk.dataset.agentId, inventoryBulk.dataset.inventoryBulk); return; }
  if (event.target.closest('[data-save-capabilities]')) { saveCapabilities(); return; }
  const configureTools = event.target.closest('[data-configure-capabilities]');
  if (configureTools) { configureCapabilities(configureTools.dataset.configureCapabilities); return; }
  const advanced = event.target.closest('[data-capability-native]');
  if (advanced) { if (ui.capabilities?.saving) return; const view = advanced.dataset.capabilityNative; closeOverlay(); if (view === 'library') openNative('library'); else openNative('settings', '', false, 'models', view === 'search' ? 'searchList' : 'imagesList'); return; }
  if (event.target.closest('[data-confirm-setup]')) { confirmSetup(); return; }
  if (event.target.closest('[data-confirm-disconnect]')) { confirmDisconnect(); return; }
  if (event.target.closest('[data-retry-disconnect]')) { openDisconnectDialog(ui.disconnect?.agentId); return; }
  if (event.target.matches('.scrim') || event.target.closest('[data-close]')) closeOverlay();
});
overlay.addEventListener('input', event => {
  if (event.target.id !== 'inventory-search') return;
  ui.inventoryQuery = event.target.value;
  filterModelInventory();
});

overlay.addEventListener('change', event => {
  if (ui.capabilities) {
    const key = event.target.dataset?.capability;
    if (key) ui.capabilities.draft[key] = event.target.type === 'checkbox' ? event.target.checked : event.target.value;
    const save = overlay.querySelector('[data-save-capabilities]');
    if (save) save.disabled = !Object.keys(ui.capabilities.draft).length || !!ui.capabilities.saving;
    return;
  }
  if (!ui.setup) return;
  if (event.target.id === 'setup-tools') { ui.setup.tools = event.target.checked; renderSetup(); }
  if (event.target.id === 'setup-model') {
    ui.setup.model = event.target.value;
    if (ui.setup.mode === 'single') {
      renderSetup();
      overlay.querySelector('#setup-model')?.focus();
    }
  }
  if (event.target.id === 'setup-auxiliary') ui.setup.auxiliary = event.target.checked;
  if (event.target.id === 'setup-replace') ui.setup.replace = event.target.checked;
  const agent = event.target.dataset?.setupAgent;
  if (agent) { if (event.target.checked) ui.setup.agents.add(agent); else ui.setup.agents.delete(agent); renderSetup(); }
});
document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(false, true); });
document.addEventListener('keydown', event => { if (event.key === 'Escape') closeOverlay(); });
document.addEventListener('keydown', event => {
  if (event.key !== 'Tab' || !overlay.firstElementChild) return;
  const controls = [...overlay.querySelectorAll('button:not([disabled]),a[href],input,select')];
  if (!controls.length) return;
  if (event.shiftKey && document.activeElement === controls[0]) { event.preventDefault(); controls.at(-1).focus(); }
  if (!event.shiftKey && document.activeElement === controls.at(-1)) { event.preventDefault(); controls[0].focus(); }
});
window.addEventListener?.('message', event => {
  const frame = document.querySelector('iframe');
  if (event.origin !== globalThis.location?.origin || event.source !== frame?.contentWindow || event.data?.type !== 'acu:window') return;
  if (event.data.action !== 'main') return;
  const query = new URLSearchParams(event.data.query || '');
  const view = query.get('view');
  if (view === 'overview' || !view) { ui.frame = null; ui.page = 'overview'; render(); }
  else if (nativeViews.has(view)) openNative(view, '', false, query.get('tab') || '', '', '', query);
});
refresh();
