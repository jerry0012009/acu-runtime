const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../web/theme.js'), 'utf8');

function setup(dark = false, pathname = '/') {
  const events = new Map();
  const root = { dataset: {} };
  const label = { textContent: '' };
  let observer;
  let onSystemChange;
  const media = {
    matches: dark,
    addEventListener(name, callback) {
      assert.equal(name, 'change');
      onSystemChange = callback;
    },
  };
  vm.runInNewContext(source, {
    matchMedia: query => { assert.equal(query, '(prefers-color-scheme: dark)'); return media; },
    location: { pathname },
    document: {
      documentElement: root,
      querySelectorAll: () => [label],
      addEventListener: (name, callback) => events.set(name, callback),
    },
    window: { addEventListener: (name, callback) => events.set(name, callback) },
    MutationObserver: class {
      constructor(callback) { observer = callback; }
      observe() {}
    },
  });
  return { root, label, events, media, update: () => onSystemChange(), enforce: () => observer() };
}

test('first paint follows light and dark systems', () => {
  for (const dark of [false, true]) {
    const instance = setup(dark);
    assert.equal(instance.root.dataset.acuTheme, dark ? 'dark' : 'light');
    assert.equal(instance.label.textContent, `跟随系统 · ${dark ? '深色' : '浅色'}`);
    assert.equal(instance.root.dataset.theme, undefined);
  }
});

test('system changes and page rerenders update without reloading', () => {
  const instance = setup();
  instance.media.matches = true;
  instance.update();
  assert.equal(instance.root.dataset.acuTheme, 'dark');
  instance.label.textContent = '';
  instance.events.get('acu:render')();
  assert.equal(instance.label.textContent, '跟随系统 · 深色');
  instance.media.matches = false;
  instance.update();
  assert.equal(instance.root.dataset.acuTheme, 'light');
});

test('embedded modules override upstream appearance only in the DOM', () => {
  const instance = setup(true, '/kernel/');
  assert.equal(instance.root.dataset.theme, 'dark');
  instance.root.dataset.theme = 'light';
  instance.enforce();
  assert.equal(instance.root.dataset.theme, 'dark');
  instance.media.matches = false;
  instance.update();
  assert.equal(instance.root.dataset.theme, 'light');
});
