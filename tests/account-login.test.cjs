const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../web/account-login.js'), 'utf8');

function setup(platform = 'windows', origin = 'https://acu.best', readyState = 'complete', options = {}) {
  const messages = [];
  const requests = [];
  const listeners = {};
  const intervals = [];
  const timers = [];
  const storage = options.storage || new Map();
  let now = Date.now();
  const body = { append(element) { this.child = element; } };
  const window = { addEventListener(name, listener) { listeners[name] = listener; } };
  window.top = window;
  const bridge = { postMessage(message) { messages.push(JSON.parse(message)); } };
  if (platform === 'windows') window.chrome = { webview: bridge };
  if (platform === 'mac') window.webkit = { messageHandlers: { external: bridge } };
  const ticket = { site: 'https://acu.best', state: 'isolated-one-use-nonce', expiresAt: 2000000000, ...options.ticket };
  const location = { origin, pathname: '/sign-in' };
  const context = vm.createContext({
    window,
    location,
    AbortSignal,
    Date: { now: () => now },
    sessionStorage: { getItem: key => storage.get(key) || null, setItem: (key, value) => storage.set(key, value), removeItem: key => storage.delete(key) },
    navigator: options.navigator,
    setTimeout: callback => { timers.push(callback); return timers.length; }, clearTimeout() {},
    setInterval: callback => { intervals.push(callback); return intervals.length; }, clearInterval() {},
    document: {
      body,
      readyState,
      addEventListener: (name, listener) => { listeners[name] = listener; },
      getElementById: () => body.child,
      createElement: () => ({ style: {}, children: [], append(...children) { this.children.push(...children); } }),
    },
    fetch: async (address, fetchOptions) => {
      requests.push({ address, options: fetchOptions });
      if (instance.respond) return instance.respond();
      return { ok: true, json: async () => ({ success: true, data: { access_token: 'isolated-session', access_expires_at: 2000000000, session: options.ticket?.sessionId ? { sid: options.ticket.sessionId } : undefined } }) };
    },
  });
  const instance = { body, messages, requests, context, listeners, intervals, timers, location, storage, advance: milliseconds => { now += milliseconds; } };
  const injection = source.replace('__ACU_LOGIN_TICKET__', JSON.stringify(ticket));
  vm.runInContext(injection, context);
  return { ...instance, injection, set respond(value) { instance.respond = value; } };
}

for (const platform of ['windows', 'mac']) {
  test(`${platform} login uses a native message, never a loopback network request`, async () => {
    const instance = setup(platform);
    const [text, button] = instance.body.child.children;
    await button.onclick();
    assert.equal(instance.requests.length, 1);
    assert.equal(instance.requests[0].address, '/api/user/auth/refresh');
    assert.equal(instance.requests[0].options.credentials, 'include');
    assert.equal(instance.messages[0].type, 'acu-account-ready');
    assert.equal(instance.messages[1].type, 'acu-account-login');
    assert.equal(instance.messages[1].state, 'isolated-one-use-nonce');
    assert.equal(button.disabled, true);
    instance.listeners['acu-runtime-account-result']({ detail: { ok: true } });
    assert.equal(button.textContent, '已完成');
    assert(text.textContent.includes('返回 ACU Runtime'));
    vm.runInContext(instance.injection, instance.context);
    assert.equal(instance.body.child.children.length, 2);
  });
}

test('login helper is never injected on an unrelated website', () => {
  assert.equal(setup('windows', 'https://evil.test').body.child, undefined);
});

async function connected(options) {
  const instance = setup('windows', 'https://acu.best', 'complete', options);
  instance.respond = () => ({ ok: true, status: 200, json: async () => ({ success: true, data: { access_token: 'first-access', access_expires_at: Math.floor(Date.now() / 1000) + 900, session: { sid: 'same-session' } } }) });
  await instance.body.child.children[1].onclick();
  instance.listeners['acu-runtime-account-result']({ detail: { ok: true } });
  return instance;
}

test('background refresh starts before access expiry and pins the same server session', async () => {
  const instance = await connected();
  instance.intervals[0]();
  assert.equal(instance.requests.length, 1);
  instance.advance(781000);
  instance.listeners['acu-runtime-account-refresh']();
  instance.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(instance.requests.length, 2);
  assert.equal(instance.requests[1].options.headers['X-Auth-Session'], 'same-session');
  assert.equal(instance.messages.at(-1).type, 'acu-account-refresh');
  instance.listeners['acu-runtime-account-result']({ detail: { ok: true } });
  assert.equal(instance.body.child.style.display, 'none');
  assert(![...instance.storage.values()].join('').includes('first-access'));
});

test('native refresh uses the same browser lock as the website', async () => {
  const names = [];
  const instance = await connected({ navigator: { locks: { request: async (name, options, callback) => { names.push(name); assert(options.signal); return callback(); } } } });
  assert.deepEqual(names, ['new-api:auth-refresh']);
  assert.equal(instance.messages.at(-1).type, 'acu-account-login');
});

for (const failure of ['network', 'html', 403, 429, 500, 409]) {
  test(`refresh failure ${failure} backs off without logging out`, async () => {
    const instance = await connected();
    instance.respond = () => {
      if (failure === 'network') throw new TypeError('network unavailable');
      return { ok: false, status: failure, json: async () => { if (failure === 'html') throw new SyntaxError('HTML challenge'); return { success: false, code: 'AUTH_REFRESH_RACE' }; } };
    };
    instance.advance(901000);
    instance.listeners.online();
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(instance.messages.at(-1).type, 'acu-account-refresh-error');
    assert.equal(instance.messages.at(-1).expired, false);
    instance.intervals[0]();
    assert.equal(instance.requests.length, 2);
    instance.advance(15001);
    instance.intervals[0]();
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(instance.requests.length, 3);
  });
}

test('only a confirmed invalid refresh session ends background renewal', async () => {
  const instance = await connected();
  instance.respond = () => ({ ok: false, status: 401, json: async () => ({ success: false, code: 'AUTH_SESSION_REVOKED' }) });
  instance.advance(901000);
  instance.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(instance.messages.at(-1).expired, true);
  assert.equal(instance.storage.size, 0);
  instance.advance(600000);
  instance.listeners.online();
  assert.equal(instance.requests.length, 2);
});

test('native acknowledgement timeout retries and never assumes renewal succeeded', async () => {
  const instance = await connected();
  instance.advance(781000);
  instance.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  instance.timers.at(-1)();
  assert.equal(instance.messages.at(-1).expired, false);
  instance.listeners['acu-runtime-account-refresh']();
  assert.equal(instance.requests.length, 2);
});

test('same-window navigation restores only renewal metadata, never repeats first-Key setup', async () => {
  const instance = await connected();
  const navigated = setup('windows', 'https://acu.best', 'complete', { storage: instance.storage });
  assert.equal(navigated.messages.length, 0);
  navigated.advance(781000);
  navigated.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(navigated.requests.length, 1);
  assert.equal(navigated.requests[0].options.headers['X-Auth-Session'], 'same-session');
  assert(!navigated.messages.some(message => message.type === 'acu-account-login'));
});

test('native validation failure enables retry without exposing tokens', async () => {
  const instance = setup();
  const [text, button] = instance.body.child.children;
  await button.onclick();
  instance.listeners['acu-runtime-account-result']({ detail: { ok: false, error: 'Session expired' } });
  assert.equal(button.disabled, false);
  assert.equal(text.textContent, 'Session expired');
  assert(!text.textContent.includes('isolated-session'));
});

test('ordinary browsers cannot pretend to complete the native connection', async () => {
  const instance = setup('browser');
  assert.equal(instance.body.child, undefined);
  assert.equal(instance.messages.length, 0);
});

test('document-start injection waits for the body and automatically connects after login navigation', async () => {
  const instance = setup('windows', 'https://acu.best', 'loading');
  assert.equal(instance.body.child, undefined);
  instance.listeners.DOMContentLoaded();
  assert.equal(instance.messages[0].type, 'acu-account-ready');
  instance.intervals[0]();
  assert.equal(instance.requests.length, 0);
  instance.location.pathname = '/dashboard';
  instance.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(instance.requests.length, 1);
  assert.equal(instance.messages[1].type, 'acu-account-login');
  instance.intervals[0]();
  assert.equal(instance.requests.length, 1);
});

test('startup restoration immediately refreshes the pinned server session on sign-in pages', async () => {
  const instance = setup('windows', 'https://acu.best', 'complete', { ticket: { restore: true, sessionId: 'restore-session' } });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(instance.requests.length, 1);
  assert.equal(instance.requests[0].options.headers['X-Auth-Session'], 'restore-session');
  assert.equal(instance.messages.at(-1).type, 'acu-account-login');
  assert.equal(instance.messages.at(-1).sessionId, 'restore-session');
});

test('startup network failure remains retryable while revoked sessions stop restoration', async () => {
  const instance = setup('windows', 'https://acu.best', 'complete', { ticket: { restore: true } });
  await new Promise(resolve => setImmediate(resolve));
  instance.listeners['acu-runtime-account-result']({ detail: { ok: false } });
  instance.advance(15001);
  instance.respond = () => { throw new TypeError('network'); };
  instance.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(instance.messages.at(-1).type, 'acu-account-restore-error');
  assert.equal(instance.messages.at(-1).expired, false);
  instance.advance(30001);
  instance.respond = () => ({ ok: false, status: 401, json: async () => ({ success: false, code: 'AUTH_UNAUTHORIZED' }) });
  instance.intervals[0]();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(instance.messages.at(-1).expired, true);
  const requests = instance.requests.length;
  instance.advance(600001);
  instance.intervals[0]();
  assert.equal(instance.requests.length, requests);
});
