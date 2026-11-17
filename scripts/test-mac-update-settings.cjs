const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const web = path.resolve(__dirname, '../web');
const output = path.join(process.env.ACU_TEST_WORKSPACE || '/tmp/acu-mac-update-settings', 'output/mac-update-settings');
fs.mkdirSync(output, { recursive: true });
(async () => {
 const browser = await chromium.launch({headless:true});
 try {
  for (const colorScheme of ['light','dark']) for (const width of [390,1440]) {
   const page=await browser.newPage({viewport:{width,height:950},colorScheme});
   const errors=[];page.on('pageerror',e=>errors.push(e.message));
   let phase='available',checks=0,installs=0;
   await page.route('http://mac-update.test/**',async route=>{
    const url=new URL(route.request().url());
    if(url.pathname==='/runtime/status')return route.fulfill({json:{desktop:true,platform:'darwin',ready:false,agents:[],account:{},connection:{known:true,choices:[]},update:{supported:true,phase,current:'0.1.113',kernel:'v0.1.4350',release:{version:'0.1.114',notes:'Mac updater fixture'}}}});
    if(url.pathname==='/runtime/update/check'){checks++;return route.fulfill({json:{phase:'available'}});}
    if(url.pathname==='/runtime/update/install'){installs++;phase='downloading';return route.fulfill({json:{phase}});}
    if(url.pathname.startsWith('/runtime/'))return route.fulfill({json:{}});
    const file=path.join(web,url.pathname==='/'?'index.html':url.pathname.replace('/ui/',''));
    if(!fs.existsSync(file))return route.fulfill({status:404});
    await route.fulfill({contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html',body:fs.readFileSync(file,'utf8').replace('__ACU_TOKEN__','fixture')});
   });
   await page.goto('http://mac-update.test/');
   await page.getByRole('button',{name:'立即更新',exact:true}).waitFor();
   await page.locator('#overlay [data-close]').first().click();
   await page.getByRole('button',{name:'设置',exact:true}).click();
   await page.getByRole('button',{name:'更新并重启',exact:true}).waitFor();
   assert.equal(await page.getByRole('heading',{name:'Agent 安装与更新',exact:true}).count(),0);
   assert.equal(await page.getByRole('button',{name:'检查更新',exact:true}).isEnabled(),true);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:path.join(output,`${colorScheme}-${width}.png`),fullPage:true});
   await page.getByRole('button',{name:'更新并重启',exact:true}).click();
   await page.getByRole('button',{name:'立即更新',exact:true}).click();
   await page.waitForFunction(()=>document.body.innerText.includes('正在下载'));
   assert.equal(installs,1);
   assert.deepEqual(errors,[]);
   await page.close();
  }
  console.log('PASS: Mac check/update confirmation starts one install; duplicate Agent section absent; both themes and widths fit.');
 } finally { await browser.close(); }
})().catch(error=>{console.error(error);process.exitCode=1});
