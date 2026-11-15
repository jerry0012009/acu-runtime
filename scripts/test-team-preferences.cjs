const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const root = path.resolve(__dirname,'..');
const output = path.join(process.env.ACU_TEST_WORKSPACE || '/tmp/acu-team-browser','output/team-preferences');
fs.mkdirSync(output,{recursive:true});
const personal={status:'ready',profile:{summary:'个人偏好',rays:[{id:'personal',label:'个人选择',category:'个人领域',score:80,confidence:70,summary:'个人摘要',scope:'个人任务',sourceIds:['personal']}],sourceCount:1,excluded:[]}};
const real=process.env.ACU_TEAM_PROFILE_FILE && JSON.parse(fs.readFileSync(process.env.ACU_TEAM_PROFILE_FILE,'utf8'));
const team={key:'GYZ',view:real || {status:'ready',profile:{summary:'团队创作取舍',rays:[{id:'team',label:'夜景光源主次',category:'光影组织',score:80,confidence:70,summary:'保留暗区与局部光池',scope:'低照度夜景',sourceIds:['team-source']}],sourceCount:1,excluded:[]}},records:[{id:'team-source',summary:'团队来源记录'}]};
let spaces=[team], account={loggedIn:true,identity:'member',username:'member'};
const server=http.createServer((req,res)=>{
 const url=new URL(req.url,'http://localhost');const json=data=>{res.setHeader('content-type','application/json');res.end(JSON.stringify(data));};
 if(url.pathname==='/runtime/status')return json({ready:true,desktop:false,agents:[],connection:{choices:[]},account});
 if(url.pathname.startsWith('/runtime/preferences/')){const name=url.pathname.split('/').pop();return json(name==='teams'?{version:1,spaces}:name==='profile'?personal:name==='library'?{version:1,records:[{id:'personal',summary:'个人来源'}]}:name==='activity'?{version:1,events:[]}:name==='controls'?{records:[]}:{globalEnabled:true,learningEnabled:true,observerEnabled:true,advisorEnabled:true,injectionEnabled:true});}
 const file=url.pathname==='/'?'index.html':url.pathname.replace('/ui/','');if(!/^[\w.-]+$/.test(file)){res.statusCode=404;return res.end();}
 try{res.setHeader('content-type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(path.join(root,'web',file),'utf8').replace('__ACU_TOKEN__','local'));}catch{res.statusCode=404;res.end();}
});
(async()=>{await new Promise(r=>server.listen(0,'127.0.0.1',r));const browser=await chromium.launch({headless:true});try{
 for(const theme of ['light','dark'])for(const width of [390,1440]){
  const page=await browser.newPage({viewport:{width,height:1000},colorScheme:theme});const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.address().port}`);await page.locator('[data-page="preferences"]').click();await page.getByRole('tab',{name:'GYZ',exact:true}).click();
  await page.locator('.starfish-svg').waitFor();assert.equal(await page.locator('.starfish-ray-group').count(),team.view.profile.rays.length);assert.equal(await page.getByRole('switch').count(),0);
  assert.equal(await page.getByText('个人摘要',{exact:true}).count(),0);assert.equal(await page.locator('.starfish-panel').evaluate(el=>!el.closest('details')&&el.getBoundingClientRect().top<innerHeight/2),true);
  await page.locator('[data-star-zoom="in"]').click();await page.locator('[data-star-zoom="out"]').click();
  assert(await page.locator('body').evaluate(el=>el.scrollWidth<=innerWidth));await page.screenshot({path:path.join(output,`${theme}-${width}.png`),fullPage:true});
  await page.getByRole('tab',{name:'我的偏好',exact:true}).click();await page.getByText('个人摘要',{exact:true}).first().waitFor();
  assert.equal(await page.getByRole('switch').count(),5);assert.deepEqual(errors,[]);await page.close();
 }
 const page=await browser.newPage();await page.goto(`http://127.0.0.1:${server.address().port}`);await page.locator('[data-page="preferences"]').click();await page.getByRole('tab',{name:'GYZ',exact:true}).click();
 spaces=[];account={...account,identity:'other'};await page.reload();await page.locator('[data-page="preferences"]').click();await page.getByText('个人摘要',{exact:true}).first().waitFor();assert.equal(await page.getByRole('tab',{name:'GYZ',exact:true}).count(),0);
 console.log('Team graph reuse, source separation, responsive views and account isolation passed.');
}finally{await browser.close();server.close();}})().catch(e=>{console.error(e);process.exitCode=1;server.close();});
