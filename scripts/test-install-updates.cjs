const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const root = process.env.ACU_TEST_WORKSPACE || '/tmp/acu-install-update-browser';
const { chromium } = process.env.PLAYWRIGHT_MODULE ? require(process.env.PLAYWRIGHT_MODULE) : createRequire(path.join(root,'package.json'))('playwright');
const output = path.join(root,'output/runtime-updates');fs.mkdirSync(output,{recursive:true});
const web = path.resolve(__dirname,'../web');
let install = null, detected = false;
const choice = {id:'fixture',name:'ACU',masked:'sk-demo',models:['acu-auto']};
const agent = {id:'codex',name:'Codex',detected:true,connected:true,model:'fixture/acu-auto',modelOptions:[{value:'fixture/acu-auto',ref:'fixture/acu-auto',label:'ACU Auto'}]};
const workbuddy = {id:'workbuddy',name:'WorkBuddy',detected:true,connected:true};
(async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  for(const theme of ['light','dark'])for(const width of [390,1440]){
   const page=await browser.newPage({viewport:{width,height:950},colorScheme:theme});
   const errors=[];page.on('pageerror',error=>errors.push(error.message));
   await page.route('http://install.test/**',async route=>{
    const url=new URL(route.request().url());
    const json=data=>route.fulfill({json:data});
    if(url.pathname==='/runtime/status')return json({desktop:true,platform:'windows',ready:true,account:{},agents:detected?[agent,workbuddy]:[],connection:{known:true,configured:true,provider:'fixture',choices:[choice]},agentInstall:install});
    if(url.pathname==='/runtime/agents/versions')return json({agents:detected?{'workbuddy-desktop':{installed:true,version:'1.0',latest:'2.0',update:true,desktop:true},chatgpt:{installed:true,version:'26.1106.6628.0',latest:'26.1111.2228.0',update:true,desktop:true}}:{}});
    if(url.pathname.startsWith('/runtime/'))return json({});
    const file=path.join(web,url.pathname==='/'?'index.html':url.pathname.replace('/ui/',''));
    if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
    return route.fulfill({contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html',body:fs.readFileSync(file,'utf8').replace('__ACU_TOKEN__','fixture')});
   });
   for(const mode of ['choose','install','update','failed']){
    detected=['update','failed'].includes(mode);
    install=mode==='choose'?null:{agent:detected?'workbuddy':'codex',phase:mode==='failed'?'error':'downloading',update:detected,downloaded:45,total:100,message:mode==='failed'?'下载中断，请重试':'verbose implementation detail'};
    await page.goto('http://install.test/');
    if(mode==='choose')await page.getByRole('button',{name:'安装并配置 ChatGPT',exact:true}).waitFor();
    else if(mode==='install')await page.getByRole('heading',{name:'正在安装 ChatGPT',exact:true}).waitFor();
    else await page.locator('.desktop-update-progress').waitFor();
    if(detected){assert.equal(await page.locator('.agent-install-panel').count(),0);assert.equal(await page.locator('[data-agent="workbuddy"] .desktop-update-progress').count(),1);assert.equal(await page.locator('[data-action="update-agent"][data-agent-id="chatgpt"]').count(),0);}
    if(mode==='install')assert(!(await page.locator('.agent-install-panel').innerText()).includes('WorkBuddy'));
    assert.equal(await page.locator('body').evaluate(node=>node.scrollWidth<=innerWidth),true);
    assert(!(await page.locator('body').innerText()).includes('verbose implementation detail'));
    await page.screenshot({path:path.join(output,`${mode}-${theme}-${width}.png`),fullPage:true});
   }
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('PASS: installation choice/progress and desktop update/error remain separate across themes and mobile/desktop layouts.');
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
