(() => {
  if (window.parent === window) return;
  const nativeFetch = window.fetch.bind(window);
  const loginOptions = {
    '': { label: '自动兼容（推荐）', note: '保留 ChatGPT 登录，额度不足时自动使用 ACU。' },
    chatgpt: { label: '仅 ChatGPT', note: '保持 ChatGPT 模式，额度不足时等待恢复。' },
    api: { label: 'ACU API', note: '只显示 ACU 模型，使用 ACU 接入。' },
  };
  const simplifyLogin = data => {
    for (const agent of data?.agents || []) {
      if (agent.id !== 'codex') continue;
      for (const field of agent.fields || []) {
        if (field.key !== 'login') continue;
        const options = field.options || (field.list && data.optionLists?.[field.list - 1]?.map(index => data.options[index]));
        if (!options) continue;
        field.options = options.map(option => loginOptions[option.value] ? { ...option, ...loginOptions[option.value] } : option);
        delete field.list;
      }
    }
    return data;
  };
  window.fetch = async (input, options) => {
    const address = new URL(typeof input === 'string' || input instanceof URL ? input : input.url, location.href);
    const match = address.origin === location.origin && address.pathname.match(/^\/(?:kernel\/)?api\/window\/([a-z]+)$/);
    if (!match) {
      const response = await nativeFetch(input, options);
      if (address.origin !== location.origin || !/^\/(?:kernel\/)?api\//.test(address.pathname) || !response.ok || !response.headers.get('content-type')?.includes('application/json')) return response;
      const data = await response.clone().json().catch(() => null);
      if (!Array.isArray(data?.agents)) return response;
      return new Response(JSON.stringify(simplifyLogin(data)), { status: response.status, statusText: response.statusText, headers: response.headers });
    }
    const action = match[1];
    if (['main', 'fit', 'hide'].includes(action)) {
      const query = new URLSearchParams(address.search);
      if (action === 'main' && !query.has('view')) {
        const tab = document.body?.dataset.ptab;
        const surface = new URLSearchParams(location.search).get('acu-surface');
        query.set('view', tab === 'stats' ? 'usage' : tab === 'routing' ? 'routing' : surface === 'overview' ? 'agents' : 'overview');
        if (tab === 'stats') query.set('tab', 'requests');
      }
      window.parent.postMessage({ type: 'acu:window', action, query: query.toString(), height: Number(query.get('h')) }, location.origin);
    }
    return new Response('{"ok":true}', { status: 200, headers: { 'Content-Type': 'application/json' } });
  };
  window.addEventListener('message', event => {
    if (event.origin !== location.origin || event.source !== window.parent || event.data?.type !== 'acu:visibility') return;
    window.magpieWindowVisibility?.(event.data.visible === true);
    if (event.data.visible) window.dispatchEvent(new Event('focus'));
  });
})();
