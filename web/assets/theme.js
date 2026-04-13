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
      </nav>
      <div class="sidebar-settings-block">
        <a href="/settings.html" class="global-sidebar__item global-sidebar__item--settings">Настройки</a>
      </div>
    `;

    const right = document.createElement('aside');
    right.id = 'globalRightSidebar';
    right.className = 'global-right-sidebar';
    right.innerHTML = `
      <div id="globalUserCard" class="rounded-[24px] border border-[#e8ecee] bg-white p-5 shadow-[0_10px_20px_rgba(178,190,198,0.10)]"></div>
      <div class="mt-5 rounded-[24px] border border-[#e8ecee] bg-white p-5 shadow-[0_10px_20px_rgba(178,190,198,0.10)]">
        <h3 class="text-[18px] font-semibold text-[#334153]">Быстрые переходы</h3>
        <div class="mt-4 space-y-3 text-[15px] text-[#5f6b79]">
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/dashboard.html">Новости</a>
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/profile.html">Мой профиль</a>
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/profile.html#friends">Друзья</a>
        </div>
      </div>`;

    document.body.classList.add('with-global-sidebar', 'with-global-sidebar-right');
    if (document.querySelector('.dashboard-shell')) {
      document.body.classList.add('use-global-sidebars');
    }
    document.body.appendChild(wrapper);
    document.body.appendChild(right);
  }


  function moveHeaderUserToRightSidebar() {
    const rightUserCard = document.getElementById('globalUserCard');
    if (!rightUserCard) return;

    const user = JSON.parse(localStorage.getItem(DEMO_USER_KEY) || '{}');
    const name = user.full_name || 'Гость';
    const letter = (name[0] || 'U').toUpperCase();

    rightUserCard.innerHTML = `
      <a href="/profile.html" class="flex items-center gap-3 rounded-2xl border border-[#edf0f2] px-3 py-3 hover:bg-[#f8fafb]">
        <div class="grid h-10 w-10 place-items-center rounded-full bg-[#6fa488] text-sm font-semibold text-white">${letter}</div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-[15px] font-semibold text-[#445164]">${name}</div>
          <div class="text-xs text-[#95a1ad]">Профиль пользователя</div>
        </div>
        <i class="fa-solid fa-chevron-right h-4 w-4 text-[#9aa7b3]"></i>
      </a>
    `;

    document.querySelectorAll('.dashboard-main-header a[href="/profile.html"]').forEach((headerProfile) => {
      headerProfile.remove();
    });
  }

  function ensureFloatingProfileButton() {
    const path = window.location.pathname;
    const isAuthPage = path === '/login' || path === '/login.html' || path === '/register' || path === '/register.html';
    if (isAuthPage) return;
    if (document.getElementById('floatingProfileButton')) return;

    const user = JSON.parse(localStorage.getItem(DEMO_USER_KEY) || '{}');
    const name = user.full_name || 'Гость';
    const letter = (name[0] || 'U').toUpperCase();

    const button = document.createElement('a');
    button.id = 'floatingProfileButton';
    button.href = '/profile.html';
    button.className = 'floating-profile-btn';
    button.innerHTML = `
      <span class="floating-profile-btn__avatar">${letter}</span>
      <span class="floating-profile-btn__label">${name}</span>
    `;
    document.body.appendChild(button);
  }

  document.addEventListener('DOMContentLoaded', function () {
    ensureDemoSession();
    applyTheme(currentTheme());
    applyLayout(currentLayout());
    createGlobalSidebar();
    applyBranding();
    simplifyLeftSidebar();
    ensureDashboardSidebarAccess();
    moveHeaderUserToRightSidebar();
    ensureFloatingProfileButton();
  });
})();
