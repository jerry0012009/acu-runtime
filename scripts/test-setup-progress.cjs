const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE);
const web=path.resolve(__dirname,'../web');
const output=path.join(process.env.ACU_TEST_WORKSPACE || '/tmp/acu-setup-browser','output/setup-progress');
fs.mkdirSync(output,{recursive:true});
(async()=>{
 const browser=await chromium.launch({headless:true});
 try {
 for(const theme of ['light','dark']) for(const width of [390,1440]) {
  const page=await browser.newPage({viewport:{width,height:1000},colorScheme:theme});
  let configured=false,phase='video',starts=0;
  const agent={id:'workbuddy',name:'WorkBuddy',detected:true,connected:false,wired:false,modelOptions:[]};
  const task=()=>({id:1,phase,message:phase==='video'?'图片与搜索已就绪 · 正在下载视频组件 · 46%':phase==='error'?'视频准备未完成':'工具已就绪',basicReady:true,...(phase==='error'?{failure:'视频下载失败'}:{})});
  await page.route('http://setup.test/**',async route=>{
   const r=route.request(),u=new URL(r.url());
   if(u.pathname==='/runtime/status')return route.fulfill({json:{ready:true,desktop:false,agents:[{...agent,connected:configured,wired:configured}],connection:{known:true,configured:true,provider:'acu-runtime',choices:[{id:'acu-runtime',name:'ACU',masked:'fixture',models:['acu-auto']}]}}});
   if(u.pathname==='/runtime/models/prepare')return route.fulfill({json:{ok:true}});
   if(u.pathname==='/runtime/agents/configure'){configured=true;return route.fulfill({json:{agents:[{id:'workbuddy',wired:true}]}})}
   if(u.pathname==='/kernel/api/agent-models/workbuddy')return route.fulfill({json:{models:[{id:'acu-runtime/acu-auto'}]}});
   if(u.pathname==='/runtime/capabilities/prepare'){if(r.method()==='POST')starts++;return route.fulfill({json:task()})}
   if(u.pathname.startsWith('/runtime/')||u.pathname.startsWith('/api/'))return route.fulfill({json:{}});
   const file=path.join(web,u.pathname==='/'?'index.html':u.pathname.replace('/ui/',''));
   if(!fs.existsSync(file))return route.fulfill({status:404});
   return route.fulfill({contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html',body:fs.readFileSync(file,'utf8').replace('__ACU_TOKEN__','fixture')});
  });
  await page.goto('http://setup.test');
  await page.locator('[data-action="apply-agents"]').click();await page.locator('[data-confirm-setup]').click();
  await page.getByText('图片与搜索已就绪 · 正在下载视频组件 · 46%',{exact:true}).waitFor();
  await page.waitForFunction(()=>!document.querySelector('[data-action="apply-agents"]')?.disabled);
  assert.equal(await page.locator('[data-action="retry-tools"]').count(),0);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:path.join(output,`${theme}-${width}-pending.png`),fullPage:true});
  await page.getByRole('button',{name:'用量与费用',exact:true}).click();
  await page.getByRole('heading',{name:'用量与费用',exact:true}).waitFor();
  await page.getByRole('button',{name:'工作台',exact:true}).click();
  phase='error';
  await page.locator('[data-action="retry-tools"]').waitFor();
  phase='complete';await page.locator('[data-action="retry-tools"]').click();
  await page.getByText('工具已就绪',{exact:true}).waitFor();
  assert.equal(starts,2);assert.equal(await page.locator('.setup-result.failed').count(),0);
  await page.screenshot({path:path.join(output,`${theme}-${width}-complete.png`),fullPage:true});
  await page.close();
 }
 console.log('PASS: setup returns before video, navigation stays usable, precise background progress and isolated retry across themes/widths.');
 }finally{await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});
