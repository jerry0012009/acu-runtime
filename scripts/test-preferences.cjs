const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { createRequire } = require('node:module');
 const root = process.env.ACU_TEST_WORKSPACE || path.resolve(__dirname, '../../..');
const sourceRoot = path.resolve(__dirname, '..');
const { chromium } = process.env.PLAYWRIGHT_MODULE ? require(process.env.PLAYWRIGHT_MODULE) : createRequire(path.join(root, '.build/package.json'))('playwright');
const output = path.join(root, 'output/runtime-preferences'); fs.mkdirSync(output, { recursive: true });
let account = { loggedIn: true, identity: 'fixture-one', username: '偏好预览账户', site: 'https://console.acucompute.com' };
let mode = 'normal', delay = false, reads = 0, writes = [];
const summary = '面向终端用户交付软件或浏览器扩展时，以可直接安装、可分发和真实启动验证作为主要验收标准；已明确授权的插件试用应默认具备指定核心能力与可用接入配置。';
const fixtures = {
 controls: {records:[{id:'11111111-1111-4111-8111-111111111111',summary:'先结论再说明',rule:'先结论；法律文书保留完整论证。',scope:'工作报告',exceptions:'法律文书',revision:'a'.repeat(64),disabled:false}]},
 profile: { status: 'error', error:'profile_refresh_failed', generatedAt: '2026-11-12T12:51:00Z', profile: { summary: '注重可核验的交付、清晰的依据与直接可用的结果。', rays: [{ id: 'hidden-ray',score:80,confidence:70, label: '直接安装使用', category: '交付与核验', summary, scope: '面向用户的交付', sourceIds: ['hidden-source', 'missing-source'] }, {id:'initialized',score:70,confidence:60,label:'简洁与详尽',summary:'按任务保留合适的细节。',category:'表达',scope:'报告',sourceIds:[],bipolar:{origin:'authorized_initialization'}}] } },
 library: { version: 1, records: [{ id: 'hidden-source', summary }, {id:'english',summary:'LongEnglishText'.repeat(50)+' <script>unsafe()</script>'}] },
 activity: { version: 1, partial: true, moreAvailable: true, events: [{ runId: 'hidden-run', trigger: 'explicit_user_preference', recordedAt: '2026-11-12T12:00:00Z', changes: [{ kind:'updated', before: '交付应可安装。', after: summary }, {kind:'updated',before:'unchanged',after:'unchanged'}] }] },
 settings: {globalEnabled:true,learningEnabled:false,injectionEnabled:false},
};
fixtures.profile.profile.highlights = [{title:'接入省事，保留选择',summary:'尽可能一键接入，同时保留必要授权和内部选模。',sourceIds:['hidden-source'],evidence:[{sourceId:'hidden-source',quote:'面向终端用户'}]}];
if (process.env.ACU_REAL_PROFILE) fixtures.profile.profile.highlights = JSON.parse(fs.readFileSync(process.env.ACU_REAL_PROFILE,'utf8')).highlights;
const firstHighlight = fixtures.profile.profile.highlights[0].title;
const server = http.createServer(async (req,res) => {
 const url = new URL(req.url, 'http://localhost');
 const json = data => {res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data));};
 if(req.method !== 'GET') {
  if(url.pathname.startsWith('/runtime/preferences/controls/')) {let body='';for await(const chunk of req)body+=chunk;const change=JSON.parse(body);assert.equal(change.expected_hash,'a'.repeat(64));assert.equal(change.rule,'先给决策，再说明原因。');assert.equal(change.exceptions,undefined);assert.equal(change.scope,undefined);fixtures.controls.records[0].rule=change.rule;return json({updated:true});}
  writes.push(url.pathname); if(url.pathname === '/runtime/account/logout') account = {}; return json({ok:true});
 }
 if(url.pathname === '/runtime/status') return json({ready:true,desktop:false,agents:[],connection:{choices:[]},account});
 if(url.pathname.startsWith('/runtime/preferences/')) {
  reads++; const name = url.pathname.split('/').pop(); const data = structuredClone(fixtures[name]);
  if(delay) await new Promise(resolve=>setTimeout(resolve,1800));
  if(mode === 'renewing') {res.statusCode=503; return json({error:'正在续期官网登录，请稍后重试。'});}
  if(mode === 'unsupported') {res.statusCode=404; return json({error:'unsupported'});}
  if(mode === 'revoked') {res.statusCode=401; return json({error:'expired'});}
  if(mode === 'error') {res.statusCode=502; return json({error:'读取失败'});}
  if(mode === 'empty') return json(name==='library'?{version:1,records:[]}:name==='activity'?{version:1,events:[],partial:false}:name==='profile'?{status:'empty',profile:null}:{});
  return json(data);
 }
 const filename = url.pathname==='/' ? 'index.html' : url.pathname.replace('/ui/','');
 if(!/^[\w.-]+$/.test(filename)) {res.statusCode=404;return res.end();}
 try { const data=fs.readFileSync(path.join(sourceRoot,'web',filename),'utf8').replace('__ACU_TOKEN__','fixture-local');res.setHeader('Content-Type',filename.endsWith('.js')?'text/javascript':filename.endsWith('.css')?'text/css':'text/html');res.end(data); } catch {res.statusCode=404;res.end();}
});
(async()=>{
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const origin='http://127.0.0.1:'+server.address().port;
 const installedChrome = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
 const browser=await chromium.launch({executablePath:process.env.CHROMIUM_PATH || (fs.existsSync(installedChrome) ? installedChrome : undefined),headless:true});
 try {
  for(const theme of ['light','dark']) for(const width of [320,390,768,1440]) {
   const page=await browser.newPage({viewport:{width,height:1000},colorScheme:theme});
   const errors=[];page.on('pageerror',error=>errors.push(error.message));
   await page.goto(origin);await page.locator('[data-page="preferences"]').click();await page.getByText(firstHighlight,{exact:true}).waitFor();
   assert.equal(await page.locator('.preference-understanding').innerText().then(text=>text.length<420),true);
   assert.equal(await page.locator('.preference-catalog[open]').count(),0);
   assert.equal(await page.getByText('摘要更新',{exact:true}).isVisible(),false);
   await page.screenshot({path:path.join(output,`preferences-first-${theme}-${width}.png`),fullPage:true});
   await page.locator('.preference-secondary > summary').filter({hasText:'探索偏好图谱'}).click();
   await page.locator('.preference-secondary > summary').filter({hasText:'近期变化'}).click();
   assert.equal(await page.locator('body').evaluate(el=>el.scrollWidth<=innerWidth),true,`overflow ${width} ${theme}`);
   assert.equal(await page.getByText('unchanged',{exact:true}).count(),0);
   assert.equal(await page.locator('.starfish-ray-list button').count(),2);
   await page.locator('.starfish-list > summary').click();
   await page.locator('.starfish-ray-list [data-star-ray]').last().focus();await page.keyboard.press('Enter');
   assert.equal(await page.locator('.starfish-ray-list [data-star-ray]').last().getAttribute('aria-pressed'),'true');
   const catalog=page.locator('.preference-catalog').first();
   if (!(await catalog.evaluate(element=>element.open))) await catalog.locator(':scope > summary').click();
   await page.locator('.preference-change summary').focus();await page.keyboard.press('Enter');
   await page.locator('.preference-change details[open]').waitFor();
   await page.locator('.preference-rays .preference-sources summary').first().click();
   assert(await page.getByText('此条偏好记录暂不可用。',{exact:true}).isVisible());
   await page.screenshot({path:path.join(output,`preferences-${theme}-${width}.png`),fullPage:true});
   await page.locator('#preference-search').fill('不存在的内容');assert(await page.getByText('没有匹配的偏好记录。',{exact:true}).isVisible());
   await page.locator('#preference-search').fill('');
   const count=reads;await page.locator('button[data-page="overview"]').click();await page.locator('[data-page="preferences"]').click();assert.equal(reads,count,'6 hour cache');
   assert.deepEqual(errors,[]);await page.close();
  }
  const editingPage=await browser.newPage();await editingPage.goto(origin);await editingPage.locator('[data-page="preferences"]').click();
  await editingPage.getByText(firstHighlight,{exact:true}).waitFor();
  await editingPage.locator('.preference-secondary > summary').filter({hasText:'管理偏好'}).click();
  await editingPage.locator('.preference-secondary > summary').filter({hasText:'探索偏好图谱'}).click();
  const editing=editingPage.locator('[data-pref-record]').first();await editing.locator('summary').click();
  await editing.locator('[name="rule"]').fill('先给决策，再说明原因。');
  await editingPage.locator('svg [data-star-ray]').last().focus();await editingPage.keyboard.press('Enter');
  assert.equal(await editing.locator('[name="rule"]').inputValue(),'先给决策，再说明原因。');
  await editingPage.locator('#preference-search').fill('找不到');
  await editingPage.locator('#preference-search').fill('');
  assert.equal(await editing.locator('[name="rule"]').inputValue(),'先给决策，再说明原因。');
  assert.equal(await editing.locator('[name="exceptions"]').inputValue(),'法律文书');
  await editing.getByRole('button',{name:'保存修正',exact:true}).click();
  await editing.getByRole('button',{name:'保存修正',exact:true}).waitFor();
  const svg=editingPage.getByRole('group',{name:'动态偏好向量海星图'});const originalView=await svg.getAttribute('viewBox');
  await editingPage.getByRole('button',{name:'缩小图谱',exact:true}).click();assert.notEqual(await svg.getAttribute('viewBox'),originalView);
  await editingPage.getByRole('button',{name:'放大图谱',exact:true}).click();assert.equal(await svg.getAttribute('viewBox'),originalView);
  await editingPage.close();
  for(const nextMode of ['empty','unsupported','error','revoked']) {
   mode=nextMode; const page=await browser.newPage();await page.goto(origin);await page.locator('[data-page="preferences"]').click();
   const text={empty:'暂无可读的偏好记录。',unsupported:'当前服务暂不支持此视图。',error:'读取失败',revoked:'登录后查看我的偏好'}[mode];
   await page.getByText(text,{exact:false}).first().waitFor();assert.equal(await page.getByText(summary,{exact:true}).count(),0);await page.close();
  }
  mode='renewing';const retryPage=await browser.newPage();await retryPage.goto(origin);await retryPage.locator('[data-page="preferences"]').click();await retryPage.getByText('正在续期官网登录，请稍后重试。',{exact:false}).first().waitFor();mode='normal';await retryPage.getByText(firstHighlight,{exact:true}).waitFor({timeout:10000});await retryPage.close();
  mode='normal';delay=true;const page=await browser.newPage();await page.goto(origin);await page.locator('[data-page="preferences"]').click();
  await page.waitForTimeout(100);account={...account,identity:'fixture-two',username:'另一账户'};
  // Status polling observes the switch; the old response cannot refill the new session.
  fixtures.library={version:1,records:[]};fixtures.profile={status:'empty',profile:null};fixtures.activity={version:1,events:[]};fixtures.controls={records:[]};
  await page.waitForTimeout(6500);assert.equal(await page.getByText(summary,{exact:true}).count(),0);
  await page.locator('[data-pref-logout]').click();await page.getByText('登录后查看我的偏好',{exact:true}).waitFor();assert.deepEqual(writes,['/runtime/account/logout']);
  await page.close();console.log('PASS: 8 theme/viewport layouts, keyboard comparison, sources, search, cache, empty/404/error/revoked, switch and logout.');
 } finally {await browser.close();server.close();}
})().catch(error=>{console.error(error);server.close();process.exitCode=1;});
