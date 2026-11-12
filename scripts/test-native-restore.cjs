const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { createRequire } = require('node:module');
const root = process.env.ACU_TEST_WORKSPACE || path.resolve(__dirname, '../..');
const { chromium } = process.env.PLAYWRIGHT_MODULE ? require(process.env.PLAYWRIGHT_MODULE) : createRequire(path.join(root,'.build/package.json'))('playwright');
const profile = fs.mkdtempSync(path.join(os.tmpdir(),'acu-billing-restore-'));
const pause = milliseconds => new Promise(resolve => setTimeout(resolve,milliseconds));
async function until(check) { const deadline = Date.now()+60000; while(Date.now()<deadline) { const value = await check(); if(value) return value; await pause(200); } throw new Error('native restore timed out'); }
async function main() {
  const first = spawn(process.execPath,[path.join(__dirname,'test-native-login.cjs')],{ env:{...process.env,ACU_NATIVE_PROFILE:profile,ACU_NATIVE_INSPECT_PORT:'9237'},windowsHide:true,stdio:'inherit' });
  assert.equal(await new Promise(resolve => first.once('exit',resolve)),0);
  const persisted = fs.readFileSync(path.join(profile,'ACURuntime/account-session.json'),'utf8');
  assert.equal(JSON.parse(persisted).selected,9);
  assert(!persisted.includes('access_token') && !persisted.includes('fixture-model-key'));
  const native = spawn(process.env.ACU_NATIVE_TEST_EXE || path.join(root,'.build/acu-runtime-native-login-smoke.exe'),['-test.run','^TestNativeLoginSmoke$','-test.v'],{ env:{...process.env,ACU_NATIVE_LOGIN_SMOKE:'1',ACU_NATIVE_RESTORE_SMOKE:'1',ACU_NATIVE_PROFILE:profile,ACU_NATIVE_INSPECT_PORT:'9238'},windowsHide:true });
  let output=''; native.stdout.on('data',chunk=>output+=chunk); native.stderr.on('data',chunk=>output+=chunk);
  const exited = new Promise(resolve=>native.once('exit',resolve));
  let browser,page;
  try {
    await until(async()=>{try{return (await fetch('http://127.0.0.1:9238/json/version')).ok;}catch{return false;}});
    browser = await chromium.connectOverCDP('http://127.0.0.1:9238');
    const context = browser.contexts()[0];
    await context.route('http://127.0.0.1:*/runtime/update/check', route => route.fulfill({contentType:'application/json',body:JSON.stringify({phase:'current',current:'0.1.55'})}));
    assert((await context.cookies('https://acu.best')).some(cookie => cookie.name === 'acu-isolated-persistence-check' && cookie.value === 'fixture-only'), 'system WebView did not retain the persistent official-site cookie across restart');
    const refreshes=[];
    await context.route('https://acu.best/**',async route=> {
      if(new URL(route.request().url()).pathname==='/api/user/auth/refresh') {
        refreshes.push(route.request().headers()['x-auth-session']);
        return route.fulfill({contentType:'application/json',body:JSON.stringify({success:true,data:{access_token:'fixture-renewed-access',access_expires_at:Math.floor(Date.now()/1000)+900,session:{sid:'fixture-server-session'}}})});
      }
      return route.fulfill({contentType:'text/html',body:'<!doctype html><title>隔离自动恢复测试</title>'});
    });
    page = await until(()=>context.pages().find(candidate=>candidate.url().startsWith('http://127.0.0.1:')&&!candidate.url().includes('/panel')));
    const login = await until(()=>context.pages().find(candidate=>candidate.url().startsWith('https://acu.best')));
    await login.goto('https://acu.best/');
    const status = async()=>page.evaluate(async()=> (await fetch('/runtime/status')).json());
    await until(async()=>{const value=await status();return value.account?.loggedIn&&value.account.selected===9;});
    assert(refreshes.includes('fixture-server-session'));
    await page.evaluate(()=>fetch('/runtime/desktop/quit',{method:'POST',headers:{'X-ACU-Token':document.querySelector('meta[name="acu-token"]').content}}));
    await browser.close(); browser=null;
    assert.equal(await exited,0,output);
    console.log('PASS: exited and restarted native Runtime restored the pinned account and selected Key, with zero provider writes.');
  } finally {
    if (output) console.log(output);
    if(native.exitCode===null&&page) await page.evaluate(()=>fetch('/runtime/desktop/quit',{method:'POST',headers:{'X-ACU-Token':document.querySelector('meta[name="acu-token"]').content}})).catch(()=>{});
    if(browser) await browser.close().catch(()=>{});
  }
}
main().catch(error=>{console.error(error);process.exitCode=1;});
