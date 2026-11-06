const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../web/kernel-host.js'), 'utf8');

function setup(tab = 'agents') {
  const messages = [], calls = [], visibility = [];
  const origin = 'http://127.0.0.1:4317';
  const parent = { postMessage: (...args) => messages.push(args) };
  let handler;
  const window = { parent, fetch: async (...args) => { calls.push(args); return new Response('{}'); }, addEventListener: (_, callback) => { handler = callback; }, magpieWindowVisibility: visible => visibility.push(visible), dispatchEvent() {} };
  vm.runInNewContext(source, { window, document: { body: { dataset: { ptab: tab } } }, location: { href: origin + '/kernel/?mode=panel', origin }, URL, URLSearchParams, Response, Event });
  return { window, parent, origin, messages, calls, visibility, message: data => handler(data) };
}

test('window controls stay in Runtime and do not affect the existing core GUI', async () => {
  const state = setup('stats');
  await state.window.fetch('/api/window/main', { method: 'POST' });
  assert.equal(state.messages[0][0].query, 'view=usage&tab=requests');
  assert.equal(state.messages[0][1], state.origin);
  await state.window.fetch('/api/window/quit', { method: 'POST' });
  await state.window.fetch('/api/window/tint', { method: 'POST' });
  assert.equal(state.calls.length, 0);
  assert.equal(state.messages.length, 1);
});

test('model, restore and streaming requests pass through unchanged', async () => {
  const state = setup();
  const options = { method: 'POST', body: '{"agent":"codex"}' };
  for (const route of ['/api/set', '/api/agents/preview/codex', '/api/agents/disconnect/codex', '/api/gateway/trace?wait=1']) await state.window.fetch(route, options);
  assert.equal(state.calls.length, 4);
  assert(state.calls.every(call => call[1] === options));
  assert.equal(state.messages.length, 0);
});

test('fit carries a bounded-input candidate, visibility requires the exact parent origin', async () => {
  const state = setup();
  await state.window.fetch('/api/window/fit?h=610');
  assert.equal(state.messages[0][0].height, 610);
  state.message({ origin: 'https://evil.invalid', source: state.parent, data: { type: 'acu:visibility', visible: false } });
  state.message({ origin: state.origin, source: {}, data: { type: 'acu:visibility', visible: false } });
  assert.equal(state.visibility.length, 0);
  state.message({ origin: state.origin, source: state.parent, data: { type: 'acu:visibility', visible: false } });
  assert.deepEqual(state.visibility, [false]);
});
