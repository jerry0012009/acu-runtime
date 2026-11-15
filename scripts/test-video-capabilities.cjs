const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const web = path.resolve(__dirname, '../web');
const output = path.join(process.env.ACU_TEST_WORKSPACE, 'output/runtime-video');
fs.mkdirSync(output, { recursive: true });
(async () => {
 const browser = await chromium.launch({headless:true});
 try {
  for (const colorScheme of ['light','dark']) for (const width of [390,1440]) {
   const page = await browser.newPage({viewport:{width,height:1000},colorScheme});
   let enabled = false, reading = false, videoGen = ''; const writes = [], errors = [];
   page.on('pageerror', error => errors.push(error.message));
   const view = () => ({provider:'acu-runtime',vision:'off',imageGen:'',visionModels:[],imageModels:[],videoGen,videoDefaultSupported:true,videoReadingReady:reading,tools:[{name:'acu-image',agents:['workbuddy']}],searchReady:true,videoEnabled:enabled,videoBlocked:false,videoAuto:'acu-runtime/grok-imagine-video',videoModels:[{id:'acu-runtime/grok-imagine-video',name:'Grok Imagine Video'},{id:'other-provider/video-model',name:'Other video model'}]});
   await page.route('http://video.test/**', async route => {
    const req = route.request(), u = new URL(req.url());
    if (u.pathname === '/runtime/status') return route.fulfill({json:{ready:true,agents:[{id:'workbuddy',name:'WorkBuddy',detected:true,connected:true}],connection:{known:true,configured:true,provider:'acu-runtime',choices:[{id:'acu-runtime',name:'ACU',models:['acu-auto','kimi-k3.1-pro']}]}}});
    if (u.pathname.startsWith('/runtime/capabilities')) {
     if (req.method() === 'POST' && !u.pathname.endsWith('refresh')) { const body=req.postDataJSON(); writes.push(body); if(body.videoEnabled!==undefined)enabled=body.videoEnabled;if(body.videoGen!==undefined)videoGen=body.videoGen;if(body.agents)reading=true; }
     return route.fulfill({json:view()});
    }
    if (u.pathname.startsWith('/runtime/')) return route.fulfill({json:{}});
    const file=path.join(web,u.pathname==='/'?'index.html':u.pathname.replace('/ui/',''));
    if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
    return route.fulfill({contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html',body:fs.readFileSync(file,'utf8').replace('__ACU_TOKEN__','fixture')});
   });
   await page.goto('http://video.test/');
   await page.locator('[data-action="capabilities"]').first().click();
   const panel=page.locator('.capability-video-panel');await panel.waitFor();
   await page.waitForFunction(()=>!document.querySelector('[data-refresh-capability-models]').disabled);
   assert.deepEqual(writes,[]);
   await page.locator('[data-video-enabled]').check();
   await page.waitForFunction(()=>document.querySelector('[data-video-enabled]')?.checked&&!document.querySelector('[data-video-enabled]').disabled);
   assert.deepEqual(writes,[{provider:'acu-runtime',videoEnabled:true}]);
   assert.equal(await panel.locator('textarea, select, [data-copy-video-request]').count(),0);
   assert(!(await panel.innerText()).includes('红色小球'));
   assert(!(await panel.innerText()).includes('生成请求'));
   assert((await panel.innerText()).includes('acu-runtime/grok-imagine-video'));
   assert.equal(writes.length,1);
   await page.locator('#capability-videoGen').selectOption('other-provider/video-model');
   assert.equal(writes.length,1);
   await page.locator('[data-save-capabilities]').click();
   await page.waitForFunction(()=>!document.querySelector('[data-save-capabilities]').disabled || document.querySelector('[data-save-capabilities]').textContent==='保存模型设置');
   assert.deepEqual(writes[1],{provider:'acu-runtime',modelsOnly:true,videoGen:'other-provider/video-model'});
   assert.equal(await page.locator('#capability-videoGen').inputValue(),'other-provider/video-model');
   await page.locator('#capability-videoGen').selectOption('');
   await page.locator('[data-save-capabilities]').click();
   await page.waitForFunction(()=>document.querySelector('[data-save-capabilities]').textContent==='保存模型设置');
   assert.equal(writes[2].videoGen,'');
   const reader=page.locator('.capability-video-reading-panel');
   await reader.locator('[data-configure-capabilities]').click();
   await page.waitForFunction(()=>document.querySelector('.capability-video-reading-panel')?.textContent.includes('已配置'));
   assert.deepEqual(writes[3],{provider:'acu-runtime',agents:['workbuddy']});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:path.join(output,`${colorScheme}-${width}.png`),fullPage:true});
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('PASS: native default video model save/reset across providers, tool toggle, no trial prompts, desktop/mobile themes.');
 } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
