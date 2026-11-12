const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const context=vm.createContext({});
vm.runInContext(fs.readFileSync(require('node:path').join(__dirname,'../web/preference-chart.js'),'utf8'),context);
const render=context.acuPreferenceChart;
test('all ordinary directions remain selectable and have finite endpoints',()=>{
  const rays=Array.from({length:100},(_,i)=>({id:String(i),label:'Preference '+i,category:'Domain',score:80,summary:'Keep scope',scope:'Reports'}));
  const html=render({rays},'3',String);
  assert.equal((html.match(/data-pref-ray=/g)||[]).length,100);
  assert.equal((html.match(/<path /g)||[]).length,100);
  assert.doesNotMatch(html,/NaN|Infinity/);
});
test('neutral and nonapplicable bipolar axes stay in accessible selection without arrows',()=>{
  const rays=[60,0,120].map((position,i)=>({id:String(i),label:'Axis '+i,category:'Domain',summary:'Scoped',scope:'Project',bipolar:{position,applicability:i!==2}}));
  const html=render({schemaVersion:'acu-preference-v5-bipolar',rays},'',String);
  assert.equal((html.match(/<option /g)||[]).length,3);
  assert.equal((html.match(/<path /g)||[]).length,1);
});
