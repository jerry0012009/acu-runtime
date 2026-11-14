const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const web = path.resolve(__dirname, '../web');
const output = path.join(process.env.ACU_TEST_WORKSPACE, 'output/capability-refresh');
fs.mkdirSync(output, { recursive: true });
const old = {provider:'acu-runtime',vision:'off',visionModels:[],imageGen:'acu-runtime/gpt-image-2.5-sunburst',imageGenAuto:'',imageModels:[{id:'acu-runtime/gpt-image-2.5-sunburst'}],tools:[],searchReady:true};
const fresh = {...old,imageModels:[...old.imageModels,{id:'acu-runtime/gpt-image-2.6-sunburst'},{id:'acu-runtime/gpt-image-2.6-flare'}]};
(async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  for(const theme of ['light','dark'])for(const width of [390,1440]){
   const page=await browser.newPage({viewport:{width,height:1000},colorScheme:theme});
   const errors=[],writes=[];let fail=false,refreshes=0;
   page.on('pageerror',e=>errors.push(e.message));
   await page.route('http://refresh.test/**',async route=>{
    const req=route.request(),u=new URL(req.url());
    if(u.pathname==='/runtime/status')return route.fulfill({json:{ready:true,agents:[],connection:{known:true,configured:true,provider:'acu-runtime',choices:[{id:'acu-runtime',name:'ACU',models:['acu-auto']}]}}});
    if(u.pathname==='/runtime/capabilities/refresh'){
     refreshes++;assert.deepEqual(req.postDataJSON(),{provider:'acu-runtime'});
     await new Promise(r=>setTimeout(r,350));
     return route.fulfill(fail?{status:502,json:{error:'模型列表未更新，请重试'}}:{json:fresh});
    }
    if(u.pathname==='/runtime/capabilities'){
     if(req.method()==='POST')writes.push(req.postDataJSON());
     return route.fulfill({json:old});
    }
    if(u.pathname.startsWith('/runtime/'))return route.fulfill({json:{}});
    const file=path.join(web,u.pathname==='/'?'index.html':u.pathname.replace('/ui/',''));
    if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
    return route.fulfill({contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html',body:fs.readFileSync(file,'utf8').replace('__ACU_TOKEN__','fixture')});
   });
   await page.goto('http://refresh.test/');
   await page.locator('[data-action="capabilities"]').first().click();
   const select=page.locator('#capability-imageGen');
   await select.waitFor();
   await page.waitForFunction(()=>document.querySelectorAll('#capability-imageGen option').length===5);
   assert.equal(await select.inputValue(),old.imageGen);
   assert.equal(refreshes,1);assert.deepEqual(writes,[]);
   await select.selectOption('acu-runtime/gpt-image-2.6-sunburst');
   fail=true;await page.getByRole('button',{name:'刷新模型',exact:true}).click();
   await page.getByRole('alert').waitFor();
   assert.equal(await page.locator('#capability-imageGen').inputValue(),'acu-runtime/gpt-image-2.6-sunburst');
   fail=false;await page.getByRole('button',{name:'刷新模型',exact:true}).click();
   await page.waitForFunction(()=>!document.querySelector('[data-refresh-capability-models]').disabled);
   assert.equal(await page.locator('#capability-imageGen').inputValue(),'acu-runtime/gpt-image-2.6-sunburst');
   assert.equal(await page.getByRole('alert').count(),0);
   assert.deepEqual(writes,[]);assert.deepEqual(errors,[]);
   assert(await page.locator('body').evaluate(b=>b.scrollWidth<=innerWidth));
   await page.screenshot({path:path.join(output,`${theme}-${width}.png`),fullPage:true});
   await page.close();
  }
  console.log('PASS old 2.5 catalog refreshes to 2.6, picks/drafts preserved, failed refresh retry, themes and widths.');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
