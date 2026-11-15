const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

function fixture() {
  const calls = [], timers = [], pending = [];
  const ui = { page: 'home', status: { account: { loggedIn: true, identity: 'first' } } };
  const app = { innerHTML: 'home is usable' };
  const context = vm.createContext({AbortController, setTimeout: (callback, delay) => {timers.push({callback, delay}); return timers.length;}, clearTimeout() {}});
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/preferences.js'), 'utf8'), context);
  const module = context.createACUPreferences({app, ui, navigation: () => '', escapeHTML: String, request: (url, options) => {
    calls.push({url, options});
    return new Promise((resolve, reject) => pending.push({url, resolve, reject}));
  }});
  return {module, calls, timers, pending, ui, app};
}
const settle = () => new Promise(resolve => setImmediate(resolve));
const response = url => url.endsWith('/teams') ? {version:1,spaces:[]} : url.endsWith('/library') ? {version:1,records:[]} : url.endsWith('/activity') ? {version:1,events:[]} : {status:'ready',profile:null};

test('restored login prewarms all preference reads on home without blocking or rerendering it', async () => {
  const f = fixture(); f.module.syncAccount();
  assert.deepEqual(f.calls.map(call=>call.url).sort(), ['activity','controls','library','profile','settings','teams'].map(name=>'/runtime/preferences/'+name).sort());
  assert(f.calls.every(call=>!call.options.method));
  assert.equal(f.app.innerHTML,'home is usable');
  f.module.syncAccount(); assert.equal(f.calls.length,6,'status polling must not duplicate reads');
  f.pending.forEach(item=>item.resolve(response(item.url))); await settle();
  f.module.syncAccount(); assert.equal(f.calls.length,6);
});

test('account changes abort old warmup and late replies cannot schedule old-account refresh', async () => {
  const f = fixture(); f.module.syncAccount();
  const old = f.pending.slice();
  f.ui.status.account.identity='second'; f.module.syncAccount();
  assert(old.every((_,index)=>f.calls[index].options.signal.aborted));
  old.forEach(item=>item.resolve(response(item.url)));await settle();assert.equal(f.timers.length,0);
  f.pending.slice(6).forEach(item=>item.resolve(response(item.url))); await settle();
  assert.equal(f.timers.length,1);
  f.ui.status.account={};f.module.syncAccount(); assert(f.calls.at(-1).options.signal.aborted);
  assert.equal(f.app.innerHTML,'home is usable');
});

test('renewal warmup retries in background with a bounded backoff, and confirmed revocation does not loop', async () => {
  const f = fixture();f.module.syncAccount();
  f.pending.forEach(item=>item.reject(Object.assign(new Error('renewing'),{status:503})));
  await settle();assert.equal(f.timers.at(-1).delay,1500);
  f.timers.at(-1).callback();assert.equal(f.calls.length,12);
  f.pending.slice(6).forEach(item=>item.reject(Object.assign(new Error('revoked'),{status:401})));
  await settle();f.module.syncAccount();assert.equal(f.calls.length,12);
});

test('logged-out startup never fetches preferences',()=>{
  const f=fixture();f.ui.status.account={};f.module.syncAccount();assert.equal(f.calls.length,0);
});

test('startup graph generation polls only three times in the background',async()=>{
  const f=fixture();f.module.syncAccount();
  let offset=0;
  for(let round=0;round<4;round++){
    f.pending.slice(offset).forEach(item=>item.resolve(item.url.endsWith('/profile')?{status:'generating',profile:null}:response(item.url)));
    offset=f.pending.length;await settle();
    assert.equal(f.timers.at(-1).delay,10000);
    f.timers.at(-1).callback();
    assert.equal(f.calls.length,round<3?(round+2)*6:24);
  }
});
