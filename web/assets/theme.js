(function () {
  const THEME_KEY = 'lastop-theme';
  const LAYOUT_KEY = 'lastop-layout-mode';
  const AUTH_USER_KEY = 'user';
  const AUTH_TOKEN_KEY = 'token';
  const API_URL = '/api';
  const AUTH_PAGE_PATHS = ['/login', '/login.html', '/register', '/register.html'];
  const PUBLIC_PATHS = new Set(['/privacy', '/privacy.html', '/terms', '/terms.html', '/']);

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

  function getStoredUser() {
    try {
      return JSON.parse(localStorage.getItem(AUTH_USER_KEY) || '{}');
    } catch (_) {
      return {};
    }
  }

  function clearAuthState() {
    localStorage.removeItem(AUTH_TOKEN_KEY);
    localStorage.removeItem(AUTH_USER_KEY);
  }

  function isAuthPage(path) {
    return AUTH_PAGE_PATHS.includes(path);
  }

  function isPublicPage(path) {
    if (PUBLIC_PATHS.has(path)) return true;
    return isAuthPage(path);
  }

  async function validateSession() {
    const token = localStorage.getItem(AUTH_TOKEN_KEY);
    if (!token) return false;

    try {
      const response = await fetch(`${API_URL}/me`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!response.ok) {
        clearAuthState();
        return false;
      }
      const payload = await response.json().catch(() => ({}));
      if (payload.user) {
        localStorage.setItem(AUTH_USER_KEY, JSON.stringify(payload.user));
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  async function logout() {
    const token = localStorage.getItem(AUTH_TOKEN_KEY);
    if (token) {
      await fetch(`${API_URL}/auth/logout`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      }).catch(() => null);
    }
    clearAuthState();
    window.location.href = '/login.html';
  }

  function bindLogoutButtons() {
    document.querySelectorAll('[data-auth-logout], #logoutBtn, #globalLogoutBtn').forEach((button) => {
      if (button.dataset.logoutBound === '1') return;
      button.dataset.logoutBound = '1';
      button.addEventListener('click', (event) => {
        event.preventDefault();
        logout();
      });
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
    const logoutButtons = leftSidebar.querySelectorAll('[data-auth-logout], #logoutBtn');
    if (settingsLinks.length > 0 && logoutButtons.length === 0) {
      const settingsContainer = settingsLinks[settingsLinks.length - 1].closest('div');
      if (settingsContainer) {
        const button = document.createElement('button');
        button.setAttribute('data-auth-logout', '');
        button.className = 'mt-3 w-full rounded-2xl border border-[#e8ecef] px-4 py-2 text-left text-[14px] font-medium text-[#5d6978] hover:bg-[#f6f9f8]';
        button.innerHTML = '<i class="fa-solid fa-arrow-right-from-bracket mr-2"></i> Выйти';
        settingsContainer.appendChild(button);
      }
      return;
    }
    if (settingsLinks.length > 0 && logoutButtons.length > 0) return;

    const footer = document.createElement('div');
    footer.className = 'mt-auto border-t border-[#e8ecef] px-5 py-5';
    footer.innerHTML = `
      <a href="/settings.html" class="flex items-center gap-3 rounded-2xl px-2 py-2">
        <div class="grid h-11 w-11 place-items-center rounded-full bg-[#d9e2e7] text-sm font-semibold text-[#51606f]">S</div>
        <div class="min-w-0 flex-1"><div class="truncate text-[15px] font-semibold text-[#3a4758]">Настройки</div></div>
        <i class="fa-solid fa-gear h-4 w-4 text-[#a0acb8]"></i>
      </a>
      <button data-auth-logout class="mt-3 w-full rounded-2xl border border-[#e8ecef] px-4 py-2 text-left text-[14px] font-medium text-[#5d6978] hover:bg-[#f6f9f8]">
        <i class="fa-solid fa-arrow-right-from-bracket mr-2"></i> Выйти
      </button>
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
        <button id="globalLogoutBtn" data-auth-logout class="global-sidebar__item global-sidebar__item--settings mt-2 text-left">Выйти</button>
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

    const user = getStoredUser();
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

    const user = getStoredUser();
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

  function applyInitialVisualState() {
    applyTheme(currentTheme());
    applyLayout(currentLayout());
    createGlobalSidebar();
  }

  // Apply critical visual classes and shell layout as early as possible
  // to avoid flash/switch of the page after async auth check finishes.
  if (document.readyState === 'loading') {
    document.addEventListener('readystatechange', function onReadyStateChange() {
      if (document.readyState !== 'interactive' && document.readyState !== 'complete') return;
      document.removeEventListener('readystatechange', onReadyStateChange);
      applyInitialVisualState();
    });
  } else {
    applyInitialVisualState();
  }

  document.addEventListener('DOMContentLoaded', async function () {
    const path = window.location.pathname;
    const requiresAuth = !isPublicPage(path);

    applyInitialVisualState();
    applyBranding();
    simplifyLeftSidebar();
    ensureDashboardSidebarAccess();
    moveHeaderUserToRightSidebar();
    ensureFloatingProfileButton();
    bindLogoutButtons();

    const validSession = await validateSession();
    if (isAuthPage(path) && validSession) {
      window.location.href = '/dashboard.html';
      return;
    }
    if (requiresAuth && !validSession) {
      clearAuthState();
      window.location.href = '/login.html';
      return;
    }
  });
})();
