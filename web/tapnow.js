(() => {
  globalThis.createACUTapnow = ({ app, ui, request, render, activeChoice, recommendedModels, escapeHTML: esc, icon, toast }) => {
    let state = null, loaded = false, loading = false, operation = '', error = '', model = '', provider = '', timer = null, generation = 0;
    const labels = { companion: '副驾驶', backup: '资产备份' };
    const post = (action, body = {}) => request(`/runtime/tapnow/${action}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const waiting = e => ['waiting', 'saving'].includes(e.connection);
    function schedule() {
      clearTimeout(timer);
      if (!state?.busy && !state?.editions?.some(waiting)) return;
      timer = setTimeout(async () => {
        try { state = await request('/runtime/tapnow'); render(); schedule(); }
        catch (e) { error = e.message; render(); }
      }, 1200);
    }
    async function load() {
      if (loaded || loading) return;
      loaded = true; loading = true;
      try { state = await request('/runtime/tapnow'); error = ''; schedule(); }
      catch (e) { error = e.message; }
      finally { loading = false; render(); }
    }
    function guide(e) {
      if (!e.prepared || e.connection === 'configured') return '';
      const pending = waiting(e);
      return `<div class="tapnow-install-guide" aria-label="${labels[e.edition]}安装进度"><ol class="tapnow-steps"><li class="complete">${icon('check')}<span>插件文件已准备 ${esc(e.prepared)}</span></li><li><span class="tapnow-step-number">2</span><span>在 Chrome 扩展页开启开发者模式，点击“加载已解压的扩展程序”；更新则点击重新加载</span></li><li><span class="tapnow-step-number">3</span><span>选择下方目录，完成后自动配置${e.edition === 'companion' ? ' Key 与模型' : '并确认连接'}</span></li></ol><div class="tapnow-directory"><code class="tapnow-path">${esc(e.path || '')}</code><button class="icon-button" data-tapnow="copy" data-edition="${e.edition}" title="复制插件目录" aria-label="复制${labels[e.edition]}插件目录">${icon('copy')}</button></div><div class="tapnow-actions"><button class="button secondary" data-tapnow="browser" data-edition="${e.edition}">${icon('globe')}打开 Chrome 扩展页</button><button class="button secondary" data-tapnow="open" data-edition="${e.edition}">${icon('folder')}打开插件目录</button>${pending ? `<button class="text-button" data-tapnow="cancel" data-edition="${e.edition}">取消接入</button>` : `<button class="text-button" data-tapnow="connect" data-edition="${e.edition}">已加载，继续接入</button>`}</div><details class="tapnow-help"><summary>没有自动连接？</summary><p>已打开的 TapNow 页面请刷新一次。旧版同类插件请先停用，避免重复显示。扩展页面提示授权过期时，在这里点击重新接入。</p>${pending ? `<button class="text-button" data-tapnow="connect" data-edition="${e.edition}">已加载，重新连接</button>` : ''}</details></div>`;
    }
    function html() {
      const choice = activeChoice();
      if (choice?.id !== provider || !choice?.models?.includes(model)) {
        provider = choice?.id || '';
        model = (choice ? recommendedModels(choice)[0] : '') || choice?.models?.[0] || '';
      }
      const editions = state?.editions || [{ edition:'companion' }, { edition:'backup' }];
      const disabled = e => operation || state?.busy || !ui.status?.desktop || e.edition === 'companion' && (!choice || !model);
      return `<div class="tapnow-agent" data-agent="tapnow"><div class="tapnow-heading"><div>${icon('globe')}<h3>TapNow</h3><span>Chrome 扩展</span></div><button class="icon-button" data-tapnow="check" title="检查插件更新" aria-label="检查插件更新" ${operation || state?.busy ? 'disabled' : ''}>${icon('refresh')}</button></div>
        ${error ? `<p class="tapnow-error" role="alert">${esc(error)} <button class="text-button" data-tapnow="refresh">重试读取状态</button></p>` : ''}${state?.message && !error ? `<p class="tapnow-note" role="status">${esc(state.message)}</p>` : ''}${loading ? '<p role="status">正在读取插件状态</p>' : ''}
        <div class="tapnow-editions">${editions.map(e => {
          const preparing = state?.preparing === e.edition || operation === `configure:${e.edition}` || operation === `install:${e.edition}`;
          const phase = state?.phase === 'preparing-video' ? '正在准备视频读取组件' : state?.phase === 'downloading' ? '正在下载、校验并解压插件' : state?.phase === 'registering' ? '正在准备本机连接组件' : '正在检查插件与连接';
          const connection = preparing ? phase : e.connection === 'configured' ? `配置已确认 · 插件 ${esc(e.version || '')}` : e.connection === 'saving' ? '正在保存并验证配置' : e.connection === 'waiting' ? '文件已就绪，等待 Chrome 加载' : e.connection === 'expired' ? '授权已过期，文件保留，可重新接入' : e.connection === 'error' ? esc(e.connectionError || '配置未完成，请重试') : e.prepared ? '文件已保留，尚未确认接入' : '尚未开启';
          return `<div class="tapnow-edition"><div><h4>${labels[e.edition]}</h4><p>${e.edition === 'backup' ? '更新资产备份插件 · 无需 ACU Key' : '更新插件并替换为当前 ACU API Key 与模型'}</p><p class="tapnow-state" role="status">${connection}</p>${e.reloadRequired ? `<p class="tapnow-note">请先在 Chrome 加载或重新加载 ${esc(e.requiredVersion || '')}，再继续配置。</p>` : ''}</div><div class="tapnow-actions">${preparing ? `<button class="text-button" data-tapnow="cancel" data-edition="${e.edition}">取消准备</button>` : waiting(e) ? '' : `<button class="button ${e.edition === 'companion' ? 'primary' : 'secondary'}" data-tapnow="configure" data-edition="${e.edition}" ${disabled(e) ? 'disabled' : ''}>${icon('refresh')}一键配置${e.update ? ` · ${esc(e.latest)}` : ''}</button>`}</div>${preparing ? '<progress aria-label="插件准备进度"></progress>' : guide(e)}${e.connection === 'configured' ? `<p class="tapnow-note tapnow-complete">${icon('check')}配置已确认；在 Chrome 中打开自己的 TapNow 画布${e.edition === 'companion' ? '，刷新后检测节点' : '，选择备份目录'}。</p>` : ''}</div>`;
        }).join('')}</div>
        ${choice ? `<div class="tapnow-config"><label for="tapnow-model">副驾驶模型</label><select id="tapnow-model" ${operation || state?.busy ? 'disabled' : ''}>${choice.models.map(m => `<option value="${esc(m)}" ${m === model ? 'selected' : ''}>${esc(m)}</option>`).join('')}</select></div><p class="tapnow-note">点击开启或接入即授权安装准备和所选 Key 的本机配置；节点检测可能发送文字与勾选的图片并产生费用。</p>` : '<p class="tapnow-note">副驾驶需先连接 ACU；资产备份可直接开启。</p>'}</div>`;
    }
    app.addEventListener('change', event => { if (event.target.id === 'tapnow-model') model = event.target.value; });
    app.addEventListener('click', async event => {
      const target = event.target.closest?.('[data-tapnow]');
      if (!target || target.disabled) return;
      const action = target.dataset.tapnow, edition = target.dataset.edition;
      if (action === 'copy') {
        try { await navigator.clipboard.writeText(state?.editions.find(e => e.edition === edition)?.path || ''); toast('插件目录已复制'); }
        catch { toast('复制不可用，请选中目录文本复制'); }
        return;
      }
      if (operation && action !== 'cancel' || ui.applying || ui.accountBusy) return;
      const current = ++generation;
      if (['configure', 'install', 'connect', 'check'].includes(action)) operation = `${action}:${edition || ''}`;
      error = ''; render();
      let progressTimer;
      if (action === 'install' || action === 'configure') progressTimer = setInterval(async () => {
        try { const next = await request('/runtime/tapnow'); if (current === generation) { state = next; render(); } } catch { /* final request reports preparation errors */ }
      }, 800);
      try {
        if (action === 'refresh') state = await request('/runtime/tapnow');
        else {
          const choice = activeChoice();
          const body = ['configure', 'install', 'connect'].includes(action) ? { edition, provider: choice?.id, model } : action === 'check' ? {} : { edition };
          const next = await post(action, body);
          if (current === generation) state = next;
        }
        if (action === 'cancel') { operation = ''; toast('接入已取消，插件文件保留'); }
      } catch (e) {
        if (current === generation) { error = e.message; try { state = await request('/runtime/tapnow'); } catch { /* retain last known state */ } }
      } finally {
        clearInterval(progressTimer);
        if (current === generation) operation = '';
        render(); schedule();
      }
    });
    return { html, load };
  };
})();
