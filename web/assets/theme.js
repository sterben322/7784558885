(function () {
  const THEME_KEY = 'lastop-theme';
  const LAYOUT_KEY = 'lastop-layout-mode';
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

  function applyLayout(mode) {
    const isWide = mode === 'wide';
    document.body.classList.toggle('widescreen-mode', isWide);
  }

  function currentLayout() {
    return localStorage.getItem(LAYOUT_KEY) || 'standard';
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

  function createSidebarToggles() {
    if (document.getElementById('sidebarSwitches')) return;

    const sidebar = document.querySelector('.dashboard-shell > aside.dashboard-sidebar');
    if (!sidebar) return;

    const settingsBlock = sidebar.querySelector('.mt-auto');
    const host = document.createElement('div');
    host.id = 'sidebarSwitches';
    host.className = 'sidebar-switches px-5 pb-3';
    host.innerHTML = `
      <button id="layoutToggle" type="button" class="sidebar-toggle" aria-label="Переключить режим экрана">
        <span class="theme-toggle-track"><span class="theme-toggle-thumb"></span></span>
        <span class="theme-toggle-label">Стандарт / широкоформат</span>
      </button>
      <button id="themeToggle" type="button" class="sidebar-toggle" aria-label="Переключить тему">
        <span class="theme-toggle-track"><span class="theme-toggle-thumb"></span></span>
        <span class="theme-toggle-label">Тёмная тема</span>
      </button>
    `;

    if (settingsBlock) {
      sidebar.insertBefore(host, settingsBlock);
    } else {
      sidebar.appendChild(host);
    }

    const themeToggle = document.getElementById('themeToggle');
    const layoutToggle = document.getElementById('layoutToggle');

    themeToggle.addEventListener('click', function () {
      const next = document.body.classList.contains('dark-theme') ? 'light' : 'dark';
      localStorage.setItem(THEME_KEY, next);
      applyTheme(next);
    });

    layoutToggle.addEventListener('click', function () {
      const next = document.body.classList.contains('widescreen-mode') ? 'standard' : 'wide';
      localStorage.setItem(LAYOUT_KEY, next);
      applyLayout(next);
    });
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

  function createNavItem({ href, iconClass, label }) {
    const link = document.createElement('a');
    link.href = href;
    link.className = 'flex w-full items-center gap-3 rounded-2xl px-4 py-3 text-left transition text-[#5d6978] hover:bg-[#f4f7f7]';
    link.innerHTML = `
      <span class="grid h-8 w-8 place-items-center rounded-xl"><i class="${iconClass}"></i></span>
      <span class="text-[15px] font-medium">${label}</span>
    `;
    return link;
  }

  function ensureDashboardSidebarAccess() {
    const navBlocks = document.querySelectorAll('.dashboard-shell > aside.dashboard-sidebar .space-y-1\\.5');
    navBlocks.forEach((nav) => {
      if (!nav.querySelector('a[href="/jobs.html"]')) {
        nav.appendChild(createNavItem({
          href: '/jobs.html',
          iconClass: 'fa-solid fa-briefcase',
          label: 'Резюме и вакансии'
        }));
      }
    });

    const leftSidebar = document.querySelector('.dashboard-shell > aside.dashboard-sidebar');
    if (!leftSidebar) return;

    const settingsLinks = leftSidebar.querySelectorAll('a[href="/settings.html"]');
    if (settingsLinks.length > 0) return;

    const footer = document.createElement('div');
    footer.className = 'mt-auto border-t border-[#e8ecef] px-5 py-5';
    footer.innerHTML = `
      <a href="/settings.html" class="flex items-center gap-3 rounded-2xl px-2 py-2">
        <div class="grid h-11 w-11 place-items-center rounded-full bg-[#d9e2e7] text-sm font-semibold text-[#51606f]">S</div>
        <div class="min-w-0 flex-1"><div class="truncate text-[15px] font-semibold text-[#3a4758]">Настройки</div></div>
        <i class="fa-solid fa-gear h-4 w-4 text-[#a0acb8]"></i>
      </a>
    `;
    leftSidebar.appendChild(footer);
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
    applyLayout(currentLayout());
    createGlobalSidebar();
    applyBranding();
    simplifyLeftSidebar();
    ensureDashboardSidebarAccess();
    createSidebarToggles();
  });
})();
