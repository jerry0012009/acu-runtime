const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { createHash } = require('node:crypto');
const { spawn, execFileSync } = require('node:child_process');
const { chromium } = require(process.env.ACU_PLAYWRIGHT || 'playwright');

async function main() {
  const source = process.env.TAPNOW_TEST_ROOT;
  if (!source) throw Error('Set TAPNOW_TEST_ROOT to built TapNow .output directory');
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tapnow-native-'));
  const env = { ...process.env, HOME: root, USERPROFILE: root, XDG_CONFIG_HOME: path.join(root,'config'),
    GOPATH:execFileSync('go',['env','GOPATH'],{encoding:'utf8'}).trim(),
    GOCACHE:execFileSync('go',['env','GOCACHE'],{encoding:'utf8'}).trim() };
  const output = path.resolve('.build/tapnow-native'); fs.mkdirSync(output,{recursive:true});
  let browser, child, registered = false;
  try {
    const identities = {};
    const extensionPaths = [];
    for (const edition of ['companion','backup']) {
      const target = path.join(root,edition);
      fs.cpSync(path.join(source,edition,'chrome-mv3'),target,{recursive:true});
      const file = path.join(target,'manifest.json');
      const manifest = JSON.parse(fs.readFileSync(file));
      assert(manifest.key, 'ZIP build must contain its stable identity');
      const publicKey = Buffer.from(manifest.key,'base64');
      const hex = createHash('sha256').update(publicKey).digest('hex').slice(0,32);
      const id = [...hex].map(c=>String.fromCharCode(97+parseInt(c,16))).join('');
      identities[edition]=id;
      extensionPaths.push(target);
    }
    const executable = path.join(root,process.platform==='win32'?'native-host.exe':'native-host');
    execFileSync('go',['build','-o',executable,'-ldflags',`-X main.tapnowCompanionID=${identities.companion} -X main.tapnowBackupID=${identities.backup}`,'.'],{stdio:'pipe'});
    for(const flavor of ['chromium','google-chrome','google-chrome-for-testing']){
      const hosts=path.join(env.XDG_CONFIG_HOME,flavor,'NativeMessagingHosts');fs.mkdirSync(hosts,{recursive:true});
      fs.writeFileSync(path.join(hosts,'com.acucompute.tapnow.json'),JSON.stringify({name:'com.acucompute.tapnow',description:'isolated native smoke',path:executable,type:'stdio',allowed_origins:Object.values(identities).map(id=>`chrome-extension://${id}/`)}));
    }
    const profileHosts=path.join(root,'profile','NativeMessagingHosts');fs.mkdirSync(profileHosts,{recursive:true});
    const manifestFile=path.join(profileHosts,'com.acucompute.tapnow.json');
    fs.writeFileSync(manifestFile,JSON.stringify({name:'com.acucompute.tapnow',description:'isolated native smoke',path:executable,type:'stdio',allowed_origins:Object.values(identities).map(id=>`chrome-extension://${id}/`)}));
    if(process.platform==='win32'){
      assert.equal(process.env.CI,'true','Windows registry smoke only runs on disposable CI runners');
      const registryKey='HKCU\\Software\\Google\\Chrome\\NativeMessagingHosts\\com.acucompute.tapnow';
      const {spawnSync}=require('node:child_process');
      assert.notEqual(spawnSync('reg',['query',registryKey]).status,0,'never overwrite an existing native registration');
      execFileSync('reg',['add',registryKey,'/ve','/t','REG_SZ','/d',manifestFile,'/f']);registered=true;
    }
    const done=path.join(root,'done');
    child=spawn('go',['test','-run','^TestTapnowNativeBrowserHarness$','-count=1','-v','.'],{env:{...env,ACU_TAPNOW_BROWSER_FIXTURE:'1',ACU_TAPNOW_COMPANION_ID:identities.companion,ACU_TAPNOW_BACKUP_ID:identities.backup,ACU_TAPNOW_SMOKE_DONE:done},stdio:['ignore','pipe','pipe']});
    let text='';child.stdout.on('data',data=>{text+=data.toString()});child.stderr.on('data',data=>{text+=data.toString()});
    const deadline=Date.now()+60000;
    while(!text.includes('TAPNOW_FIXTURE_READY=')){
      if(child.exitCode!==null||Date.now()>deadline)throw Error('fixture failed: '+text);
      await new Promise(r=>setTimeout(r,100));
    }
    const origin=text.match(/TAPNOW_FIXTURE_READY=(http:\/\/[^\s]+)/)[1];
    const request=async(route,body)=>{
      const res=await fetch(origin+route,{method:body?'POST':'GET',headers:{'X-ACU-Token':'isolated-native-browser-session','Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});
      const result=await res.json();assert(res.ok,JSON.stringify(result));return result;
    };
    browser=await chromium.launchPersistentContext(path.join(root,'profile'),{channel:'chromium',headless:true,env,args:['--no-sandbox',`--disable-extensions-except=${extensionPaths.join(',')}`,`--load-extension=${extensionPaths.join(',')}`]});
    const page=await browser.newPage();let requests=0;
    await browser.route('https://**/*',route=>{requests++;return route.abort();});
    for(const edition of ['companion','backup']){
      await request('/runtime/tapnow/connect',{edition,...(edition==='companion'?{provider:'acu',model:'acu-auto'}:{})});
      await page.goto(`chrome-extension://${identities[edition]}/connect.html`);
      try { await page.waitForFunction(()=>/已配置|已就绪/.test(document.getElementById('status').textContent),null,{timeout:15000}); }
      catch(error){
        const diagnostic=await page.evaluate(async()=>{try{return await chrome.runtime.sendNativeMessage('com.acucompute.tapnow',{action:'connect',version:chrome.runtime.getManifest().version})}catch(e){return {error:e.message}}});
        const local=await page.evaluate(async()=>{const d=await chrome.storage.local.get(null);return {hasKey:!!d.apiKey,managed:d.runtimeConnection,pending:d.runtimePendingReceipt}});
        throw Error(`${edition}: ${await page.locator('#status').innerText()} / ${JSON.stringify(diagnostic)} / ${JSON.stringify(await request('/runtime/tapnow'))} / ${JSON.stringify(local)}`);
      }
      const data=await page.evaluate(async()=>({local:await chrome.storage.local.get(null),sync:await chrome.storage.sync.get(null)}));
      if(edition==='companion'){
        assert.equal(data.local.apiKey,'sk-browser-native-fixture');assert.equal(data.local.runtimeConnection.settings.llmBaseUrl,'https://acu.best/v1');assert.equal(data.sync.apiKey,undefined);
      }else assert.equal(data.local.apiKey,undefined);
      const status=await request('/runtime/tapnow');const view=status.editions.find(e=>e.edition===edition);
      assert.equal(view.connection,'configured');assert(view.confirmedAt);
      assert(!JSON.stringify(status).includes('sk-browser-native-fixture'));
      await page.screenshot({path:path.join(output,`${edition}-connected.png`),fullPage:true});
    }
    assert.equal(requests,0,'connection must not call cloud or inference');
    // Check managed credentials survive extension reload; no inferred success.
    await page.goto(`chrome-extension://${identities.companion}/popup.html`);
    await page.locator('#llmBaseUrl').waitFor();
    assert.equal(await page.locator('#llmBaseUrl').inputValue(),'https://acu.best/v1');
    fs.writeFileSync(done,'ok');
    await new Promise((resolve,reject)=>{if(child.exitCode!==null)return child.exitCode===0?resolve():reject(Error(text));child.once('exit',code=>code===0?resolve():reject(Error(text)));});
    console.log('PASS: actual MV3 -> Chrome native host executable -> authenticated Runtime -> local credential readback -> receipt; Backup has no Key; no cloud calls');
  } finally {
    if(browser)await browser.close();if(child&&child.exitCode===null){child.kill();await new Promise(r=>child.once('exit',r));}
    if(registered)execFileSync('reg',['delete','HKCU\\Software\\Google\\Chrome\\NativeMessagingHosts\\com.acucompute.tapnow','/f']);
    fs.rmSync(root,{recursive:true,force:true});
  }
}
main().catch(error=>{console.error(error);process.exitCode=1});
