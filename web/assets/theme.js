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

  function createGlobalSidebar() {
    const path = window.location.pathname;
    const isAuthPage = path === '/login' || path === '/login.html' || path === '/register' || path === '/register.html';
    if (isAuthPage) return;
    if (document.querySelector('.dashboard-sidebar')) return;
    if (document.getElementById('globalSidebar')) return;

    const wrapper = document.createElement('aside');
    wrapper.id = 'globalSidebar';
    wrapper.className = 'global-sidebar';

    wrapper.innerHTML = `
      <a href="/" class="global-sidebar__brand">LASTOP</a>
      <nav class="global-sidebar__nav">
        <a href="/" class="global-sidebar__item">Главная</a>
        <a href="/dashboard.html" class="global-sidebar__item">Новости</a>
        <a href="/forum.html" class="global-sidebar__item">Форум</a>
        <a href="/company.html" class="global-sidebar__item">Компании</a>
        <a href="/jobs.html" class="global-sidebar__item">Резюме и вакансии</a>
        <a href="/profile.html" class="global-sidebar__item">Профиль</a>
        <a href="/settings.html" class="global-sidebar__item">Настройки</a>
      </nav>
    `;

    document.body.classList.add('with-global-sidebar');
    document.body.appendChild(wrapper);
  }

  document.addEventListener('DOMContentLoaded', function () {
    applyTheme(currentTheme());
    createGlobalSidebar();
    createToggle();
  });
})();
