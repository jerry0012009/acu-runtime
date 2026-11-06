(() => {
  if (window.parent === window) return;
  const nativeFetch = window.fetch.bind(window);
  window.fetch = async (input, options) => {
    const address = new URL(typeof input === 'string' || input instanceof URL ? input : input.url, location.href);
    const match = address.origin === location.origin && address.pathname.match(/^\/(?:kernel\/)?api\/window\/([a-z]+)$/);
    if (!match) return nativeFetch(input, options);
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
