const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const {spawn}=require('node:child_process');
const assert=require('node:assert/strict');
const {chromium}=require(process.env.ACU_PLAYWRIGHT||'playwright');

async function main(){
 const exe=process.env.ACU_CHROME||'/opt/google/chrome/chrome';
 const results=[];
 for(const address of ['chrome://extensions/','about:blank']){
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'tapnow-launch-'));
  let browser;
  const child=spawn(exe,[`--user-data-dir=${root}`,'--remote-debugging-port=0','--no-sandbox','--no-first-run','--no-default-browser-check','--new-window',address],{stdio:'ignore'});
  const exited=new Promise(resolve=>child.once('exit',resolve));
  try{
   let endpoint;
   for(let i=0;i<100;i++){
    try{const lines=fs.readFileSync(path.join(root,'DevToolsActivePort'),'utf8').trim().split('\n');endpoint=`http://127.0.0.1:${lines[0]}`;break;}catch{}
    if(child.exitCode!==null)throw Error('Chrome failed to start');
    await new Promise(r=>setTimeout(r,100));
   }
   assert(endpoint);
   browser=await chromium.connectOverCDP(endpoint);
   const pages=browser.contexts()[0].pages();
   const urls=pages.map(page=>page.url());
   results.push({address,urls,version:browser.version()});
   if(address==='about:blank')assert(urls.includes('about:blank'));
   const session=await browser.newBrowserCDPSession();await session.send('Browser.close').catch(()=>{});await exited;
  }finally{if(browser)await browser.close().catch(()=>{});if(child.exitCode===null){child.kill();await exited;}fs.rmSync(root,{recursive:true,force:true});}
 }
 fs.mkdirSync('.build',{recursive:true});fs.writeFileSync('.build/chrome-launch-probe.json',JSON.stringify(results,null,2));
 console.log(JSON.stringify(results,null,2));
}
main().catch(e=>{console.error(e);process.exitCode=1});
