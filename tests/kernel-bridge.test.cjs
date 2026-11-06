const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../web/kernel-bridge.js'), 'utf8');

function setup(action = '', connected = true, agent = 'codex', modelsAvailable = true, detachedEntry = false) {
  const callbacks = new Map();
  const clicks = [];
  const notes = [];
  let sequence = 0;
  let expanded = false;
  const control = name => ({ onclick() {}, click() { clicks.push(name); if (name === 'details') expanded = true; } });
  const target = {
    classList: { add() {} }, scrollIntoView() {},
    append() { clicks.push('attach-native-entry'); },
    querySelector(selector) {
      if (selector === '.ag-conn[aria-checked="true"]' && connected) return control('native-disconnect-confirmation');
      if (selector === '.ag-link[aria-expanded="false"]' && !expanded && !detachedEntry) return control('details');
      if (selector === '.ag-models' && expanded && modelsAvailable) return control('model-picker');
      return null;
    },
  };
  vm.runInNewContext(source, {
    URLSearchParams, Node: { TEXT_NODE: 3, ELEMENT_NODE: 1 }, NodeFilter: { SHOW_TEXT: 4 },
    ...(detachedEntry ? { state: { agents: [{ id: agent, models: { listed: 3 } }] }, modelsEntry: () => control('model-picker') } : {}),
    location: { search: `?acu-view=agents&acu-agent=${agent}&acu-action=${action}` },
    document: {
      body: { dataset: {}, classList: { add() {} }, prepend: element => notes.push(element.textContent) },
      getElementById: () => ({ hidden: false }),
      querySelector: selector => selector.includes('.row.agent') ? target : null,
      createTreeWalker: () => ({ nextNode: () => false }),
      createElement: () => ({ setAttribute() {} }),
    },
    MutationObserver: class { observe() {} disconnect() {} },
    setInterval: callback => { callbacks.set(++sequence, callback); return sequence; },
    clearInterval: id => callbacks.delete(id),
  });
  return { clicks, notes, tick: () => [...callbacks.values()].forEach(callback => callback()) };
}

test('stopping opens the existing native confirmation, never directly disconnects', () => {
  const instance = setup('disconnect');
  instance.tick();
  instance.tick();
  assert.deepEqual(instance.clicks, ['native-disconnect-confirmation']);
  assert.deepEqual(instance.notes, []);
});

test('model settings expands the native Agent component without toggling its route', () => {
  const instance = setup();
  instance.tick(); instance.tick(); instance.tick();
  assert.deepEqual(instance.clicks, ['details']);
});

test('model inventory opens the original picker exactly once without connecting the Agent', () => {
  for (const connected of [true, false]) {
    const instance = setup('models', connected);
    for (let index = 0; index < 45; index++) instance.tick();
    assert.deepEqual(instance.clicks, ['details', 'model-picker']);
    assert.deepEqual(instance.notes, []);
  }
});

test('missing native model picker reports a safe fallback without mutation', () => {
  const instance = setup('models', false, 'workbuddy', false);
  for (let index = 0; index < 45; index++) instance.tick();
  assert.deepEqual(instance.clicks, ['details']);
  assert.equal(instance.notes.length, 1);
  assert(instance.notes[0].includes('尚未修改任何配置'));
});

test('disconnected Agent reuses the native model-entry component without connecting', () => {
  const instance = setup('models', false, 'codex', false, true);
  for (let index = 0; index < 45; index++) instance.tick();
  assert.deepEqual(instance.clicks, ['attach-native-entry', 'model-picker']);
  assert.deepEqual(instance.notes, []);
});

test('an already disconnected or incompatible Agent is never silently reconnected', () => {
  const instance = setup('disconnect', false);
  for (let index = 0; index < 42; index++) instance.tick();
  assert.deepEqual(instance.clicks, []);
  assert.equal(instance.notes.length, 1);
  assert(instance.notes[0].includes('尚未修改任何配置'));
});

test('unrecognized actions and Agent names cannot trigger a native disconnect', () => {
  for (const [action, agent] of [['connect', 'codex'], ['disconnect', 'unexpected']]) {
    const instance = setup(action, true, agent);
    instance.tick(); instance.tick();
    assert(!instance.clicks.includes('native-disconnect-confirmation'));
  }
});
