const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { chromium } = require(process.env.ACU_PLAYWRIGHT || process.env.PLAYWRIGHT_MODULE || 'playwright');

const origin = process.env.ACU_TEST_ORIGIN || 'http://127.0.0.1:18761';
const output = path.resolve('.build/tapnow-browser');
fs.mkdirSync(output, { recursive: true });
const status = { ready: true, desktop: true, agents: [
  { id:'codex', name:'Codex', detected:true, connected:false, modelOptions:[{value:'acu/acu-auto',ref:'acu/acu-auto',label:'acu-auto'}] },
  { id:'workbuddy', name:'WorkBuddy', detected:false },
], connection:{ known:true, configured:true, provider:'acu', choices:[{id:'acu',name:'ACU',masked:'sk-***fixture',models:['acu-auto','gpt-6.1-sol']}] }, account:{} };
let installCount = 0, phase = 'unknown', prepared = false, confirmed = false, checks = 0;
const calls=[];
const tapnowState = () => ({editions:['companion','backup'].map(edition => ({edition,installAvailable:true,connection:edition==='companion'?(confirmed?'configured':phase):'unknown',prepared:prepared&&edition==='companion'?'0.1.23':undefined,path:'/isolated/ACURuntime/用户 插件目录/'+edition,version:confirmed?'0.1.23':undefined,confirmedAt:confirmed?'2026-11-14T18:00:00Z':undefined}))});

async function main() {
  const browser = await chromium.launch({ headless:true, args:['--no-sandbox'] });
  const errors=[];
  try {
    const page = await browser.newPage();
    page.on('pageerror',e=>errors.push(e.message));
    await page.route('**/runtime/**', async route=> {
      const url=new URL(route.request().url());
      calls.push(url.pathname);
      let result={};
      if(url.pathname==='/runtime/status') result=status;
      else if(url.pathname==='/runtime/tapnow/configure') {assert.equal(route.request().postDataJSON().model,'gpt-6.1-sol');installCount++;prepared=true;phase='waiting';confirmed=false;result=tapnowState();}
      else if(url.pathname==='/runtime/tapnow/connect') {phase='waiting';result=tapnowState();}
      else if(url.pathname==='/runtime/tapnow/cancel') {phase='unknown';result=tapnowState();}
      else if(url.pathname==='/runtime/tapnow/check') {checks++;result=tapnowState();}
      else if(url.pathname==='/runtime/tapnow/browser') result={state:tapnowState(),address:'chrome://extensions/',manualNavigationRequired:true};
      else if(url.pathname==='/runtime/tapnow/open') result=tapnowState();
      else if(url.pathname==='/runtime/tapnow') result=tapnowState();
      await route.fulfill({json:result});
    });
    await page.goto(origin);
    await page.locator('[data-agent="codex"]').waitFor();
    const other=page.locator('[data-agent-section="other"]');
    assert.equal(await other.getAttribute('open'),null);
    assert.equal(installCount,0);
    await other.locator('summary').click();
    await page.locator('#tapnow-model').waitFor();
    assert.equal(await page.locator('#tapnow-model').inputValue(),'gpt-6.1-sol');
    assert.equal(await page.locator('[data-tapnow-consent]').count(),0,'the enable command is the consent action');
    await page.locator('[data-tapnow="configure"][data-edition="companion"]').click();
    await page.getByText('文件已就绪，等待 Chrome 加载', {exact:false}).waitFor();
    assert.equal(await page.locator('[data-tapnow-address]').inputValue(),'chrome://extensions/');
    assert.equal(await page.getByRole('button',{name:'打开 Chrome 扩展页',exact:true}).count(),0);
    await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.fixtureCopied=text;}}}));
    await page.locator('[data-tapnow="browser"][data-edition="companion"]').click();
    assert(calls.includes('/runtime/tapnow/browser'));
    assert.equal(await page.evaluate(()=>window.fixtureCopied),'chrome://extensions/');
    await page.evaluate(()=>{
      Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{throw Error('denied');}}});
      document.execCommand=()=>false;
    });
    await page.locator('[data-tapnow="copy-address"]').click();
    assert.equal(await page.locator('#toast').innerText(),'复制失败，请选中上方地址复制');
    assert.equal(await page.locator('[data-tapnow-address]').inputValue(),'chrome://extensions/');
    await page.locator('[data-tapnow="open"][data-edition="companion"]').click();
    assert(calls.includes('/runtime/tapnow/open'));
    await page.locator('[data-tapnow="cancel"][data-edition="companion"]').click();
    await page.getByText('文件已保留，尚未确认接入').waitFor();
    await page.locator('[data-tapnow="connect"][data-edition="companion"]').first().click();
    await page.getByText('文件已就绪，等待 Chrome 加载').waitFor();
    assert.equal(installCount,1);
    assert.equal(await other.evaluate(e=>e.open),true,'rerender preserves expansion');
    for(const width of [320,390,768,1440]) for(const theme of ['light','dark']) {
      await page.setViewportSize({width,height:1000});
      await page.emulateMedia({colorScheme:theme});
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`${width} ${theme} overflow`);
      const controls=await page.locator('.tapnow-agent button,.tapnow-agent select').evaluateAll(elements=>elements.map(e=>({w:e.getBoundingClientRect().width,sw:e.scrollWidth,cw:e.clientWidth})));
      assert(controls.every(e=>e.sw<=e.cw+2),`${width} clipped control`);
      await page.screenshot({path:path.join(output,`runtime-${width}-${theme}.png`),fullPage:true});
    }
    confirmed=true;
    await page.locator('.tapnow-state').filter({hasText:'配置已确认'}).waitFor();
    assert.equal(await page.locator('.tapnow-install-guide').count(),0);
    await page.locator('[data-tapnow="check"]').click();
    await page.waitForFunction(()=>!document.querySelector('[data-tapnow="check"]').disabled);
    assert.equal(checks,1);
    await page.locator('[data-tapnow="configure"][data-edition="companion"]').click();
    await page.getByText('文件已就绪，等待 Chrome 加载').waitFor();
    assert.equal(installCount,2,'configured plugin must still check/update on the primary action');
    assert.deepEqual(errors,[]);
    console.log('PASS: collapsed opt-in, single-click authorization without extra checkbox, receipt state; 8 viewport/theme screenshots');
  } finally {await browser.close();}

  const extension=process.env.TAPNOW_TEST_EXTENSION;
  if(!extension) return;
  const profile=fs.mkdtempSync(path.join(os.tmpdir(),'acu-tapnow-browser-'));
  const context=await chromium.launchPersistentContext(profile,{channel:'chromium',headless:true,args:[`--disable-extensions-except=${extension}`,`--load-extension=${extension}`,'--no-sandbox']});
  try {
    const worker=context.serviceWorkers()[0]||await context.waitForEvent('serviceworker');
    const id=new URL(worker.url()).host;
    await worker.evaluate(async()=>chrome.storage.sync.set({requiredTerms:['preserve-team-rule'],llmModel:'original-model'}));
    const page=await context.newPage();
    let network=0;
    page.on('request',r=>{if(/^https?:/.test(r.url()))network++;});
    await page.goto(`chrome-extension://${id}/popup.html`);
    await page.locator('#runtime-config').waitFor({state:'attached'});
    await page.locator('details summary').click();
    const config={kind:'acu-tapnow-companion',schema:1,apiKey:'sk-browser-isolated-key',settings:{llmBaseUrl:'https://api.acucompute.com/v1',llmProtocol:'responses',llmModel:'acu-auto'}};
    await page.locator('#runtime-config').setInputFiles({name:'config.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(config))});
    await page.locator('#runtime-import:not([disabled])').waitFor();
    let stored=await worker.evaluate(async()=>({local:await chrome.storage.local.get(null),sync:await chrome.storage.sync.get(null)}));
    assert.notEqual(stored.local.apiKey,config.apiKey,'preview must not write');
    await page.locator('#runtime-import').click();
    await page.getByText('已导入配置；尚未发送检测请求。请删除配置文件。').waitFor();
    stored=await worker.evaluate(async()=>({local:await chrome.storage.local.get(null),sync:await chrome.storage.sync.get(null)}));
    assert.equal(stored.local.apiKey,config.apiKey);
    assert.equal(stored.sync.apiKey,undefined);
    assert.deepEqual(stored.sync.requiredTerms,['preserve-team-rule']);
    assert.equal(stored.sync.llmModel,'acu-auto');
    await page.locator('#runtime-config').setInputFiles({name:'bad.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({...config,settings:{...config.settings,llmBaseUrl:'https://evil.invalid/v1'}}))});
    await page.getByText('接入配置版本、ACU 地址、Key 或模型无效').waitFor();
    assert(await page.locator('#runtime-import').isDisabled());
    assert.equal(network,0);
    await page.screenshot({path:path.join(output,'companion-import.png'),fullPage:true});
    console.log('PASS: actual unpacked MV3 popup, preview before write, local-only key, rules preserved, hostile URL rejected, zero HTTP requests');
  } finally {await context.close();fs.rmSync(profile,{recursive:true,force:true});}
}
main().catch(error=>{console.error(error);process.exitCode=1;});
