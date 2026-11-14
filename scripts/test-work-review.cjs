const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/tmp/acu-understanding-browser/node_modules/playwright');
const root = path.resolve(__dirname, '..');
const output = path.join(process.env.ACU_TEST_WORKSPACE || '/tmp/acu-work-review-browser', 'output/runtime-work-review');
fs.mkdirSync(output, {recursive:true});
let identity = 'one', mode = 'normal', reads = 0;
const fixture = {version:1,unavailable:[],moreAvailable:true,items:[
 {id:'image:1',kind:'image_assistance',occurredAt:'2026-11-14T07:42:00Z',title:'让桌前工作区更突出',summary:'以台灯照亮纸面，减弱屏幕和窗外灯光的抢眼程度。',basis:'GYZ 团队经验',rationale:'按光源作用安排亮度，让夜间工作区成为视觉中心。',state:'guidance_applied',detail:'Make the desk lamp the primary pool of illumination.',sources:[{scope:'team',quote:'根据光源角色分配亮度，保留暗部细节。'}],requestId:'fixture-image'},
 {id:'learning:1',kind:'preference_learning',occurredAt:'2026-11-14T06:30:00Z',title:'偏好记录有更新',summary:'报告先给结论，再展开依据；正式材料保留完整论证。',basis:'你的反馈与选择',state:'preferences_changed',changes:[{kind:'updated',before:'报告先给结论。',after:'报告先给结论，再展开依据；正式材料保留完整论证。'}]},
 {id:'advice:1',kind:'private_advice',occurredAt:'2026-11-13T09:20:00Z',title:'报告顺序参考',summary:'已向后续模型提供报告表达要求。',basis:'个人偏好',state:'advice_delivered',detail:'结论和决策在前，必要依据随后。 <script>window.pwned=true</script>'}
]};
const server=http.createServer(async(req,res)=>{
 const u=new URL(req.url,'http://localhost');const json=(x,status=200)=>{res.statusCode=status;res.setHeader('Content-Type','application/json');res.end(JSON.stringify(x));};
 if(u.pathname==='/runtime/status')return json({ready:true,desktop:false,agents:[],connection:{choices:[]},account:identity?{loggedIn:true,identity,username:'验收账户'}:{}});
 if(u.pathname==='/runtime/preferences/work-review'){
  reads++;const captured=mode==='empty'?{...fixture,items:[]}:structuredClone(fixture);
  if(mode==='delay')await new Promise(r=>setTimeout(r,1200));
  if(mode==='error')return json({error:'unavailable'},502);
  if(mode==='partial')captured.unavailable=['preference_learning'];
  return json(captured);
 }
 if(u.pathname.startsWith('/runtime/'))return json({});
 const file=u.pathname.startsWith('/ui/')?u.pathname.slice(4):'index.html';
 if(file.includes('..')){res.statusCode=404;return res.end();}
 const p=path.join(root,'web',file);if(!fs.existsSync(p)){res.statusCode=404;return res.end();}
 res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(p,'utf8').replace('__ACU_TOKEN__','fixture'));
});
(async()=>{
 await new Promise(r=>server.listen(0,'127.0.0.1',r));const browser=await chromium.launch({headless:true});
 try{
  for(const theme of ['light','dark'])for(const width of [320,390,768,1440]){
   const page=await browser.newPage({viewport:{width,height:900},colorScheme:theme});
   await page.goto(`http://127.0.0.1:${server.address().port}`);
   await page.locator('[data-page="work-review"]').click();
   await page.locator('.review-item').first().waitFor();assert.equal(await page.locator('.review-item').count(),3);
   assert(!await page.locator('body').innerText().then(t=>t.includes('Make the desk lamp')));
   assert(!await page.locator('body').innerText().then(t=>t.includes('更新于')));
   await page.locator('[data-review-detail="image:1"] > summary').click();
   assert(await page.locator('body').innerText().then(t=>t.includes('按光源作用安排亮度')));
   assert(!await page.locator('body').innerText().then(t=>t.includes('Make the desk lamp')));
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
   await page.screenshot({path:path.join(output,`${theme}-${width}.png`),fullPage:true});
   await page.locator('.review-original > summary').click();
   assert(await page.locator('body').innerText().then(t=>t.includes('Make the desk lamp')));
   await page.locator('[data-review-filter="preference_learning"]').click();assert.equal(await page.locator('.review-item').count(),1);
   await page.locator('[data-review-filter="private_advice"]').click();await page.locator('.review-item summary').click();assert.equal(await page.evaluate(()=>window.pwned),undefined);
   await page.close();
  }
  const page=await browser.newPage();await page.goto(`http://127.0.0.1:${server.address().port}`);await page.locator('[data-page="work-review"]').click();await page.locator('.review-item').first().waitFor();
  mode='error';await page.locator('[data-review-refresh]').first().click();await page.getByRole('alert').waitFor();assert.equal(await page.locator('.review-item').count(),3);
  mode='partial';await page.locator('[data-review-refresh]').first().click();await page.getByText('记住的偏好暂未完整读取').waitFor();
  mode='empty';await page.locator('[data-review-refresh]').first().click();await page.getByText('暂无工作记录').waitFor();
  mode='delay';await page.locator('[data-review-refresh]').first().click();identity='';await page.locator('button[data-page="overview"]').click();await page.locator('[data-page="work-review"]').click();await page.getByText('登录后查看工作回顾').waitFor();await page.waitForTimeout(1400);assert.equal(await page.locator('.review-item').count(),0);
  console.log(JSON.stringify({layouts:8,states:['filter','details','error','partial','empty','account-change','escaped-content'],reads,output}));
 }finally{await browser.close();server.close();}
})().catch(e=>{console.error(e);process.exitCode=1;server.close();});
