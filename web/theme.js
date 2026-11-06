(() => {
  const systemTheme = matchMedia('(prefers-color-scheme: dark)');
  const root = document.documentElement;
  const embedded = location.pathname.startsWith('/kernel/');
  const apply = () => {
    const theme = systemTheme.matches ? 'dark' : 'light';
    root.dataset.acuTheme = theme;
    if (embedded && root.dataset.theme !== theme) root.dataset.theme = theme;
    document.querySelectorAll('[data-system-theme]').forEach(element => {
      element.textContent = `跟随系统 · ${theme === 'dark' ? '深色' : '浅色'}`;
    });
  };
  apply();
  systemTheme.addEventListener('change', apply);
  document.addEventListener('DOMContentLoaded', apply, { once: true });
  window.addEventListener('acu:render', apply);
  if (embedded) {
    new MutationObserver(() => {
      const theme = systemTheme.matches ? 'dark' : 'light';
      if (root.dataset.theme !== theme) root.dataset.theme = theme;
    }).observe(root, { attributes: true, attributeFilter: ['data-theme'] });
  }
})();
