(() => {
  const parameters = new URLSearchParams(location.search);
  if (parameters.get('acu-developer') === '1') return;
  document.body.classList.add('acu-embedded');
  const view = parameters.get('acu-view') || 'agents';
  const allowed = new Set(['agents', 'providers', 'settings', 'library', 'usage', 'routing', 'plugins', 'masking']);
  if (!allowed.has(view)) return;
  const surface = parameters.get('acu-surface');
  if (surface === 'tray' || surface === 'overview') {
    document.body.classList.add('acu-panel');
    document.body.dataset.acuSurface = surface;
    document.title = 'ACU Runtime';
    const prepare = () => {
      const brand = document.querySelector('header.top .brand');
      if (brand && brand.textContent !== 'ACU Runtime') brand.textContent = 'ACU Runtime';
      for (const [tab, label] of [['agents', 'Agent'], ['stats', '用量'], ['routing', '路由']]) {
        const button = document.querySelector(`#ptabs button[data-ptab="${tab}"] > span`);
        if (button && button.textContent !== label) button.textContent = label;
      }
      const allowance = document.querySelector('#ptabs button[data-ptab="usage"]');
      if (allowance) allowance.hidden = true;
      if (document.body.dataset.ptab === 'usage') document.querySelector('#ptabs button[data-ptab="agents"]')?.click();
      const open = document.getElementById('openMain');
      const caption = surface === 'overview' ? ({ agents: 'Agent 详细设置', stats: '查看用量详情', routing: '查看路由详情' })[document.body.dataset.ptab] || '查看详情' : '打开主窗口';
      if (open && open.textContent !== caption) open.textContent = caption;
      const prefs = document.getElementById('prefs');
      if (prefs) prefs.onclick = () => window.parent.postMessage({ type: 'acu:window', action: 'main', query: 'view=settings' }, location.origin);
    };
    prepare();
    setInterval(prepare, 1500);
    const note = document.createElement('p');
    note.className = 'acu-usage-note';
    note.textContent = '本机统计 · 费用为估算，非 ACU 账单';
    document.getElementById('panelUsage')?.before(note);
  }
  let attempts = 0;
  const navigate = () => {
    const page = document.getElementById(`view-${view}`);
    if (page && !page.hidden) return true;
    const button = document.querySelector(view === 'settings' ? '#prefs' : `[data-view="${view}"]`);
    if (button?.onclick) { button.click(); return true; }
    return false;
  };
  const timer = setInterval(() => { attempts += 1; if (navigate() || attempts > 30) clearInterval(timer); }, 150);
  const scrub = root => {
    if (root.nodeType === Node.TEXT_NODE) {
      if (!root.parentElement?.closest('script,style,input,textarea,pre,code')) root.nodeValue = root.nodeValue.replace(/\bmagpie\b/gi, 'ACU 内核');
      return;
    }
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const pending = [];
    while (walker.nextNode()) {
      const node = walker.currentNode;
      if (!node.parentElement?.closest('script,style,input,textarea,pre,code') && /\bmagpie\b/i.test(node.nodeValue)) pending.push(node);
    }
    pending.forEach(node => { node.nodeValue = node.nodeValue.replace(/\bmagpie\b/gi, 'ACU 内核'); });
  };
  const scrubAttributes = root => {
    if (root.nodeType !== Node.ELEMENT_NODE) return;
    for (const element of [root, ...root.querySelectorAll('[title],[aria-label]')]) {
      for (const attribute of ['title', 'aria-label']) {
        const value = element.getAttribute(attribute);
        if (value && /\bmagpie\b/i.test(value)) element.setAttribute(attribute, value.replace(/\bmagpie\b/gi, 'ACU'));
      }
    }
  };
  scrub(document.body);
  scrubAttributes(document.body);
  const observer = new MutationObserver(records => {
    observer.disconnect();
    for (const record of records) {
      if (record.type === 'attributes') scrubAttributes(record.target);
      else if (record.type === 'characterData') scrub(record.target);
      else for (const node of record.addedNodes) if (node.nodeType === Node.ELEMENT_NODE || node.nodeType === Node.TEXT_NODE) { scrub(node); scrubAttributes(node); }
    }
    observer.observe(document.body, { childList: true, characterData: true, attributes: true, attributeFilter: ['title', 'aria-label'], subtree: true });
  });
  observer.observe(document.body, { childList: true, characterData: true, attributes: true, attributeFilter: ['title', 'aria-label'], subtree: true });
  const focusID = parameters.get('acu-focus');
  if (new Set(['imagesList', 'searchList']).has(focusID)) {
    let count = 0;
    const focusTimer = setInterval(() => {
      count += 1;
      const target = document.getElementById(focusID);
      if (target?.children.length && target.getClientRects().length) { target.scrollIntoView({ block: 'start' }); clearInterval(focusTimer); }
      else if (count > 40) clearInterval(focusTimer);
    }, 200);
  }
  const agent = parameters.get('acu-agent');
  const labels = { codex: 'Codex', claude: 'Claude Code', workbuddy: 'WorkBuddy', hermes: 'Hermes' };
  if (labels[agent]) {
    if (!surface) document.body.dataset.acuAgent = agent;
    let count = 0;
    let expanded = false;
    const disconnect = parameters.get('acu-action') === 'disconnect';
    const models = parameters.get('acu-action') === 'models';
    const focus = setInterval(() => {
      count += 1;
      const target = document.querySelector(`#agents .row.agent[data-id="${agent}"]`);
      if (target) {
        target.classList.add('acu-focused-agent');
        target.scrollIntoView({ block: 'center', behavior: 'smooth' });
        if (disconnect) {
          const stop = target.querySelector('.ag-conn[aria-checked="true"]');
          if (stop?.onclick && !stop.disabled) { clearInterval(focus); stop.click(); return; }
        } else if (models) {
          const picker = target.querySelector('.ag-models') || target.querySelector('.ag-menu') || target.querySelector('.ag-chips .ag-quiet');
          if (picker?.onclick && !picker.disabled) { clearInterval(focus); picker.click(); return; }
          const details = target.querySelector('.ag-link[aria-expanded="false"]');
          if (!expanded && details?.onclick) { expanded = true; details.click(); }
          if (!picker && !details && typeof modelsEntry === 'function' && typeof state !== 'undefined') {
            const nativeAgent = state?.agents?.find(item => item.id === agent);
            if (nativeAgent?.models) {
              const entry = modelsEntry(nativeAgent);
              target.append(entry);
              clearInterval(focus);
              entry.click();
              return;
            }
          }
        } else {
          const details = target.querySelector('.ag-link[aria-expanded="false"]');
          if (!expanded && details?.onclick) { expanded = true; details.click(); return; }
          clearInterval(focus);
          return;
        }
      }
      if (count > 40) {
        clearInterval(focus);
        if (disconnect || models) {
          const message = document.createElement('p');
          message.className = 'acu-control-note';
          message.setAttribute('role', 'status');
          message.textContent = models ? '模型选择器暂未打开，请在对应 Agent 中点击模型数量；尚未修改任何配置。' : '接入状态已变化，或此版本不支持快捷停止。请在下方对应 Agent 中检查；尚未修改任何配置。';
          document.body.prepend(message);
        }
      }
    }, 200);
  }
})();
