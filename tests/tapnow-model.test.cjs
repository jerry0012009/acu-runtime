const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function fixture(models) {
  const listeners = {};
  let choice = { id:'acu', models };
  const ctx = vm.createContext({ setTimeout, clearTimeout });
  vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/tapnow.js'),'utf8'),ctx);
  const component = ctx.createACUTapnow({app:{addEventListener:(name,fn)=>listeners[name]=fn},ui:{status:{desktop:true}},request:async()=>({}),render(){},activeChoice:()=>choice,escapeHTML:String,icon:()=>'',toast(){}});
  return {component,listeners,setChoice:value=>choice=value};
}

test('TapNow recommends GPT-6.1 Sol independently of the generic acu-auto recommendation',()=>{
  const f=fixture(['acu-auto','gpt-6.1-sol']);
  assert(f.component.html().includes('value="gpt-6.1-sol" selected'));
  assert(!f.component.html().includes('value="acu-auto" selected'));
  f.listeners.change({target:{id:'tapnow-model',value:'acu-auto'}});
  assert(f.component.html().includes('value="acu-auto" selected'),'explicit model remains on rerender');
  f.setChoice({id:'other-key',models:['acu-auto','gpt-6.1-sol']});
  assert(f.component.html().includes('value="gpt-6.1-sol" selected'));
});

test('missing Sol asks for a model instead of silently selecting acu-auto',()=>{
  const f=fixture(['acu-auto']);
  assert(f.component.html().includes('value="" selected disabled'));
  assert(!f.component.html().includes('value="acu-auto" selected'));
  f.listeners.change({target:{id:'tapnow-model',value:'acu-auto'}});
  assert(f.component.html().includes('value="acu-auto" selected'));
});
