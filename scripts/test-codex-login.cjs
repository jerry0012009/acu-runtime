const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const assets = process.env.ACU_TEST_KERNEL_ASSETS;
const output = path.join(process.env.ACU_TEST_WORKSPACE, 'output/codex-login');
fs.mkdirSync(output, { recursive: true });
const options = [{value:'',label:'ChatGPT',note:'Long upstream default explanation'}, {value:'chatgpt',label:'Always ChatGPT',note:'Long upstream strict explanation'}, {value:'api',label:'magpie API',note:'Long upstream API explanation'}];
const state = {agents:[{id:'codex',name:'Codex',wired:true,path:'/fixture/config.toml',fields:[{key:'model',label:'model',value:'fixture/model',options:[{value:'fixture/model',label:'ACU model',ref:'fixture/model'}]},{key:'login',label:'sign-in',quiet:true,value:'',options}]}],profiles:[],settings:{lang:'zh',theme:'dark'},clients:[],served:0};
(async()=>{const browser=await chromium.launch({headless:true});try {
 for(const theme of ['dark','light'])for(const width of [390,1440]) {
  const page=await browser.newPage({viewport:{width,height:1000},colorScheme:theme});const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('http://login.test/**',async route=>{
   const u=new URL(route.request().url());
   if(u.pathname==='/api/state')return route.fulfill({json:state});
   if(u.pathname==='/boot.js')return route.fulfill({contentType:'text/javascript',body:`window.bootPrefs={lang:'zh',theme:'${theme}',web:true,version:'0.1.4096'};`});
   if(u.pathname.startsWith('/api/'))return route.fulfill({json:{providers:[],presets:[],excluded:[],gateway:{groups:[],calls:[]}}});
   const local=u.pathname.startsWith('/ui/');
   const file=local?path.join(__dirname,'../web',u.pathname.slice(4)):path.join(assets,u.pathname==='/'?'index.html':u.pathname.slice(1));
   if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
   let body=fs.readFileSync(file,'utf8');
   if(u.pathname==='/')body=body.replace('</head>','<script src="/ui/kernel-host.js"></script><link rel="stylesheet" href="/ui/kernel-embed.css"><script src="/ui/kernel-bridge.js" defer></script></head>');
   return route.fulfill({contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html',body});
  });
  await page.route('http://login.test/host',route=>route.fulfill({contentType:'text/html',body:'<style>body{margin:0}iframe{border:0;width:100%;height:100vh}</style><iframe src="/" title="ACU"></iframe>'}));
  await page.goto('http://login.test/host');
  const native=page.frames().find(frame=>frame.parentFrame());
  await native.waitForFunction(()=>typeof openPicker==='function');
  await native.evaluate(()=>{
   const text=document.createElement('p');text.id='brand-fixture';text.textContent='via magpie';text.title='Magpie settings';document.body.append(text);
   const code=document.createElement('code');code.id='code-fixture';code.textContent='model_provider = "magpie"';document.body.append(code);
   const link=document.createElement('a');link.id='link-fixture';link.href='https://github.com/yetone/magpie';link.textContent='magpie';document.body.append(link);
  });
  await native.waitForFunction(()=>document.querySelector('#brand-fixture').textContent==='via ACU');
  assert.equal(await native.locator('#brand-fixture').getAttribute('title'),'ACU settings');
  assert.equal(await native.locator('#code-fixture').innerText(),'model_provider = "magpie"');
  assert.equal(await native.locator('#link-fixture').getAttribute('href'),'https://github.com/yetone/magpie');
  await native.evaluate(()=>document.querySelectorAll('#brand-fixture,#code-fixture,#link-fixture').forEach(e=>e.remove()));
  await native.evaluate(async()=>{const r=await fetch('/api/state');const d=await r.json();const a=d.agents[0];openPicker(a,a.fields.find(f=>f.key==='login'),document.body,{stopPropagation(){}});});
  await native.getByText('自动兼容（推荐）',{exact:true}).waitFor();
  assert.equal(await native.getByText('ACU API',{exact:true}).count(),1);
  assert.equal(await native.getByText('仅 ChatGPT',{exact:true}).count(),1);
  assert(!await native.locator('body').innerText().then(t=>t.includes('Long upstream')));
  assert(await native.locator('body').evaluate(b=>b.scrollWidth<=innerWidth));
  assert.deepEqual(errors,[]);
  await page.screenshot({path:path.join(output,`login-${theme}-${width}.png`),fullPage:true});
  await page.close();
 }
 console.log('PASS native Codex login picker: concise labels, three values, themes and responsive layout.');
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
