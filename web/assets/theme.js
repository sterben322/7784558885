(function () {
  const THEME_KEY = 'lastop-theme';

  function applyTheme(theme) {
    const isDark = theme === 'dark';
    document.body.classList.toggle('dark-theme', isDark);
    document.documentElement.classList.toggle('dark', isDark);
  }

  function currentTheme() {
    return localStorage.getItem(THEME_KEY) || 'light';
  }

  function createToggle() {
    if (document.getElementById('themeToggle')) return;

    const btn = document.createElement('button');
    btn.id = 'themeToggle';
    btn.type = 'button';
    btn.className = 'theme-toggle';
    btn.setAttribute('aria-label', 'Переключить тему');
    btn.innerHTML = '<span class="theme-toggle-track"><span class="theme-toggle-thumb"></span></span><span class="theme-toggle-label">Тёмная тема</span>';

    btn.addEventListener('click', function () {
      const next = document.body.classList.contains('dark-theme') ? 'light' : 'dark';
      localStorage.setItem(THEME_KEY, next);
      applyTheme(next);
    });

    document.body.appendChild(btn);
  }

  document.addEventListener('DOMContentLoaded', function () {
    applyTheme(currentTheme());
    createToggle();
  });
})();
