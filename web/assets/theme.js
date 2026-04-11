(function () {
  const THEME_KEY = 'lastop-theme';
  const DEMO_USER_KEY = 'user';
  const DEMO_TOKEN_KEY = 'token';

  const DEMO_USER = {
    id: 'demo-user-001',
    full_name: 'Анна Карпова',
    email: 'demo@lastop.group',
    company_name: 'LASTOP GROUP',
    position: 'Менеджер по развитию бизнеса',
    phone: '+7 (999) 222-44-11'
  };

  function applyTheme(theme) {
    const isDark = theme === 'dark';
    document.body.classList.toggle('dark-theme', isDark);
    document.documentElement.classList.toggle('dark', isDark);
  }

  function currentTheme() {
    return localStorage.getItem(THEME_KEY) || 'light';
  }

  function ensureDemoSession() {
    const path = window.location.pathname;
    const isAuthPage = path === '/login' || path === '/login.html' || path === '/register' || path === '/register.html';
    if (isAuthPage) return;

    if (!localStorage.getItem(DEMO_TOKEN_KEY)) {
      localStorage.setItem(DEMO_TOKEN_KEY, 'demo-token-local');
    }

    const storedUser = localStorage.getItem(DEMO_USER_KEY);
    if (!storedUser) {
      localStorage.setItem(DEMO_USER_KEY, JSON.stringify(DEMO_USER));
      return;
    }

    try {
      const parsed = JSON.parse(storedUser);
      if (!parsed || !parsed.full_name) {
        localStorage.setItem(DEMO_USER_KEY, JSON.stringify(DEMO_USER));
      }
    } catch (e) {
      localStorage.setItem(DEMO_USER_KEY, JSON.stringify(DEMO_USER));
    }
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

  function logoMarkup() {
    return '<img src="/assets/lastop-group-logo.svg" alt="LASTOP GROUP" class="brand-logo" />';
  }

  function applyBranding() {
    document.querySelectorAll('a').forEach((anchor) => {
      const text = (anchor.textContent || '').trim().toUpperCase();
      if (text === 'LASTOP' || text === 'LASTOP GROUP') {
        anchor.setAttribute('href', '/');
        anchor.classList.add('brand-link');
        anchor.innerHTML = logoMarkup();
      }
    });

    document.querySelectorAll('.dashboard-sidebar a[href="/"], .dashboard-sidebar a[href="/dashboard.html"]').forEach((a) => {
      if (!a.closest('.space-y-1\.5')) {
        a.setAttribute('href', '/');
        a.classList.add('brand-link');
        a.innerHTML = logoMarkup();
      }
    });
  }

  function simplifyLeftSidebar() {
    document.querySelectorAll('.dashboard-sidebar #userAvatar').forEach((avatar) => {
      const block = avatar.closest('div.border-b');
      if (block) block.remove();
    });
  }

  function createGlobalSidebar() {
    const path = window.location.pathname;
    const isAuthPage = path === '/login' || path === '/login.html' || path === '/register' || path === '/register.html';
    if (isAuthPage) return;
    if (document.querySelector('.dashboard-shell')) return;
    if (document.getElementById('globalSidebar')) return;

    const wrapper = document.createElement('aside');
    wrapper.id = 'globalSidebar';
    wrapper.className = 'global-sidebar';

    wrapper.innerHTML = `
      <a href="/" class="global-sidebar__brand brand-link">${logoMarkup()}</a>
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

    const right = document.createElement('aside');
    right.id = 'globalRightSidebar';
    right.className = 'global-right-sidebar';
    right.innerHTML = `
      <div class="rounded-[24px] border border-[#e8ecee] bg-white p-5 shadow-[0_10px_20px_rgba(178,190,198,0.10)]">
        <h3 class="text-[18px] font-semibold text-[#334153]">Быстрые переходы</h3>
        <div class="mt-4 space-y-3 text-[15px] text-[#5f6b79]">
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/dashboard.html">Новости</a>
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/profile.html">Мой профиль</a>
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/profile.html#friends">Друзья</a>
        </div>
      </div>`;

    const search = document.createElement('div');
    search.id = 'globalSearchBar';
    search.className = 'global-search';
    search.innerHTML = '<i class="fa-solid fa-magnifying-glass h-5 w-5 text-[#95a1ad]"></i><input class="w-full bg-transparent text-[15px] text-[#324154] outline-none placeholder:text-[#a1acb5]" placeholder="Поиск по разделам" />';

    document.body.classList.add('with-global-sidebar', 'with-global-sidebar-right', 'with-global-search');
    document.body.appendChild(wrapper);
    document.body.appendChild(right);
    document.body.appendChild(search);
  }

  document.addEventListener('DOMContentLoaded', function () {
    ensureDemoSession();
    applyTheme(currentTheme());
    createGlobalSidebar();
    applyBranding();
    simplifyLeftSidebar();
    createToggle();
  });
})();
