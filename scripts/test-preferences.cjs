const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { createRequire } = require('node:module');
const root = path.resolve(__dirname, '../..');
const { chromium } = createRequire(path.join(root, '.build/package.json'))('playwright');
const output = path.join(root, 'output/runtime-preferences'); fs.mkdirSync(output, { recursive: true });
let account = { loggedIn: true, identity: 'fixture-one', username: '偏好预览账户', site: 'https://console.acucompute.com' };
let mode = 'normal', delay = false, reads = 0, writes = [];
const summary = '面向终端用户交付软件或浏览器扩展时，以可直接安装、可分发和真实启动验证作为主要验收标准；已明确授权的插件试用应默认具备指定核心能力与可用接入配置。';
const fixtures = {
 profile: { status: 'error', generatedAt: '2026-11-12T12:51:00Z', profile: { summary: '注重可核验的交付、清晰的依据与直接可用的结果。', rays: [{ id: 'hidden-ray', label: '直接安装使用', category: '交付与核验', summary, scope: '面向用户的交付', sourceIds: ['hidden-source', 'missing-source'] }, {id:'initialized',label:'简洁与详尽',summary:'按任务保留合适的细节。',category:'表达',scope:'报告',sourceIds:[],bipolar:{origin:'authorized_initialization'}}] } },
 library: { version: 1, records: [{ id: 'hidden-source', summary }, {id:'english',summary:'LongEnglishText'.repeat(50)+' <script>unsafe()</script>'}] },
 activity: { version: 1, partial: true, moreAvailable: true, events: [{ runId: 'hidden-run', trigger: 'explicit_user_preference', recordedAt: '2026-11-12T12:00:00Z', changes: [{ kind:'updated', before: '交付应可安装。', after: summary }, {kind:'updated',before:'unchanged',after:'unchanged'}] }] },
 settings: {globalEnabled:true,learningEnabled:false,injectionEnabled:false},
};
const server = http.createServer(async (req,res) => {
 const url = new URL(req.url, 'http://localhost');
 const json = data => {res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data));};
 if(req.method !== 'GET') { writes.push(url.pathname); if(url.pathname === '/runtime/account/logout') account = {}; return json({ok:true}); }
 if(url.pathname === '/runtime/status') return json({ready:true,desktop:false,agents:[],connection:{choices:[]},account});
 if(url.pathname.startsWith('/runtime/preferences/')) {
  reads++; const name = url.pathname.split('/').pop(); const data = structuredClone(fixtures[name]);
  if(delay) await new Promise(resolve=>setTimeout(resolve,1800));
  if(mode === 'unsupported') {res.statusCode=404; return json({error:'unsupported'});}
  if(mode === 'revoked') {res.statusCode=401; return json({error:'expired'});}
  if(mode === 'error') {res.statusCode=502; return json({error:'读取失败'});}
  if(mode === 'empty') return json(name==='library'?{version:1,records:[]}:name==='activity'?{version:1,events:[],partial:false}:name==='profile'?{status:'empty',profile:null}:{});
  return json(data);
 }
 const filename = url.pathname==='/' ? 'index.html' : url.pathname.replace('/ui/','');
 if(!/^[\w.-]+$/.test(filename)) {res.statusCode=404;return res.end();}
 try { const data=fs.readFileSync(path.join(root,'acu-runtime/web',filename),'utf8').replace('__ACU_TOKEN__','fixture-local');res.setHeader('Content-Type',filename.endsWith('.js')?'text/javascript':filename.endsWith('.css')?'text/css':'text/html');res.end(data); } catch {res.statusCode=404;res.end();}
});
(async()=>{
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const origin='http://127.0.0.1:'+server.address().port;
 const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 try {
  for(const theme of ['light','dark']) for(const width of [320,390,768,1440]) {
   const page=await browser.newPage({viewport:{width,height:1000},colorScheme:theme});
   const errors=[];page.on('pageerror',error=>errors.push(error.message));
   await page.goto(origin);await page.locator('[data-page="preferences"]').click();await page.getByText('摘要更新',{exact:true}).waitFor();
   assert.equal(await page.locator('body').evaluate(el=>el.scrollWidth<=innerWidth),true,`overflow ${width} ${theme}`);
   assert.equal(await page.getByText('unchanged',{exact:true}).count(),0);
   await page.locator('.preference-change summary').focus();await page.keyboard.press('Enter');
   assert.equal(await page.locator('.preference-change details').getAttribute('open'),'');
   await page.locator('.preference-sources summary').first().click();
   assert(await page.getByText('此条偏好记录暂不可用。',{exact:true}).isVisible());
   await page.screenshot({path:path.join(output,`preferences-${theme}-${width}.png`),fullPage:true});
   await page.locator('#preference-search').fill('不存在的内容');assert(await page.getByText('没有匹配的偏好记录。',{exact:true}).isVisible());
   await page.locator('#preference-search').fill('');
   const count=reads;await page.locator('[data-page="overview"]').click();await page.locator('[data-page="preferences"]').click();assert.equal(reads,count,'60 second cache');
   assert.deepEqual(errors,[]);await page.close();
  }
  for(const nextMode of ['empty','unsupported','error','revoked']) {
   mode=nextMode; const page=await browser.newPage();await page.goto(origin);await page.locator('[data-page="preferences"]').click();
   const text={empty:'暂无可读的偏好记录。',unsupported:'当前服务暂不支持此视图。',error:'读取失败',revoked:'登录后查看我的偏好'}[mode];
   await page.getByText(text,{exact:false}).first().waitFor();assert.equal(await page.getByText(summary,{exact:true}).count(),0);await page.close();
  }
  mode='normal';delay=true;const page=await browser.newPage();await page.goto(origin);await page.locator('[data-page="preferences"]').click();
  await page.waitForTimeout(100);account={...account,identity:'fixture-two',username:'另一账户'};
  // Status polling observes the switch; the old response cannot refill the new session.
  fixtures.library={version:1,records:[]};fixtures.profile={status:'empty',profile:null};fixtures.activity={version:1,events:[]};
  await page.waitForTimeout(6500);assert.equal(await page.getByText(summary,{exact:true}).count(),0);
  await page.locator('[data-pref-logout]').click();await page.getByText('登录后查看我的偏好',{exact:true}).waitFor();assert.deepEqual(writes,['/runtime/account/logout']);
  await page.close();console.log('PASS: 8 theme/viewport layouts, keyboard comparison, sources, search, cache, empty/404/error/revoked, switch and logout.');
 } finally {await browser.close();server.close();}
})().catch(error=>{console.error(error);server.close();process.exitCode=1;});
