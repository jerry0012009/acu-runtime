(() => {
  const frame = document.getElementById('panel');
  const loading = document.getElementById('loading');
  const error = document.getElementById('panel-error');
  const token = document.querySelector('meta[name="acu-token"]').content;
  let visible = !document.hidden;
  let fitTimer;
  const report = message => { error.textContent = message; error.hidden = false; };
  const notifyVisibility = () => frame.contentWindow?.postMessage({ type: 'acu:visibility', visible }, location.origin);
  const action = async command => {
    try {
      const response = await fetch('/runtime/desktop/action', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ACU-Token': token }, body: JSON.stringify(command) });
      if (!response.ok) throw new Error((await response.json()).error);
      error.hidden = true;
    } catch (failure) { report(failure.message || '窗口操作失败，请重试'); }
  };
  window.addEventListener('message', event => {
    if (event.origin !== location.origin || event.source !== frame.contentWindow || event.data?.type !== 'acu:window') return;
    const message = event.data;
    if (message.action === 'fit' && Number.isFinite(message.height) && message.height > 0) {
      clearTimeout(fitTimer);
      fitTimer = setTimeout(() => action({ action: 'fit', height: Math.ceil(message.height) }), 100);
    } else if (message.action === 'main' || message.action === 'hide') action({ action: message.action, query: String(message.query || '') });
  });
  window.addEventListener('acu:visibility', event => { visible = event.detail === true; notifyVisibility(); });
  document.addEventListener('visibilitychange', () => { visible = !document.hidden; notifyVisibility(); });
  window.addEventListener('focus', () => { visible = true; notifyVisibility(); });
  frame.addEventListener('load', notifyVisibility);
  const connect = async () => {
    try {
      const response = await fetch('/runtime/status');
      if (!response.ok) throw new Error('暂时无法连接 ACU');
      const status = await response.json();
      if (status.ready) {
        frame.src = '/kernel/?mode=panel&acu-surface=tray&acu-view=agents';
        frame.hidden = false;
        loading.hidden = true;
        return;
      }
      loading.textContent = status.error || status.phase || '正在准备 ACU…';
    } catch (failure) { loading.textContent = failure.message; }
    setTimeout(connect, 2000);
  };
  const bridge = window.chrome?.webview || window.webkit?.messageHandlers?.external;
  bridge?.postMessage('wails:runtime:ready');
  connect();
})();
