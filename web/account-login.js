(() => {
  const ticket = __ACU_LOGIN_TICKET__;
  if (location.origin !== ticket.site || window.top !== window) return;
  const mount = () => {
    if (!document.body || document.getElementById('acu-runtime-connect')) return;
    const bridge = window.chrome?.webview || window.webkit?.messageHandlers?.external;
    if (!bridge?.postMessage) return;
    const send = value => bridge.postMessage(JSON.stringify({ ...value, state: ticket.state }));
    const storageKey = `acu-runtime-session-${ticket.state}`;
    let saved;
    try { saved = JSON.parse(sessionStorage.getItem(storageKey)); } catch {}
    if (!saved) send({ type: 'acu-account-ready' });
    const bar = document.createElement('div');
    bar.id = 'acu-runtime-connect';
    bar.style.cssText = 'position:fixed;bottom:18px;left:50%;transform:translateX(-50%);z-index:2147483647;display:flex;align-items:center;gap:14px;max-width:calc(100% - 32px);padding:14px 18px;border:1px solid #605039;border-radius:12px;background:#252625;color:#f3f3f1;font:13px/1.6 system-ui,sans-serif;box-shadow:0 8px 30px #0005';
    const text = document.createElement('span');
    text.textContent = '登录成功后将自动返回 ACU Runtime。';
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = '已登录？立即连接';
    button.style.cssText = 'border:0;border-radius:7px;padding:10px 14px;background:#eb8d22;color:#241b12;white-space:nowrap;font:inherit;font-weight:600;cursor:pointer';
    let replyTimer;
    let completed = !!saved;
    let stopped = false;
    let sessionId = saved?.sessionId || '';
    let expiresAt = saved?.expiresAt || 0;
    let retryAt = 0;
    let failures = 0;
    let attempts = 0;
    const retry = () => {
      failures++;
      retryAt = Date.now() + Math.min(300000, 15000 * 2 ** Math.min(failures - 1, 5));
      button.disabled = false;
    };
    const connect = async (automatic = false, renewing = completed) => {
      if (button.disabled || stopped || (renewing && Date.now() < retryAt)) return;
      button.disabled = true;
      if (!renewing) text.textContent = '正在连接账号与 API Key…';
      try {
        const request = async () => {
          const headers = { Accept: 'application/json' };
          if (sessionId) headers['X-Auth-Session'] = sessionId;
          const response = await fetch('/api/user/auth/refresh', { method: 'POST', credentials: 'include', headers, signal: AbortSignal.timeout(12000) });
          return { response, result: await response.json() };
        };
        const { response, result } = typeof navigator !== 'undefined' && navigator.locks
          ? await navigator.locks.request('new-api:auth-refresh', { signal: AbortSignal.timeout(15000) }, request)
          : await request();
        if (renewing && ((response.status === 401 && ['AUTH_UNAUTHORIZED', 'AUTH_SESSION_REVOKED', 'AUTH_TOKEN_EXPIRED'].includes(result.code)) || (response.status === 409 && result.code === 'AUTH_SESSION_MISMATCH'))) {
          stopped = true;
          try { sessionStorage.removeItem(storageKey); } catch {}
          send({ type: 'acu-account-refresh-error', expired: true });
          return;
        }
        if (!response.ok || !result.success || !result.data?.access_token) throw new Error('请先完成 ACU 登录，成功后会自动连接。');
        if (!Number.isFinite(result.data.access_expires_at) || result.data.access_expires_at <= Date.now() / 1000) throw new Error('账号续期数据无效，请重试。');
        if (sessionId && result.data.session?.sid !== sessionId) throw new Error('账号会话发生变化，请重新连接。');
        sessionId = result.data.session?.sid || sessionId;
        expiresAt = result.data.access_expires_at;
        send({ type: renewing ? 'acu-account-refresh' : 'acu-account-login', accessToken: result.data.access_token, expiresAt, sessionId });
        replyTimer = setTimeout(() => {
          retry();
          if (renewing) send({ type: 'acu-account-refresh-error', expired: false });
          else text.textContent = 'Runtime 尚未确认连接，请重试或返回应用查看错误。';
        }, 50000);
      } catch (error) {
        retry();
        if (renewing) send({ type: 'acu-account-refresh-error', expired: false });
        else text.textContent = automatic ? '请完成登录；若未自动返回，可点击右侧连接。' : error instanceof TypeError ? '未能连接 ACU 网站，请重试或回到应用直接填 Key。' : error.message;
      }
    };
    window.addEventListener('acu-runtime-account-result', event => {
      clearTimeout(replyTimer);
      if (event.detail?.ok) {
        completed = true;
        failures = 0;
        retryAt = 0;
        button.disabled = false;
        bar.style.display = 'none';
        try { sessionStorage.setItem(storageKey, JSON.stringify({ expiresAt, sessionId })); } catch {}
        text.textContent = '已连接，正在返回 ACU Runtime。';
        button.textContent = '已完成';
      } else {
        text.textContent = event.detail?.error || '连接未完成，请重试。';
        retry();
      }
    });
    button.onclick = () => connect(false);
    bar.append(text, button);
    document.body.append(bar);
    if (completed) bar.style.display = 'none';
    const renew = () => { if (completed && !stopped && Date.now() / 1000 >= expiresAt - 120) connect(true, true); };
    window.addEventListener('acu-runtime-account-refresh', renew);
    window.addEventListener('online', renew);
    window.addEventListener('focus', renew);
    const poll = setInterval(() => {
      if (stopped || (!completed && Date.now() / 1000 >= ticket.expiresAt)) { clearInterval(poll); return; }
      if (completed) { renew(); return; }
      if (!/^\/(sign-in|sign-up|otp|oauth|auth)(\/|$)/.test(location.pathname) && !button.disabled && attempts < 3) { attempts++; connect(true); }
    }, 2000);
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount, { once: true });
  else mount();
})();
