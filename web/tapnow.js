(() => {
  globalThis.createACUTapnow = ({ app, ui, request, render, activeChoice, escapeHTML: esc, icon, toast }) => {
    let state = null, loaded = false, loading = false, operation = '', error = '', model = '', provider = '', timer = null, generation = 0;
    const steps = {}, copied = {}, labels = { companion: '副驾驶', backup: '资产备份（可选）' };
    const extensionsAddress = 'chrome://extensions/';
    const post = (action, body = {}) => request(`/runtime/tapnow/${action}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const waiting = e => ['waiting', 'saving'].includes(e.connection);
    async function copy(value, edition, kind) {
      try { await navigator.clipboard.writeText(value); return true; } catch { /* native clipboard fallback */ }
      try { await post(kind === 'address' ? 'copy-address' : 'copy-path', { edition }); return true; } catch { /* selectable text fallback */ }
      const input = app.querySelector(`[data-tapnow-copy="${edition}-${kind}"]`);
      if (!input) return false;
      input.focus(); input.select();
      try { return document.execCommand('copy'); } catch { return false; }
    }
    function schedule() {
      clearTimeout(timer);
      if (!state?.busy && !state?.editions?.some(e => waiting(e) && (e.edition === 'companion' || steps.backup))) return;
      timer = setTimeout(async () => {
        try { state = await request('/runtime/tapnow'); render(); schedule(); }
        catch (e) { error = e.message; render(); }
      }, 1200);
    }
    async function load() {
      if (loaded || loading) return;
      loaded = true; loading = true;
      try { state = await request('/runtime/tapnow'); schedule(); }
      catch (e) { error = e.message; }
      finally { loading = false; render(); }
    }
    function copyField(e, kind, value) {
      return `<div class="tapnow-directory"><input class="tapnow-address" readonly data-tapnow-copy="${e.edition}-${kind}" ${kind === 'address' ? 'data-tapnow-address' : ''} aria-label="${kind === 'address' ? '扩展页地址' : '插件目录'}" value="${esc(value)}"><button class="icon-button" data-tapnow="${kind === 'address' ? 'copy-address' : 'copy'}" data-edition="${e.edition}" title="复制${kind === 'address' ? '地址' : '目录'}" aria-label="复制${kind === 'address' ? '地址' : '目录'}">${icon('copy')}</button></div>`;
    }
    function guide(e) {
      if (!e.prepared || e.connection === 'configured') return '';
      if (e.edition === 'backup' && !steps.backup) return '';
      const step = steps[e.edition] || (e.reloadRequired ? 'chrome' : 'connect');
      const index = step === 'chrome' ? 0 : step === 'load' ? 1 : 2;
      const button = (action, text, primary = true) => `<button class="button ${primary ? 'primary' : 'secondary'}" data-tapnow="${action}" data-edition="${e.edition}">${text}</button>`;
      let content;
      if (step === 'chrome') {
        content = `<h5>打开 Chrome 扩展页</h5><p>${copied[e.edition] === 'address' ? `地址已复制。在 Chrome 按 ${ui.status?.platform === 'darwin' ? 'Command+L' : 'Ctrl+L'}，粘贴并回车。` : '复制下方地址，在 Chrome 地址栏粘贴并回车。'}</p>${copyField(e,'address',extensionsAddress)}<div class="tapnow-actions">${button('browser','复制地址并打开 Chrome',false)}${button('step-load','已打开，下一步')}</div>`;
      } else if (step === 'load') {
        content = `<h5>在扩展页加载插件</h5><p>打开右上角“开发者模式”，点击左上角“加载已解压的扩展程序”。在目录选择窗口粘贴下方目录并确认。</p>${copyField(e,'path',e.path || '')}<div class="tapnow-actions">${button('open','复制目录并打开文件夹',false)}${button('step-connect','已加载，继续')}</div><p class="tapnow-note">已安装此插件：点击插件卡片上的重新加载按钮。</p>`;
      } else {
        content = `<h5>${e.connection === 'saving' ? '正在连接' : '连接 TapNow 插件'}</h5><p>${e.connection === 'saving' ? e.edition === 'companion' ? '正在保存 Key 和模型，请稍候。' : '正在完成连接，请稍候。' : `打开 Chrome 右上角扩展菜单中的“TapNow ${e.edition === 'companion' ? 'Companion' : 'Backup'}”。弹窗会自动连接；未完成时点击“连接 ACU Runtime”。`}</p>${e.connectionError ? `<p class="tapnow-error">${esc(e.connectionError)}</p>` : ''}<div class="tapnow-actions">${button('connect','重新连接',false)}<button class="text-button" data-tapnow="step-load" data-edition="${e.edition}">重新加载插件</button></div>`;
      }
      return `<div class="tapnow-install-guide" aria-label="${labels[e.edition]}安装进度"><ol class="tapnow-step-tabs">${['打开扩展页','加载插件','连接'].map((text,i)=>`<li ${index === i ? 'aria-current="step"' : ''}><span>${i+1}</span>${text}</li>`).join('')}</ol>${content}<div class="tapnow-guide-footer"><span>文件已准备 · ${esc(e.prepared)}</span><button class="text-button" data-tapnow="cancel" data-edition="${e.edition}">取消</button></div></div>`;
    }
    function html() {
      const choice = activeChoice();
      if (choice?.id !== provider || !choice?.models?.includes(model)) {
        provider = choice?.id || ''; model = choice?.models?.includes('gpt-6.1-sol') ? 'gpt-6.1-sol' : '';
      }
      const editions = state?.editions || [{ edition:'companion' }, { edition:'backup' }];
      const disabled = e => operation || state?.busy || !ui.status?.desktop || e.edition === 'companion' && (!choice || !model);
      return `<div class="tapnow-agent" data-agent="tapnow"><div class="tapnow-heading"><div>${icon('globe')}<h3>TapNow</h3><span>Chrome 扩展</span></div><button class="icon-button" data-tapnow="check" title="检查插件更新" aria-label="检查插件更新" ${operation || state?.busy ? 'disabled' : ''}>${icon('refresh')}</button></div>${error ? `<p class="tapnow-error" role="alert">${esc(error)} <button class="text-button" data-tapnow="refresh">重试</button></p>` : ''}${loading ? '<p role="status">读取中</p>' : ''}
        <div class="tapnow-editions">${editions.map(e => {
          const preparing = state?.preparing === e.edition || operation === `configure:${e.edition}`;
          const phase = state?.phase === 'preparing-video' ? '准备视频读取工具…' : state?.phase === 'downloading' ? '下载并准备插件…' : state?.phase === 'registering' ? '准备连接…' : '检查更新…';
          const status = preparing ? phase : e.connection === 'configured' ? `已连接 · ${esc(e.version || '')}` : e.edition === 'backup' && !steps.backup ? '可独立开启，不影响副驾驶' : e.connection === 'saving' ? '正在连接…' : waiting(e) ? '等待插件确认' : e.connection === 'expired' ? '连接已过期，可重新连接' : e.connection === 'error' ? '连接未完成，请重试' : e.prepared ? '待连接' : '未开启';
          return `<div class="tapnow-edition"><div><h4>${labels[e.edition]}</h4><p>${e.edition === 'backup' ? '画布与素材备份' : `节点审阅${model ? ` · ${esc(model)}` : ''}`}</p><p class="tapnow-state" role="status">${status}</p></div><div class="tapnow-actions">${preparing ? `<button class="text-button" data-tapnow="cancel" data-edition="${e.edition}">取消</button>` : waiting(e) && (e.edition !== 'backup' || steps.backup) ? '' : `<button class="button ${e.edition === 'companion' ? 'primary' : 'secondary'}" data-tapnow="configure" data-edition="${e.edition}" ${disabled(e) ? 'disabled' : ''}>${icon('refresh')}${e.connection === 'configured' ? '更新并配置' : e.edition === 'backup' ? '配置备份（可选）' : '一键配置'}</button>`}</div>${preparing ? '<progress aria-label="插件准备进度"></progress>' : guide(e)}${e.connection === 'configured' ? `<div class="tapnow-complete"><span>${icon('check')}设置完成</span><button class="text-button" data-tapnow="connect" data-edition="${e.edition}" ${disabled(e) ? 'disabled' : ''}>重新连接</button><button class="button secondary" data-tapnow="canvas" data-edition="${e.edition}">${icon('globe')}打开 TapNow</button></div>` : ''}</div>`;
        }).join('')}</div>${choice ? `<div class="tapnow-config"><label for="tapnow-model">副驾驶模型</label><select id="tapnow-model" ${operation || state?.busy ? 'disabled' : ''}>${!model ? '<option value="" selected disabled>请选择模型</option>' : ''}${choice.models.map(m => `<option value="${esc(m)}" ${m === model ? 'selected' : ''}>${esc(m)}</option>`).join('')}</select></div><p class="tapnow-note">节点检测使用所选 API Key，可能产生费用。</p>` : '<p class="tapnow-note">副驾驶需先连接 ACU；资产备份可直接开启。</p>'}</div>`;
    }
    app.addEventListener('change', event => { if (event.target.id === 'tapnow-model') model = event.target.value; });
    app.addEventListener('click', async event => {
      const target = event.target.closest?.('[data-tapnow]'); if (!target || target.disabled) return;
      const action = target.dataset.tapnow, edition = target.dataset.edition;
      const e = state?.editions?.find(item=>item.edition===edition);
      if (operation && action !== 'cancel' || ui.applying || ui.accountBusy) return;
      if (action === 'copy-address' || action === 'copy') {
        const kind=action==='copy'?'path':'address';
        const ok=await copy(kind==='address'?extensionsAddress:e?.path||'',edition,kind);
        copied[edition]=ok?kind:'';
        toast(ok ? kind==='address'?'地址已复制，粘贴到 Chrome 地址栏':'目录已复制，粘贴到目录选择窗口' : '请选中上方内容复制'); render(); return;
      }
      if (action === 'step-load') {
        steps[edition]='load';copied[edition]=await copy(e?.path||'',edition,'path')?'path':'';
        try{state=await post('open',{edition});error='';}catch(err){error=err.message;}
        render();return;
      }
      if (action === 'step-connect') {
        steps[edition]='connect';
        try{const choice=activeChoice();state=await post('connect',{edition,provider:choice?.id,model});error='';}catch(err){error=err.message;}
        render();schedule();return;
      }
      if (['browser','open','canvas'].includes(action)) {
        try {
          if(action!=='canvas'){const kind=action==='browser'?'address':'path';copied[edition]=await copy(kind==='address'?extensionsAddress:e?.path||'',edition,kind)?kind:'';}
          const next=await post(action,{edition});state=next.state||next;error='';
        }catch(err){error=err.message;}
        render();schedule();return;
      }
      const current=++generation;error='';
      if(['configure','connect','check'].includes(action))operation=`${action}:${edition||''}`;
      const canCopy=action==='configure'&&(!e?.prepared||e?.update);
      if(canCopy)copied[edition]=await copy(extensionsAddress,edition,'address')?'address':'';
      render();
      const progressTimer=action==='configure'?setInterval(async()=>{try{const next=await request('/runtime/tapnow');if(current===generation){state=next;render();}}catch{}},800):null;
      try {
        const choice=activeChoice();
        const body=['configure','connect'].includes(action)?{edition,provider:choice?.id,model}:action==='check'?{}:{edition};
        const next=action==='refresh'?await request('/runtime/tapnow'):await post(action,body);
        if(current===generation){state=next;if(action==='configure'){const updated=state.editions.find(item=>item.edition===edition);steps[edition]=updated?.reloadRequired?'chrome':'connect';if(updated?.reloadRequired&&!canCopy)copied[edition]=await copy(extensionsAddress,edition,'address')?'address':'';}if(action==='connect')steps[edition]='connect';if(action==='cancel'){operation='';delete steps[edition];}}
      }catch(err){if(current===generation){error=err.message;try{state=await request('/runtime/tapnow');}catch{}}}
      finally{clearInterval(progressTimer);if(current===generation)operation='';render();schedule();}
    });
    return { html, load };
  };
})();
