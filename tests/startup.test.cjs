const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../web/app.js'), 'utf8');
const connected = () => ({ known: true, configured: true, provider: 'acu-runtime', choices: [{ id: 'acu-runtime', name: 'ACU', masked: 'sk-***1234', models: ['acu-auto', 'gpt-6.1-sol'] }] });
const codex = () => ({ id: 'codex', name: 'Codex', detected: true, canConnect: true, modelOptions: [{ value: 'acu-runtime/acu-auto', ref: 'acu-runtime/acu-auto', label: 'acu-auto' }, { value: 'acu-runtime/gpt-6.1-sol', ref: 'acu-runtime/gpt-6.1-sol', label: 'gpt-6.1-sol' }, { value: 'chatgpt', label: 'ChatGPT' }] });

async function setup(state, handler) {
  const calls = [];
  const timers = [];
  const listeners = { app: {}, overlay: {}, document: {} };
  const elements = Object.fromEntries(['app', 'overlay', 'toast'].map(name => [name, { innerHTML: '', classList: { add() {}, remove() {} }, querySelector: () => ({ focus() {} }), addEventListener(event, callback) { if (listeners[name]) listeners[name][event] = callback; } }]));
  const context = vm.createContext({
    document: {
      getElementById: name => elements[name],
      querySelector: () => ({ content: 'isolated-session', focus() {} }),
      addEventListener: (event, callback) => { listeners.document[event] = callback; },
      body: { classList: { add() {}, remove() {} } },
    },
    window: { dispatchEvent() {} }, Event, URLSearchParams, queueMicrotask,
    setTimeout: (callback, milliseconds) => { timers.push({ callback, milliseconds }); return timers.length; }, clearTimeout() {},
    fetch: async (route, options = {}) => {
      calls.push({ route, method: options.method || 'GET', body: options.body });
      const result = await handler?.(route, options);
      if (result) return { ok: result.ok !== false, status: result.status || 200, json: async () => result.body };
      if (route.startsWith('/kernel/api/agent-models/') && !options.method) return { ok: true, json: async () => inventory() };
      if (route === '/runtime/kernel/start') Object.assign(state, { starting: true, phase: '正在下载内核', downloaded: 50, downloadTotal: 100 });
      return { ok: true, json: async () => ({ agents: [], kernelFound: false, connection: connected(), ...state }) };
    },
  });
  vm.runInContext(source, context);
  await new Promise(resolve => setImmediate(resolve));
  const change = target => listeners.app.change({ target });
  return { calls, timers, elements, listeners, run: command => vm.runInContext(command, context), select: (id, checked) => change({ dataset: { selectAgent: id }, checked }), change, refresh: () => vm.runInContext('refresh()', context) };
}

const posts = instance => instance.calls.filter(call => call.method === 'POST' && call.route !== '/kernel/api/provider/models');

const inventory = () => ({ models: [
  { id: 'acu-runtime/acu-auto', name: 'acu-auto', group: 'ACU', inUse: true },
  { id: 'acu-runtime/gpt-6.1-sol', name: 'gpt-6.1-sol', group: 'ACU' },
  { id: 'acu-runtime/image', name: 'Image', group: 'ACU', hidden: true },
  { id: 'auto/image', name: 'Image', group: 'Routing groups' },
  { id: 'openai/native', name: 'Native GPT', group: 'ChatGPT', own: true },
] });

test('WorkBuddy lists exact ACU additions without needing a default model field', async () => {
  const instance = await setup({ ready: true, agents: [{ id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: true, catalog: inventory() }] });
  const html = instance.elements.app.innerHTML;
  for (const phrase of ['model-flow compact-flow', 'acu-auto', 'gpt-6.1-sol', 'WorkBuddy 原生模型', '保留', '查看全部模型']) assert(html.includes(phrase), phrase);
  for (const phrase of ['默认 / 角色路由', '另有', '同名模型按来源区分', '新增模型各有来源', '未提供完整原生模型', '当前没有统一替换']) assert(!html.includes(phrase), phrase);
  assert(!html.includes('data-agent-model="workbuddy"'));
  instance.run('openModelInventory("workbuddy")');
  const modal = instance.elements.overlay.innerHTML;
  for (const phrase of ['acu-runtime/image', 'Native GPT', '只显示我勾选的模型', '隐藏未使用', '已隐藏']) assert(modal.includes(phrase), phrase);
  assert(!modal.includes('<table'));
  assert(!modal.includes('<h3>'));
  assert.equal(posts(instance).length, 0);
});


test('inventory toggles model visibility inline without opening native settings', async () => {
  const state = { ready: true, agents: [{ ...codex(), connected: true, catalog: inventory() }] };
  const instance = await setup(state);
  instance.run('openModelInventory("codex")');
  const target = { closest: selector => selector === '[data-model-visibility]' ? { dataset: { agentId: 'codex', modelId: 'acu-runtime/gpt-6.1-sol' } } : null };
  instance.listeners.overlay.click({ target });
  await new Promise(resolve => setImmediate(resolve));
  const writes = posts(instance);
  assert.equal(writes.at(-1).route, '/kernel/api/agent-models/codex');
  assert.deepEqual(JSON.parse(writes.at(-1).body), { hidden: ['acu-runtime/gpt-6.1-sol', 'acu-runtime/image'] });
  assert.equal(instance.elements.overlay.innerHTML.includes('data-native-model-list'), false);
});
test('disconnected and drifted Agents never claim their catalog was added', async () => {
  for (const [connected, needsRepair, label] of [[false, false, '可添加'], [true, true, '待核对']]) {
    const instance = await setup({ ready: true, agents: [{ ...codex(), connected, needsRepair, catalog: inventory() }] });
    assert(instance.elements.app.innerHTML.includes(label));
    assert(!instance.elements.app.innerHTML.includes('已加入列表'));
  }
});

test('inventory shows role destinations and group candidates without claiming native bypass', async () => {
  const instance = await setup({ ready: true, routingGroups: [{ ref: 'auto/image', members: ['custom/image', 'acu-runtime/image'], patterns: ['image-*'] }], agents: [{ ...codex(), connected: true, loginMode: 'api', catalog: inventory(), roles: [{ key: 'model', value: 'acu-runtime/acu-auto', ref: 'acu-runtime/acu-auto' }, { key: 'subagent', value: 'auto/image', ref: 'auto/image' }] }] });
  instance.run('openModelInventory("codex")');
  const modal = instance.elements.overlay.innerHTML;
  for (const phrase of ['flow-arrow', '默认模型', 'acu-auto', '高级配置', '原生模型说明']) assert(modal.includes(phrase), phrase);
  assert(!modal.includes('隐藏模型也不等于禁止显式调用'));
  assert.equal(posts(instance).length, 0);
});

test('inventory preserves namespaced identity and escapes model labels', async () => {
  const catalog = inventory();
  catalog.models[0].name = '<img src=x onerror=alert(1)>';
  const instance = await setup({ ready: true, agents: [{ ...codex(), connected: true, catalog }] });
  instance.run('openModelInventory("codex")');
  assert(!instance.elements.overlay.innerHTML.includes('<img src=x'));
  assert(instance.elements.overlay.innerHTML.includes('&lt;img'));
  assert(instance.elements.overlay.innerHTML.includes('acu-runtime/image'));
  assert(!instance.elements.overlay.innerHTML.includes('data-model-id="auto/image"'));
});

test('catalog polling refreshes an open inventory and reports unavailable data', async () => {
  const state = { ready: true, agents: [{ ...codex(), connected: true, catalog: inventory() }] };
  const instance = await setup(state);
  instance.run('openModelInventory("codex")');
  state.agents = [{ ...codex(), connected: true, catalogError: '模型清单暂未读取到' }];
  await instance.refresh();
  assert(instance.elements.overlay.innerHTML.includes('模型清单暂未读取到'));
  assert(!instance.elements.overlay.innerHTML.includes('acu-runtime/image'));
  assert.equal(posts(instance).length, 0);
});

test('empty route facts produce no filler sections or invented native retention', async () => {
  const instance = await setup({ ready: true, agents: [{ ...codex(), connected: true, catalog: { models: [] } }] });
  instance.run('openModelInventory("codex")');
  const html = instance.elements.overlay.innerHTML;
  for (const phrase of ['当前没有统一替换', '未列出额外', '这不是历史', '未提供完整', '原生模型保留', '<h3>']) assert(!html.includes(phrase), phrase);
  assert.equal(posts(instance).length, 0);
});

test('model search filters the single list and survives status updates without writes', async () => {
  const state = { ready: true, agents: [{ ...codex(), connected: true, catalog: inventory() }] };
  const instance = await setup(state);
  instance.run('openModelInventory("codex")');
  const rows = [{ dataset: { flowSearch: 'acu-auto acu' } }, { dataset: { flowSearch: 'gpt-6.1-sol acu' } }];
  const empty = {};
  instance.elements.overlay.querySelectorAll = selector => selector === '[data-flow-search]' ? rows : [];
  instance.elements.overlay.querySelector = selector => selector === '[data-flow-empty]' ? empty : null;
  instance.listeners.overlay.input({ target: { id: 'inventory-search', value: 'GPT' } });
  assert(rows[0].hidden);
  assert(!rows[1].hidden);
  assert(empty.hidden);
  state.agents = [{ ...state.agents[0], restartRequired: true }];
  await instance.refresh();
  assert(instance.elements.overlay.innerHTML.includes('value="GPT"'));
  instance.listeners.overlay.input({ target: { id: 'inventory-search', value: 'no-match' } });
  assert(!empty.hidden);
  instance.run('closeOverlay()');
  instance.run('openModelInventory("codex")');
  assert(!instance.elements.overlay.innerHTML.includes('value="no-match"'));
  assert.equal(posts(instance).length, 0);
});

test('desktop automatically starts one kernel and polls startup progress', async () => {
  const state = { desktop: true, ready: false, starting: false };
  const instance = await setup(state);
  assert.equal(posts(instance).length, 1);
  assert.equal(posts(instance)[0].route, '/runtime/kernel/start');
  assert(instance.elements.app.innerHTML.includes('正在准备 ACU · 50%'));
  assert.equal(instance.timers.at(-1).milliseconds, 1500);
  await instance.refresh();
  assert.equal(posts(instance).length, 1);
  state.ready = true; state.starting = false;
  await instance.refresh();
  assert.equal(instance.timers.at(-1).milliseconds, 4000);
});

test('an existing desktop kernel and developer mode do not trigger installation', async () => {
  for (const state of [{ desktop: true, ready: true }, { desktop: false, ready: false }]) assert.equal(posts(await setup(state)).length, 0);
});

test('home only shows the connection toolbar and compact agent controls', async () => {
  const instance = await setup({ ready: true, desktop: true, agents: [codex()] });
  const html = instance.elements.app.innerHTML;
  for (const phrase of ['启用的 API Key', '登录 ACU', '一键配置', 'ACU 接入未开启', '点击查看推荐配置并开启', '原生配置']) assert(html.includes(phrase));
  assert.equal((html.match(/class="flow-number/g) || []).length, 3);
  for (const phrase of ['步骤 1', '步骤 2', '步骤 3', '回到 Agent 使用']) assert(html.includes(phrase));
  for (const phrase of ['已识别现有连接', '不需要再次添加 Key', '首次接入可能调整', 'CODING AGENT', '当前通道', '第 1 步', '已选 1 个', 'data-step=']) assert(!html.includes(phrase));
});

test('home shows per-Agent routes, native model options and a visible route toggle', async () => {
  const instance = await setup({ ready: true, connection: { known: true, configured: false, choices: [] }, agents: [{ ...codex(), connected: true, model: 'acu/auto', modelOptions: [{ value: 'acu/auto', ref: 'acu/auto', label: 'acu-auto', group: 'ACU' }, { value: 'chatgpt', label: 'ChatGPT' }] }] });
  const html = instance.elements.app.innerHTML;
  for (const phrase of ['本机路由已配置', 'data-agent-model="codex"', 'ChatGPT', 'ACU 接入已开启', '点击关闭并还原原配置', 'route-switch', '高级配置']) assert(html.includes(phrase));
  assert(!html.includes('ACU 路由已开启'));
  assert(!html.includes('id="agent-model-codex" data-agent-model="codex" aria-label="Codex 模型" disabled'));
  await instance.run('openDisconnectDialog("codex")');
  assert(instance.elements.overlay.innerHTML.includes('Codex 关闭 ACU 接入'));
  assert(instance.elements.overlay.innerHTML.includes('将要还原的配置'));
  assert.equal(posts(instance).length, 0, 'opening restore preview must not write configuration');
});

test('compact model flow exposes a subtask model selector', async () => {
  const instance = await setup({ ready: true, agents: [{ ...codex(), connected: true, model: 'acu-runtime/acu-auto', roles: [{ key: 'model', value: 'acu-runtime/acu-auto', ref: 'acu-runtime/acu-auto', name: 'acu-auto' }, { key: 'subagent', value: 'acu-runtime/gpt-6.1-sol', ref: 'acu-runtime/gpt-6.1-sol', name: 'gpt-6.1-sol' }] }] });
  const html = instance.elements.app.innerHTML;
  assert(html.includes('子任务模型'));
  assert(html.includes('id="agent-subagent-codex"'));
  assert(html.includes('data-agent-role="subagent"'));
});
test('route toggle closes WorkBuddy through a contained Runtime confirmation', async () => {
  const state = { ready: true, agents: [{ id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: true }] };
  const instance = await setup(state, async (route, options) => {
    if (route === '/kernel/api/agents/preview/workbuddy') return { body: { changes: [{ op: '-', text: '"vendor": "magpie"' }] } };
    if (route === '/kernel/api/agents/disconnect/workbuddy') { state.agents[0].connected = false; return { body: { agents: [{ id: 'workbuddy', wired: false }] } }; }
    return undefined;
  });
  await instance.run('openDisconnectDialog("workbuddy")');
  assert(instance.elements.overlay.innerHTML.includes('WorkBuddy 移除附加模型'));
  await instance.run('confirmDisconnect()');
  assert.equal(instance.elements.overlay.innerHTML, '');
  assert(instance.elements.toast.textContent.includes('移除附加模型'));
  assert(!instance.elements.app.innerHTML.includes('附加模型已开启'));
  assert.equal(posts(instance).filter(call => call.route.includes('disconnect/workbuddy')).length, 1);
});

test('disconnect errors remain in the modal and do not blank the homepage', async () => {
  const instance = await setup({ ready: true, agents: [{ id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: true }] }, async route => route === '/kernel/api/agents/preview/workbuddy' ? { ok: false, status: 500, body: { error: '内核拒绝修改' } } : undefined);
  await instance.run('openDisconnectDialog("workbuddy")');
  assert(instance.elements.overlay.innerHTML.includes('无法读取还原预览'));
  assert(instance.elements.app.innerHTML.includes('WorkBuddy'));
  await instance.run('confirmDisconnect()');
  assert(instance.elements.app.innerHTML.includes('WorkBuddy'));
});

test('connection management always uses acucompute.com and keeps key replacement inline', async () => {
  const instance = await setup({ ready: true, desktop: true, account: { loggedIn: true, username: 'example' }, connection: connected() });
  instance.run('ui.connectionOpen=true; render()');
  const html = instance.elements.app.innerHTML;
  assert(html.includes('acucompute.com'));
  assert(!html.includes('id="account-site"'));
  assert(html.indexOf('选择 API Key') < html.indexOf('更换 Key'));
});

test('an unconnected Agent opens a per-Agent recommendation instead of the batch picker', async () => {
  const instance = await setup({ ready: true, connection: connected(), agents: [{ ...codex(), connected: false }] });
  await instance.run('configureAgent("codex")');
  const html = instance.elements.overlay.innerHTML;
  for (const phrase of ['Codex 接入 ACU', '原有模型', 'ACU 模型', '默认模型', '看图', '读文档', '联网搜索', '推荐已选好', '同意并接入', '不把所有 GPT 模型统一改走 ACU']) assert(html.includes(phrase), phrase);
  assert(!html.includes('同步辅助模型'));
  assert(html.includes('id="setup-tools"'));
  assert.equal(posts(instance).length, 0);
  await instance.run('confirmSetup()');
  assert.deepEqual(JSON.parse(posts(instance)[0].body), { agent: 'codex', provider: 'acu-runtime', model: 'acu-auto', auxiliary: false });
});

test('WorkBuddy recommendation explains that model choice stays inside WorkBuddy', async () => {
  const instance = await setup({ ready: true, connection: connected(), agents: [{ id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: false, modelOptions: [] }] });
  await instance.run('configureAgent("workbuddy")');
  const html = instance.elements.overlay.innerHTML;
  assert(html.includes('WorkBuddy 接入 ACU'));
  assert(html.includes('仍由你在 WorkBuddy 任务内选择'));
  assert(html.includes('同意并添加模型'));
  assert(!html.includes('id="setup-model"'));
  assert(!html.includes('→ acu-auto → ACU'));
});

test('recommendation follows the selected model and only offers supported Agent options', async () => {
  const connection = connected();
  connection.choices[0].models.push('image-only-not-supported');
  const instance = await setup({ ready: true, connection, agents: [codex()] });
  await instance.run('configureAgent("codex")');
  assert(!instance.elements.overlay.innerHTML.includes('image-only-not-supported'));
  instance.listeners.overlay.change({ target: { id: 'setup-model', value: 'gpt-6.1-sol' } });
  assert(instance.elements.overlay.innerHTML.includes('Codex → gpt-6.1-sol → ACU'));
  assert(instance.elements.overlay.innerHTML.includes('gpt-6.1-sol 的识图取决于模型能力'));
  await instance.run('confirmSetup()');
  assert.equal(JSON.parse(posts(instance)[0].body).model, 'gpt-6.1-sol');
});

test('no compatible models blocks confirmation instead of guessing from the account list', async () => {
  const instance = await setup({ ready: true, agents: [{ ...codex(), modelOptions: [] }] });
  await instance.run('configureAgent("codex")');
  assert(instance.elements.overlay.innerHTML.includes('暂无兼容模型'));
  assert(instance.elements.overlay.innerHTML.includes('data-confirm-setup disabled'));
  await instance.run('confirmSetup()');
  assert.equal(posts(instance).length, 0);
});

test('capability preview reads shared settings without modifying tools or claiming success', async () => {
  const settings = { vision: 'off', imageGen: 'missing/image', imageGenMissing: 'missing/image', imageGenAuto: 'fixture/draw', imageGenModels: [{ id: 'fixture/draw', name: '示例生图', pname: '已有来源' }], searchAPIs: [{ name: '关闭的搜索', ready: true, off: true }, { name: '已有搜索', ready: true }] };
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route === '/api/settings' ? { body: settings } : undefined);
  await instance.run('configureAgent("codex")');
  const html = instance.elements.overlay.innerHTML;
  for (const phrase of ['未启用', '示例生图 · 已有来源', '已有搜索', '已有配置 ≠ 已验证可用', '同时添加 Magpie 生图 MCP', '暂不提供']) assert(html.includes(phrase), phrase);
  assert(!html.includes('关闭的搜索'));
  assert(!html.includes('missing/image'));
  assert.equal(posts(instance).length, 0);
});

test('unreadable capability settings stay unknown and never fabricate enabled services', async () => {
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route === '/api/settings' ? { ok: false, body: { error: 'offline' } } : undefined);
  await instance.run('configureAgent("codex")');
  assert(instance.elements.overlay.innerHTML.includes('暂未读取到配置'));
  assert(instance.elements.overlay.innerHTML.includes('待确认'));
  assert.equal(instance.run('ui.setup.loading'), false);
  assert.equal(posts(instance).length, 0);
});

test('a dismissed recommendation does not reopen when settings arrive', async () => {
  let resolveSettings;
  const pending = new Promise(resolve => { resolveSettings = resolve; });
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route === '/api/settings' ? pending : undefined);
  const opening = instance.run('configureAgent("codex")');
  await new Promise(resolve => setImmediate(resolve));
  assert(instance.run('ui.setup.loading'));
  await instance.run('confirmSetup()');
  assert.equal(posts(instance).length, 0);
  instance.run('closeOverlay()');
  resolveSettings({ body: {} });
  await opening;
  assert.equal(instance.elements.overlay.innerHTML, '');
});

test('completed connection keeps management secondary', async () => {
  const instance = await setup({ ready: true, desktop: true, account: { loggedIn: true, username: 'example' }, agents: [codex()] });
  const html = instance.elements.app.innerHTML;
  assert(html.includes('连接配置已完成'));
  assert(html.includes('class="account-name">example'));
  assert(html.includes('class="text-button connection-manage"'));
  assert(!html.includes('button primary" data-action="manage-connection"'));
});

test('native model choices and in-app model pickers are not falsely shown as gateway routing', async () => {
  const instance = await setup({ ready: true, agents: [{ ...codex(), connected: true, model: 'chatgpt', modelOptions: [{ value: 'chatgpt', label: 'ChatGPT' }] }, { id: 'workbuddy', name: 'WorkBuddy', connected: true, detected: true }] });
  assert(instance.elements.app.innerHTML.includes('ChatGPT'));
  assert(instance.elements.app.innerHTML.includes('原来源'));
  assert(instance.elements.app.innerHTML.includes('WorkBuddy 原生模型'));
  assert(instance.elements.app.innerHTML.includes('附加模型已开启'));
  assert(!instance.elements.app.innerHTML.includes('经 ACU Runtime 本机网关'));
});

test('homepage keeps account management inline and hides developer-only details', async () => {
  const instance = await setup({ ready: true, desktop: true, agents: [codex()] });
  let html = instance.elements.app.innerHTML;
  for (const phrase of ['我的工作区', '开发说明', '内核已就绪', '内核路径', '发布与跨平台', '模型与工具配置']) assert(!html.includes(phrase));
  assert(html.includes('登录 ACU'));
  instance.listeners.app.click({ target: { closest: selector => selector === '[data-action]' ? { dataset: { action: 'manage-connection' } } : null } });
  html = instance.elements.app.innerHTML;
  assert(html.includes('管理连接'));
  assert(html.includes('ACU API Key'));
  assert(!html.includes('data-page="connection"'));
});

test('home model change calls only native set and preserves other model roles and Key', async () => {
  const state = { ready: true, agents: [{ ...codex(), connected: true, model: 'acu/auto', modelOptions: [{ value: 'acu/auto' }, { value: 'chatgpt' }] }] };
  const instance = await setup(state, async (route, options) => {
    if (route !== '/api/set') return;
    const input = JSON.parse(options.body);
    state.agents[0].model = input.value;
    return { body: { agents: [{ id: 'codex', wired: true, fields: [{ key: 'model', value: input.value }] }] } };
  });
  await instance.run('setAgentModel("codex", "chatgpt")');
  assert.deepEqual(posts(instance).map(call => call.route), ['/api/set']);
  assert.deepEqual(JSON.parse(posts(instance)[0].body), { agent: 'codex', field: 'model', value: 'chatgpt' });
  assert.equal(instance.run('ui.status.agents[0].model'), 'chatgpt');
});

test('unknown, disconnected or unchanged model selections never write configuration', async () => {
  const state = { ready: true, agents: [{ ...codex(), connected: true, model: 'acu/auto', modelOptions: [{ value: 'acu/auto' }, { value: 'chatgpt' }] }] };
  const instance = await setup(state);
  await instance.run('setAgentModel("codex", "acu/auto")');
  await instance.run('setAgentModel("codex", "unexpected")');
  state.agents[0].connected = false;
  await instance.run('setAgentModel("codex", "chatgpt")');
  assert.equal(posts(instance).length, 0);
});

test('model change must verify the native response and never claims failed writes succeeded', async () => {
  const instance = await setup({ ready: true, agents: [{ ...codex(), connected: true, model: 'old', modelOptions: [{ value: 'new' }] }] }, async route => route === '/api/set' ? { body: { agents: [{ id: 'codex', wired: true, fields: [{ key: 'model', value: 'old' }] }] } } : undefined);
  await instance.run('setAgentModel("codex", "new")');
  assert(instance.elements.toast.textContent.includes('未确认模型已生效'));
  assert.equal(instance.run('ui.applying'), false);
});

test('one click previews options, and only confirmation configures selected agents', async () => {
  const state = { ready: true, agents: [codex(), { id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: true }] };
  const instance = await setup(state, async (route, options) => {
    if (route !== '/runtime/agents/configure') return;
    const input = JSON.parse(options.body);
    state.agents.find(agent => agent.id === input.agent).connected = true;
    return { body: { agents: [{ id: input.agent, wired: true }] } };
  });
  await instance.run('applyAgents()');
  assert.equal(posts(instance).length, 0);
  assert(instance.elements.overlay.innerHTML.includes('确认配置'));
  assert(instance.elements.overlay.innerHTML.includes('data-setup-agent="codex"'));
  assert(!instance.elements.overlay.innerHTML.includes('role="switch"'));
  assert.equal(instance.run('ui.setup.model'), 'acu-auto');
  await instance.run('confirmSetup()');
  assert.deepEqual(posts(instance).map(call => call.route), ['/runtime/agents/configure', '/kernel/api/agent-models/codex', '/runtime/capabilities']);
  assert.deepEqual(JSON.parse(posts(instance)[0].body), { agent: 'codex', provider: 'acu-runtime', model: 'acu-auto', auxiliary: false });
  assert.equal(instance.run('ui.selected.has("codex")'), false);
});

test('cancelling the configuration dialog never writes any configuration', async () => {
  const instance = await setup({ ready: true, agents: [codex()] });
  await instance.run('applyAgents()');
  instance.run('closeOverlay()');
  await instance.run('confirmSetup()');
  assert.equal(posts(instance).length, 0);
  assert.equal(instance.elements.overlay.innerHTML, '');
});

test('dialog model and auxiliary switches are sent exactly as chosen', async () => {
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route === '/runtime/agents/configure' ? { body: { agents: [{ id: 'codex', wired: true }] } } : undefined);
  await instance.run('applyAgents()');
  instance.listeners.overlay.change({ target: { id: 'setup-model', value: 'gpt-6.1-sol' } });
  instance.listeners.overlay.change({ target: { id: 'setup-auxiliary', checked: true } });
  await instance.run('confirmSetup()');
  assert.equal(JSON.parse(posts(instance)[0].body).model, 'gpt-6.1-sol');
  assert.equal(JSON.parse(posts(instance)[0].body).auxiliary, true);
});

test('dialog Agent switches can include an already connected WorkBuddy', async () => {
  const instance = await setup({ ready: true, agents: [codex(), { id: 'workbuddy', name: 'WorkBuddy', detected: true, connected: true }] });
  await instance.run('applyAgents()');
  instance.listeners.overlay.change({ target: { dataset: { setupAgent: 'workbuddy' }, checked: true } });
  assert.equal(instance.run('ui.setup.agents.has("workbuddy")'), true);
  assert(instance.elements.overlay.innerHTML.includes('保留现有模型'));
});

test('deleting a local Key removes its check and blocks setup on the next poll', async () => {
  const state = { ready: true, agents: [codex()], connection: connected() };
  const instance = await setup(state);
  assert(instance.elements.app.innerHTML.includes('quick-check complete'));
  state.connection = { known: true, configured: false, choices: [] };
  await instance.run('refresh(false, true)');
  assert(!instance.elements.app.innerHTML.includes('quick-check complete'));
  await instance.run('applyAgents()');
  assert.equal(instance.run('ui.setup'), null);
  assert.equal(posts(instance).length, 0);
});

test('a revoked remote Key cannot retain a check merely because it is stored', async () => {
  const connection = { ...connected(), configured: false, error: 'Key 已失效，请更换' };
  const instance = await setup({ ready: true, connection, agents: [codex()] });
  assert(!instance.elements.app.innerHTML.includes('quick-check complete'));
  assert(instance.elements.app.innerHTML.includes('Key 已失效'));
  assert.equal(instance.run('activeChoice()'), undefined);
});

test('a deleted selected Key is not silently replaced with a different Key', async () => {
  const state = { ready: true, connection: connected() };
  const instance = await setup(state);
  instance.change({ id: 'quick-key', value: 'local:acu-runtime' });
  state.connection = { known: true, configured: true, provider: 'other', choices: [{ id: 'other', name: 'Other ACU', models: ['acu-auto'] }] };
  await instance.run('refresh(false, true)');
  assert.equal(instance.run('activeChoice()'), undefined);
});

test('status read failures clear a previously connected indicator', async () => {
  let failed = false;
  const instance = await setup({ ready: true }, async route => failed && route === '/runtime/status' ? { ok: false, body: { error: 'Connection unavailable' } } : undefined);
  failed = true;
  await instance.run('refresh(false, true)');
  assert(!instance.elements.app.innerHTML.includes('quick-check complete'));
});

test('background polling does not recreate the page when state is unchanged', async () => {
  const instance = await setup({ ready: true });
  instance.elements.app.innerHTML = 'unchanged DOM marker';
  await instance.run('refresh(false, true)');
  assert.equal(instance.elements.app.innerHTML, 'unchanged DOM marker');
});

test('polling preserves in-progress dialog selections', async () => {
  const state = { ready: true, agents: [codex()] };
  const instance = await setup(state);
  await instance.run('applyAgents()');
  instance.run('ui.setup.model = "gpt-6.1-sol"');
  state.connection = { known: true, configured: false, choices: [] };
  await instance.run('refresh(false, true)');
  assert.equal(instance.run('ui.setup.model'), 'gpt-6.1-sol');
  assert(instance.elements.overlay.innerHTML.includes('确认配置'));
});

test('partial failures do not undo successful Agent configuration', async () => {
  const state = { ready: true, agents: [codex(), { id: 'claude', name: 'Claude Code', detected: true }] };
  const instance = await setup(state, async (route, options) => {
    if (route !== '/runtime/agents/configure') return;
    if (JSON.parse(options.body).agent === 'claude') return { ok: false, body: { error: 'fixture write failure' } };
    state.agents[0].connected = true;
    return { body: { agents: [{ id: 'codex', wired: true }] } };
  });
  await instance.run('applyAgents();');
  await instance.run('confirmSetup()');
  assert(instance.elements.app.innerHTML.includes('fixture write failure'));
  assert.equal(instance.run('ui.selected.has("codex")'), false);
  assert.equal(instance.run('ui.selected.has("claude")'), true);
  assert(!posts(instance).some(call => call.route.includes('disconnect')));
});

test('repeated confirmations cannot start concurrent writes', async () => {
  let release;
  const waiting = new Promise(resolve => { release = resolve; });
  const instance = await setup({ ready: true, agents: [codex()] }, async route => {
    if (route !== '/runtime/agents/configure') return;
    await waiting;
    return { body: { agents: [{ id: 'codex', wired: true }] } };
  });
  await instance.run('applyAgents()');
  const first = instance.run('confirmSetup()');
  await instance.run('confirmSetup()');
  assert.equal(posts(instance).length, 1);
  release(); await first;
});

test('HTTP success without wired state is not reported as setup success', async () => {
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route === '/runtime/agents/configure' ? { body: { agents: [{ id: 'codex', wired: false }] } } : undefined);
  await instance.run('applyAgents()'); await instance.run('confirmSetup()');
  assert.equal(instance.run('ui.results[0].ok'), false);
});

test('account dropdown selection prepares a Key without writing Agent files', async () => {
  const instance = await setup({ ready: true }, async route => route === '/runtime/connection' ? { body: { ok: true } } : undefined);
  instance.change({ id: 'quick-key', value: 'account:9' });
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(posts(instance).map(call => [call.route, call.body]), [['/runtime/connection', '{"id":9}']]);
});

test('account page is management only, without a duplicated numbered step', async () => {
  const instance = await setup({ ready: true });
  instance.run('ui.page="connection"; render()');
  assert(instance.elements.app.innerHTML.includes('管理连接'));
  assert.equal(instance.run('ui.page'), 'overview');
  assert(!instance.elements.app.innerHTML.includes('data-page="connection"'));
  assert(!instance.elements.app.innerHTML.includes('第 1 步'));
});






test('workbench omits local routing choices but preserves the exact active advanced value', async () => {
  const agent = { ...codex(), connected: true, model: 'acu-runtime/gpt-6.1-sol', modelOptions: [
    { value: 'auto/sol', ref: 'auto/sol', label: 'Sol', group: 'Routing groups' },
    { value: 'special-route', ref: 'smart/sol', label: 'Smart Sol' },
    { value: 'acu-runtime/claude-opus-5-5', ref: 'acu-runtime/claude-opus-5-5', label: 'Claude Opus 5.5', group: 'ACU' },
    { value: 'acu-runtime/glm-5.3-flash', ref: 'acu-runtime/glm-5.3-flash', label: 'GLM-5.3-Flash', group: 'ACU' },
    { value: 'acu-runtime/gpt-6.1-sol', ref: 'acu-runtime/gpt-6.1-sol', label: 'Sol', group: 'ACU' },
    { value: 'native-sol', label: 'Sol', group: 'OpenAI' }
  ] };
  const instance = await setup({ ready: true, connection: { known: true, configured: true, provider: 'acu-runtime', choices: [{ id: 'acu-runtime', name: 'ACU', models: ['acu-auto', 'gpt-6.1-sol', 'glm-5.3-flash', 'claude-opus-5-5'] }] }, agents: [agent], routingGroups: [{ ref: 'smart/sol' }] });
  let html = instance.run('agentModelSelect(ui.status.agents[0])');
  assert(!html.includes('value="auto/sol"'));
  assert(!html.includes('value="special-route"'));
  assert(html.includes('Sol · ACU'));
  assert(html.includes('Sol · Codex 原生'));
  assert(html.indexOf('label="推荐模型 · ACU"') < html.indexOf('GLM-5.3-Flash · ACU'));
  assert(html.indexOf('GLM-5.3-Flash · ACU') < html.indexOf('Claude Opus 5.5 · ACU'));
  assert(html.indexOf('推荐模型 · ACU') < html.indexOf('Codex 原生'));
  agent.model = 'special-route';
  html = instance.run('agentModelSelect(ui.status.agents[0])');
  assert(html.includes('value="special-route" selected'));
  assert(html.includes('当前高级配置'));
  assert(!html.includes('Routing groups'));
  assert.equal(posts(instance).length, 0);
});

test('ordinary inventory excludes routing groups and bulk actions preserve their visibility', async () => {
  const catalog = inventory();
  catalog.models.push({ id: 'auto/hidden', name: 'Hidden route', group: 'Routing groups', hidden: true });
  const instance = await setup({ ready: true, agents: [{ ...codex(), connected: true, catalog }] });
  instance.run('openModelInventory("codex")');
  const html = instance.elements.overlay.innerHTML;
  assert(!html.includes('data-model-id="auto/image"'));
  assert(!html.includes('Routing groups'));
  assert(!html.includes('自动选择'));
  instance.run('bulkInventory("codex", "hide"); bulkInventory("codex", "hide")');
  await new Promise(resolve => setImmediate(resolve));
  const payload = JSON.parse(posts(instance).at(-1).body);
  assert(!payload.hidden.includes('auto/image'));
  assert(payload.hidden.includes('auto/hidden'));
});


test('recommendations hide older and special-purpose models but protect active and unrelated entries', async () => {
  const instance = await setup({ ready: true, agents: [codex()] }, async (route, options) => {
    if (route === '/kernel/api/agent-models/codex' && !options.method) return { body: { models: [
      { id: 'acu-runtime/gpt-6.1-sol', hidden: true },
      { id: 'acu-runtime/gpt-image-1' },
      { id: 'acu-runtime/old-active', inUse: true },
      { id: 'auto/image' }, { id: 'other/hidden', hidden: true }
    ] } };
  });
  await instance.run('applyRecommendedVisibility("codex", "acu-runtime", "acu-auto")');
  const payload = JSON.parse(posts(instance).at(-1).body);
  assert.deepEqual(payload.hidden, ['acu-runtime/gpt-image-1', 'other/hidden']);
  assert.equal(instance.run('recommendedModels({ models: ["gpt-image-1", "gpt-6.1-sol", "glm-5.3-flash"] }).join(",")'), 'gpt-6.1-sol,glm-5.3-flash');
});

test('DeepSeek V4.1 Flash variants are recommended while legacy V4 Flash is omitted', async () => {
  const instance = await setup({ ready: true, agents: [codex()] });
  const models = instance.run('recommendedACUModels');
  assert(models.includes('deepseek-v4-pro'));
  assert(models.includes('deepseek-v4.1-flash'));
  assert(models.includes('deepseek-v4.1-flash-1017'));
  assert(!models.includes('deepseek-v4-flash'));
});

test('desktop settings exposes compatibility update status and the check action', async () => {
  const instance = await setup({ desktop: true, version: '0.1.15', update: { supported: true, phase: 'available', current: '0.1.15', kernel: 'v0.1.3150', release: { version: '0.1.16' } } });
  instance.run('ui.page="settings"; render()');
  const html = instance.elements.app.innerHTML;
  for (const phrase of ['应用更新', '0.1.15', 'v0.1.3150', '0.1.16', '更新并重启', '检查更新']) assert(html.includes(phrase), phrase);
  assert(html.includes('本机 Key 和 Agent 配置会保留'));
  assert(!instance.elements.app.innerHTML.includes('GitHub'));
});

const capabilityFixture = () => ({ vision: 'native/vision', imageGen: 'off', visionModels: [{ id: 'acu-runtime/gpt-6.1-sol', name: 'GPT Sol' }], imageModels: [], tools: [], searchReady: false, searchURL: '', messages: ['PDF 继续使用应用文件工具'] });

test('bulk setup prepares kernel MCP only for confirmed agents and can opt out', async () => {
  for (const enabled of [true, false]) {
    const instance = await setup({ ready: true, agents: [codex()] }, async route => {
      if (route === '/runtime/agents/configure') return { body: { agents: [{ id: 'codex', wired: true }] } };
      if (route === '/runtime/capabilities') return { body: capabilityFixture() };
    });
    await instance.run('applyAgents()');
    assert.equal(instance.run('ui.setup.tools'), true);
    if (!enabled) instance.listeners.overlay.change({ target: { id: 'setup-tools', checked: false } });
    await instance.run('confirmSetup()');
    const tools = posts(instance).filter(call => call.route === '/runtime/capabilities');
    assert.equal(tools.length, enabled ? 1 : 0);
    if (enabled) {
      assert.deepEqual(JSON.parse(tools[0].body), { provider: 'acu-runtime', agents: ['codex'] });
      assert(instance.elements.app.innerHTML.includes('搜索待配置'));
      assert(instance.elements.app.innerHTML.includes('图片与搜索'));
    }
  }
});

test('capability manager preserves untouched external models and sends only explicit changes', async () => {
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route.startsWith('/runtime/capabilities') ? { body: capabilityFixture() } : undefined);
  await instance.run('openCapabilities()');
  assert.equal(posts(instance).length, 0);
  assert(instance.elements.overlay.innerHTML.includes('native/vision（当前来源）'));
  assert(instance.elements.overlay.innerHTML.includes('待配置'));
  instance.listeners.overlay.change({ target: { dataset: { capability: 'vision' }, value: 'acu-runtime/gpt-6.1-sol' } });
  await instance.run('saveCapabilities()');
  assert.deepEqual(JSON.parse(posts(instance)[0].body), { provider: 'acu-runtime', modelsOnly: true, vision: 'acu-runtime/gpt-6.1-sol' });
});

test('capability save failure remains visible and retryable', async () => {
  const instance = await setup({ ready: true, agents: [codex()] }, async (route, options) => route.startsWith('/runtime/capabilities') ? options.method === 'POST' ? { ok: false, body: { error: '部分 Agent 写入失败' } } : { body: capabilityFixture() } : undefined);
  await instance.run('openCapabilities()');
  instance.listeners.overlay.change({ target: { dataset: { capability: 'vision' }, value: 'off' } });
  await instance.run('saveCapabilities()');
  assert(instance.elements.overlay.innerHTML.includes('部分 Agent 写入失败'));
  assert.equal(instance.run('ui.capabilities.saving'), false);
});

test('free search hides dependency choices and remains automatic on a fresh machine', async () => {
  const view = { ...capabilityFixture(), freeSearchAvailable: false };
  const instance = await setup({ ready: true, agents: [codex()] }, async route => route.startsWith('/runtime/capabilities') ? { body: view } : undefined);
  await instance.run('openCapabilities()');
  const html = instance.elements.overlay.innerHTML;
  assert(html.includes('应用工具'));
  assert(!html.includes('data-capability="freeSearch"'));
  assert(!html.includes('data-capability-agent'));
  assert(!html.includes('searchURL'));
  for (const phrase of ['uvx', '安装 uv', '需先安装']) assert(!html.includes(phrase), phrase);
  assert(!html.includes('data-capability="freeSearch" checked disabled'));
});


test('configured apps show status and explicit actions without switches or ACU pending warnings', async () => {
 const view = {...capabilityFixture(), freeSearchConfigured: true, tools: ['acu-vision','acu-image','acu-free-search'].map(name => ({name, agents:['codex']}))};
 const instance = await setup({ready:true,agents:[codex()]}, async route => route.startsWith('/runtime/capabilities') ? {body:view} : undefined);
 await instance.run('openCapabilities()');
 const html=instance.elements.overlay.innerHTML;
 for(const phrase of ['已配置 ✓','重新配置','使用免费搜索','保存模型设置','高级设置']) assert(html.includes(phrase),phrase);
 for(const phrase of ['3/3','type="checkbox"','ACU 搜索待配置','扫描件','HTTP','MCP','searchURL']) assert(!html.includes(phrase),phrase);
 assert(html.includes('data-save-capabilities disabled'));
 await instance.run('saveCapabilities()');assert.equal(posts(instance).length,0);
 await instance.run('configureCapabilities("codex")');
 assert.deepEqual(JSON.parse(posts(instance)[0].body),{provider:'acu-runtime',agents:['codex']});
});

test('failed tool setup refreshes partial results and retries only missing apps', async () => {
 const view = {...capabilityFixture(),tools:['acu-vision','acu-image','acu-free-search'].map(name=>({name,agents:['codex']}))};
 const instance=await setup({ready:true,agents:[codex(),{id:'claude',name:'Claude Code',detected:true}]},async (route,options)=>route.startsWith('/runtime/capabilities') ? options.method==='POST'?{ok:false,body:{error:'工具准备失败'}}:{body:view}:undefined);
 await instance.run('openCapabilities()');await instance.run('configureCapabilities("missing")');
 assert.deepEqual(JSON.parse(posts(instance)[0].body),{provider:'acu-runtime',agents:['claude']});
 assert(instance.elements.overlay.innerHTML.includes('重试未完成项'));
 assert(instance.elements.overlay.innerHTML.includes('已配置 ✓'));
 assert(instance.elements.overlay.innerHTML.includes('工具准备失败'));
});

test('repeated one-click setup preserves connected app model and visibility by default',async()=>{
 const instance=await setup({ready:true,agents:[{...codex(),connected:true}]},async route=>route==='/runtime/agents/configure'?{body:{agents:[{id:'codex',wired:true}]}}:route==='/runtime/capabilities'?{body:capabilityFixture()}:undefined);
 instance.run('ui.selected = new Set(["codex"])');
 await instance.run('applyAgents()');await instance.run('confirmSetup()');
 assert.equal(JSON.parse(posts(instance)[0].body).preserve,true);
 assert(!posts(instance).some(call=>call.route==='/kernel/api/agent-models/codex'));
});


test('capability advanced search opens the kernel search section',async()=>{
 const instance=await setup({ready:true,agents:[codex()]},async route=>route.startsWith('/runtime/capabilities')?{body:capabilityFixture()}:undefined);
 await instance.run('openCapabilities()');
 instance.listeners.overlay.click({target:{closest:selector=>selector==='[data-capability-native]'?{dataset:{capabilityNative:'search'}}:null}});
 assert(instance.elements.app.innerHTML.includes('iframe'));
 assert(instance.elements.app.innerHTML.includes('searchList'));
 assert(instance.elements.app.innerHTML.includes('tab=models'));
 assert.equal(posts(instance).length,0);
});

test('custom tool settings are named and tool reconfiguration does not submit model drafts',async()=>{
 const view={...capabilityFixture(),tools:[{name:'acu-vision',agents:['codex'],custom:true}]};
 const instance=await setup({ready:true,agents:[codex()]},async route=>route.startsWith('/runtime/capabilities')?{body:view}:undefined);
 await instance.run('openCapabilities()');
 assert(instance.elements.overlay.innerHTML.includes('自定义设置'));
 instance.listeners.overlay.change({target:{dataset:{capability:'vision'},value:'off'}});
 await instance.run('configureCapabilities("codex")');
 assert.deepEqual(JSON.parse(posts(instance)[0].body),{provider:'acu-runtime',agents:['codex']});
 assert.equal(instance.run('ui.capabilities.draft.vision'),'off');
});
