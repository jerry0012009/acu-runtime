import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, unlinkSync, mkdirSync } from 'node:fs';
import assert from 'node:assert/strict';

const [userId, helper] = process.argv.slice(2);
assert(/^\d+$/.test(userId || ''));
assert(/^\/var\/lib\/acu\/releases\/[\w-]+\/session-helper$/.test(helper || ''));
const directory = '/tmp/acu-runtime-settings-acceptance';
mkdirSync(directory, {recursive:true,mode:0o700});
const inspect = id => JSON.parse(execFileSync('docker',['inspect',id],{encoding:'utf8'}))[0];
const routes = JSON.parse(readFileSync('/var/lib/acu/blue-green/routes.json'));
const consoleContainer = inspect(routes['new-api'].container);
const env = Object.fromEntries(consoleContainer.Config.Env.map(value => {
  const index=value.indexOf('=');return [value.slice(0,index),value.slice(index+1)];
}));
const dsn=new URL(env.SQL_DSN);
dsn.hostname=inspect('acu-router-alpha-postgres-newapi-1').NetworkSettings.Networks['acu-router-alpha_alpha-backend'].IPAddress;
const auth=JSON.parse(execFileSync(helper,[],{encoding:'utf8',env:{...process.env,SESSION_SECRET:env.SESSION_SECRET,ACCEPTANCE_DATABASE_URL:dsn.toString(),ACCEPTANCE_USER_ID:userId}}));
const tokenPath=directory+'/auth.private.json';
writeFileSync(tokenPath,JSON.stringify(auth),{mode:0o600});
try {
  const result=execFileSync('go',['test','-run','^TestPreferenceSettingsCloudRoundtrip$','-count=1','-v','.'],{encoding:'utf8',timeout:120000,env:{...process.env,ACU_SETTINGS_ACCEPTANCE_TOKEN:auth.access_token,ACU_SETTINGS_ACCEPTANCE_USER:userId}});
  writeFileSync(directory+'/result.txt',result,{mode:0o600});
  console.log(result);
} finally {
  const base='https://console.acucompute.com';
  const headers={authorization:`Bearer ${auth.access_token}`};
  const logout=await fetch(base+'/api/user/auth/logout',{method:'POST',headers:{...headers,Origin:base}});
  assert.equal((await logout.json()).success,true);
  const check=await fetch(base+'/api/user/self',{headers});
  assert.equal(check.status,401);
  unlinkSync(tokenPath);
  console.log('Acceptance session revoked; verified HTTP 401.');
}
