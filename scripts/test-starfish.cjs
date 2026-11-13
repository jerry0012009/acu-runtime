const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {createRequire}=require('node:module');
const root=process.env.ACU_TEST_WORKSPACE||path.resolve(__dirname,'../..');
const {chromium}=process.env.PLAYWRIGHT_MODULE?require(process.env.PLAYWRIGHT_MODULE):createRequire(path.join(root,'.build/package.json'))('playwright');
const source=path.resolve(__dirname,'../web'),output=path.join(root,'output/runtime-starfish');fs.mkdirSync(output,{recursive:true});
let count=3,mode='ordinary',status='ready',identity='starfish-one',loggedIn=true,reads=0;
const origins=['authorized_initialization','learned','user_explicit'];
function fixture(){return {status,error:status==='error'?'profile_refresh_failed':null,generatedAt:'2026-11-12T12:00:00Z',profile:count?{summary:'按任务范围与证据展示偏好方向。',sourceCount:2,schemaVersion:mode==='bipolar'?'acu-preference-v5-bipolar':mode==='unknown'?'future-unrecognized':undefined,acceptanceConditions:['软件交付需要完成启动验证。'],rays:Array.from({length:count},(_,i)=>({id:'ray-'+i,label:'偏好方向 '+i,category:['交付验收与范围','表达与协作','条件和决策'][i%3],score:70+i%21,confidence:72,summary:'偏好 '+i+' 的可核对解释。',scope:'软件维护任务',sourceIds:['source-a','source-b'],...(mode==='bipolar'?{bipolar:{axis_id:'axis-'+i,schema_version:'acu-preference-v5-bipolar',name:'双极轴 '+i,end_a:'简洁说明',end_b:'充分解释',position:[60,0,120,18,102][i%5],acceptable_interval:[18,102],importance:96,confidence:84,applicability:i%17!==3,origin:origins[i%3],scope:{task:'实现',project:'Runtime',subtype:'验收'}}}:{})}))}:null};}
const server=http.createServer((req,res)=>{
 const url=new URL(req.url,'http://localhost');const json=x=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify(x));};
 if(url.pathname==='/runtime/status')return json({ready:true,desktop:false,agents:[],connection:{choices:[]},account:loggedIn?{loggedIn:true,identity,username:'图谱验收账户'}:{}});
 if(url.pathname.startsWith('/runtime/preferences/')){reads++;const name=url.pathname.split('/').pop();return json(name==='profile'?fixture():name==='library'?{version:1,records:[{id:'source-a',summary:'来源 A：交付范围与启动验证。'},{id:'source-b',summary:'来源 B：表达长度按任务调整。'}]}:name==='activity'?{version:1,events:[]}:{learningEnabled:false,injectionEnabled:false});}
 if(url.pathname==='/runtime/account/logout'){loggedIn=false;return json({ok:true});}
 const name=url.pathname==='/'?'index.html':url.pathname.replace('/ui/','');if(!/^[\w.-]+$/.test(name)){res.statusCode=404;return res.end();}
 try{const body=fs.readFileSync(path.join(source,name),'utf8').replace('__ACU_TOKEN__','fixture-only');res.setHeader('Content-Type',name.endsWith('.js')?'text/javascript':name.endsWith('.css')?'text/css':'text/html');res.end(body);}catch{res.statusCode=404;res.end();}
});
(async()=>{
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));const origin='http://127.0.0.1:'+server.address().port;
 const chrome='C:/Program Files/Google/Chrome/Application/chrome.exe';const browser=await chromium.launch({executablePath:process.env.CHROMIUM_PATH||(fs.existsSync(chrome)?chrome:undefined),headless:true});
 async function open(options={}){const page=await browser.newPage(options);await page.goto(origin);await page.locator('[data-page="preferences"]').click();if(count&&mode!=='unknown')await page.getByRole('group',{name:'动态偏好向量海星图'}).waitFor();return page;}
 try{
  for(const n of [0,1,3,100]){count=n;mode='ordinary';const page=await open();if(n){assert.equal(await page.locator('svg .starfish-arrow').count(),n);assert.equal(await page.locator('.starfish-ray-list button').count(),n);}else{await page.getByText('暂无偏好。',{exact:true}).waitFor();assert.equal(await page.locator('.starfish-svg').count(),0);}await page.close();}
  for(const kind of ['ordinary','bipolar'])for(const theme of ['light','dark'])for(const width of [390,1440]){
   count=kind==='bipolar'?192:100;mode=kind;const page=await open({viewport:{width,height:1000},colorScheme:theme});
   assert.equal(await page.locator('html').getAttribute('data-acu-theme'),theme);
   const color=await page.locator('.starfish-viewport').evaluate(node=>getComputedStyle(node).color);assert.equal(color,theme==='dark'?'rgb(255, 255, 255)':'rgb(36, 38, 32)');
   assert.equal(await page.locator('.starfish-ray-list button').count(),count);
   assert.equal(await page.locator('.starfish-arrow').count(),kind==='bipolar'?fixture().profile.rays.filter(r=>r.bipolar.applicability&&r.bipolar.position!==60).length:count);
   assert(await page.locator('.starfish-panel').evaluate(node=>node.getBoundingClientRect().top<innerHeight/2));
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
   assert(await page.locator('.starfish-core').evaluate(node=>getComputedStyle(node).animationName!=='none'));
   const endpoints=await page.locator('.starfish-ray-group').evaluateAll(nodes=>nodes.every(node=>{const line=node.querySelector('.starfish-ray'),d=node.querySelector('path').getAttribute('d'),m=d.match(/ L ([\d.e+-]+) ([\d.e+-]+) L /);return m&&Number(m[1])===Number(line.getAttribute('x2'))&&Number(m[2])===Number(line.getAttribute('y2'));}));assert(endpoints);
   await page.screenshot({path:path.join(output,`${kind}-${theme}-${width}.png`),fullPage:false});await page.close();
  }
  count=3;mode='ordinary';const page=await open({viewport:{width:1120,height:860},colorScheme:'dark'});const detail=page.locator('.starfish-detail');const ray=page.locator('svg [data-star-ray="ray-1"]');
  await ray.hover();await detail.getByRole('heading',{name:'偏好方向 1',exact:true}).waitFor();await ray.click();await page.locator('.starfish-header h2').hover();await detail.getByRole('heading',{name:'偏好方向 1',exact:true}).waitFor();
  await page.locator('svg [data-star-ray="ray-2"]').focus();await page.keyboard.press('Space');await page.locator('.starfish-header h2').hover();await detail.getByRole('heading',{name:'偏好方向 2',exact:true}).waitFor();
  await detail.locator('summary').first().click();await detail.getByText('来源 A：交付范围与启动验证。',{exact:true}).waitFor();await detail.getByText('来源 B：表达长度按任务调整。',{exact:true}).waitFor();
  await page.getByRole('button',{name:'领域：交付验收与范围',exact:true}).focus();await page.keyboard.press('Enter');await detail.getByRole('heading',{name:'偏好方向 0',exact:true}).waitFor();
  const svg=page.locator('.starfish-svg'),oldBox=await svg.getAttribute('viewBox');await page.getByRole('button',{name:'缩小图谱'}).click();assert.notEqual(await svg.getAttribute('viewBox'),oldBox);await page.getByRole('button',{name:'放大图谱'}).click();assert.equal(await svg.getAttribute('viewBox'),oldBox);
  const before=reads;await page.locator('[data-pref-refresh]').click();await page.waitForFunction(()=>!document.querySelector('[data-pref-refresh]').disabled);assert(reads>before,'explicit refresh must bypass the 6-hour cache');
  await page.emulateMedia({reducedMotion:'reduce'});assert.equal(await page.locator('.starfish-core').evaluate(node=>getComputedStyle(node).animationName),'none');await page.close();
  mode='bipolar';count=192;const bipolar=await open();await bipolar.locator('.starfish-list > summary').click();await bipolar.locator('.starfish-ray-list [data-star-ray="ray-0"]').click();await bipolar.locator('.starfish-detail').getByText('无明显倾向',{exact:true}).waitFor();await bipolar.locator('.starfish-ray-list [data-star-ray="ray-3"]').click();await bipolar.locator('.starfish-detail').getByText('当前场景不适用',{exact:true}).waitFor();await bipolar.close();
  mode='ordinary';count=3;const cached=await browser.newPage();await cached.clock.install();await cached.goto(origin);await cached.locator('[data-page="preferences"]').click();await cached.locator('.starfish-svg').waitFor();const cachedReads=reads;
  await cached.clock.fastForward(5*60*60*1000);assert.equal(reads,cachedReads,'five hours must not refresh preference data');
  await cached.clock.fastForward(60*60*1000+100);await cached.waitForFunction(()=>!document.querySelector('[data-pref-refresh]').disabled);assert(reads>cachedReads,'six hours must refresh the visible preference page');await cached.close();
  mode='ordinary';status='error';count=3;const savedError=await open();assert.equal(await savedError.locator('.starfish-arrow').count(),3);await savedError.getByText('刷新失败 · 上次更新',{exact:false}).waitFor();await savedError.close();status='ready';
  mode='unknown';count=3;const unknown=await open();await unknown.getByText('图谱暂不可用，请更新 Runtime。',{exact:true}).waitFor();assert.equal(await unknown.locator('.starfish-svg').count(),0);await unknown.close();
  console.log('PASS: ordinary 0/1/3/100, complete 192 bipolar axes, arrow alignment, hover/pin/keyboard/domain/source linkage, zoom, actual themes, narrow layout, animation and reduced motion; 8 screenshots.');
 }finally{await browser.close();server.close();}
})().catch(error=>{console.error(error);server.close();process.exitCode=1;});
