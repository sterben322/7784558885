(function () {
  const THEME_KEY = 'lastop-theme';
  const LAYOUT_KEY = 'lastop-layout-mode';
  const AUTH_USER_KEY = 'user';
  const AUTH_TOKEN_KEY = 'token';
  const API_URL = '/api';
  const AUTH_PAGE_PATHS = ['/login', '/login.html', '/register', '/register.html'];
  const PROFILE_PAGE_PATHS = ['/profile', '/profile.html'];
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

  function isProfilePage(path) {
    return PROFILE_PAGE_PATHS.includes(path);
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

  function isLogoutTrigger(target) {
    return Boolean(target && target.closest('[data-auth-logout], #logoutBtn, #globalLogoutBtn'));
  }

  function setupLogoutDelegation() {
    if (document.documentElement.dataset.logoutDelegationBound === '1') return;
    document.documentElement.dataset.logoutDelegationBound = '1';

    document.addEventListener('click', (event) => {
      if (!isLogoutTrigger(event.target)) return;
      event.preventDefault();
      logout();
    });
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
      if (!nav.querySelector('a[href="/communities.html"]')) {
        nav.appendChild(createNavItem({
          href: '/communities.html',
          iconClass: 'fa-solid fa-users',
          label: 'Сообщества'
        }));
      }

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
    if (isAuthPage(path)) return;
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
        <a href="/communities.html" class="global-sidebar__item">Сообщества</a>
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
          <a class="block rounded-xl border border-[#edf0f2] px-3 py-2 hover:bg-[#f8fafb]" href="/communities.html">Сообщества</a>
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
    if (isAuthPage(path)) return;
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

  const SEARCH_GROUP_ORDER = [
    { key: 'users', title: 'Пользователи' },
    { key: 'companies', title: 'Компании' },
    { key: 'forums', title: 'Форум' },
    { key: 'topics', title: 'Темы' },
    { key: 'chats', title: 'Быстрый чат' },
    { key: 'news', title: 'Публикации' }
  ];

  const SEARCH_TYPE_ICON = {
    user: 'fa-regular fa-user',
    company: 'fa-regular fa-building',
    forum_section: 'fa-regular fa-comments',
    forum_topic: 'fa-regular fa-message',
    chat_user: 'fa-regular fa-envelope',
    news: 'fa-regular fa-newspaper'
  };

  function escapeHtml(value) {
    return String(value || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  function getResultIconClass(item) {
    return SEARCH_TYPE_ICON[item.type] || 'fa-solid fa-hashtag';
  }

  function flattenGroupedResults(data) {
    const flat = [];
    SEARCH_GROUP_ORDER.forEach((group) => {
      const groupItems = Array.isArray(data[group.key]) ? data[group.key] : [];
      groupItems.forEach((item) => flat.push(item));
    });
    return flat;
  }

  function buildResultItem(item, globalIndex) {
    const title = escapeHtml(item.title);
    const subtitle = escapeHtml(item.subtitle || '');
    const category = escapeHtml(item.category || '');
    const iconClass = getResultIconClass(item);
    return `
      <a href="${escapeHtml(item.route)}" class="global-search__item" data-result-index="${globalIndex}">
        <span class="global-search__item-icon"><i class="${iconClass}"></i></span>
        <span class="global-search__item-content">
          <span class="global-search__item-title">${title}</span>
          ${subtitle ? `<span class="global-search__item-subtitle">${subtitle}</span>` : ''}
        </span>
        ${category ? `<span class="global-search__item-badge">${category}</span>` : ''}
      </a>
    `;
  }

  function buildResultGroup(title, items, startIndex) {
    if (!Array.isArray(items) || !items.length) return { html: '', nextIndex: startIndex };
    let cursor = startIndex;
    const cards = items.map((item) => {
      const html = buildResultItem(item, cursor);
      cursor += 1;
      return html;
    }).join('');

    return {
      html: `
        <section class="global-search__group">
          <div class="global-search__group-title">${escapeHtml(title)}</div>
          <div class="global-search__group-items">${cards}</div>
        </section>
      `,
      nextIndex: cursor
    };
  }

  function attachGlobalSearch() {
    const path = window.location.pathname;
    if (isAuthPage(path) || isPublicPage(path) || isProfilePage(path)) return;
    if (document.getElementById('globalSearchBlock')) return;

    const header = document.querySelector('.dashboard-main-header, .app-header') || document.querySelector('body > header .max-w-6xl, body > header .max-w-7xl, body > header');
    if (!header) return;

    const block = document.createElement('div');
    block.id = 'globalSearchBlock';
    block.className = 'global-search';
    block.innerHTML = `
      <div class="global-search__field-wrap">
        <i class="fa-solid fa-magnifying-glass global-search__icon"></i>
        <input id="globalSearchInput" class="global-search__input" type="search" placeholder="Поиск по пользователям, компаниям, форуму, чатам..." autocomplete="off" />
        <button id="globalSearchClear" class="global-search__clear" type="button" aria-label="Очистить поиск">✕</button>
      </div>
      <div id="globalSearchDropdown" class="global-search__dropdown hidden"></div>
    `;

    if (header.classList.contains('dashboard-main-header') || header.classList.contains('app-header')) {
      const rightSide = header.querySelector(':scope > .ml-auto');
      if (rightSide) {
        header.insertBefore(block, rightSide);
      } else {
        header.prepend(block);
      }
    } else {
      header.appendChild(block);
    }

    const input = block.querySelector('#globalSearchInput');
    const clearBtn = block.querySelector('#globalSearchClear');
    const dropdown = block.querySelector('#globalSearchDropdown');
    const token = localStorage.getItem(AUTH_TOKEN_KEY);

    let debounceTimer = null;
    let requestId = 0;
    let activeController = null;
    let activeIndex = -1;
    let flatResults = [];

    function hideDropdown() {
      dropdown.classList.add('hidden');
      dropdown.innerHTML = '';
      activeIndex = -1;
    }

    function highlightActiveResult() {
      dropdown.querySelectorAll('[data-result-index]').forEach((el) => {
        const idx = Number(el.getAttribute('data-result-index'));
        el.classList.toggle('is-active', idx === activeIndex);
      });
    }

    function showDropdown(html) {
      dropdown.innerHTML = html;
      dropdown.classList.remove('hidden');
    }

    function setLoading() {
      showDropdown('<div class="global-search__state">Ищем...</div>');
    }

    function setEmpty(message) {
      showDropdown(`<div class="global-search__state">${message}</div>`);
    }

    function renderResults(data) {
      flatResults = flattenGroupedResults(data);
      activeIndex = -1;
      let indexOffset = 0;
      let html = '';
      SEARCH_GROUP_ORDER.forEach((group) => {
        const groupContent = buildResultGroup(group.title, data[group.key], indexOffset);
        html += groupContent.html;
        indexOffset = groupContent.nextIndex;
      });

      if (!flatResults.length || !html) {
        setEmpty('Ничего не найдено.');
        return;
      }

      const footer = `<a href="/search.html?q=${encodeURIComponent(input.value.trim())}" class="global-search__all-results">Показать все результаты</a>`;
      showDropdown(`${html}${footer}`);
    }

    async function runSearch(rawValue) {
      const query = rawValue.trim();
      if (!query) {
        hideDropdown();
        return;
      }
      if (query.length < 2) {
        setEmpty('Введите минимум 2 символа');
        return;
      }

      setLoading();
      requestId += 1;
      const currentRequest = requestId;

      if (activeController) {
        activeController.abort();
      }
      activeController = new AbortController();

      try {
        const response = await fetch(`${API_URL}/search/global?q=${encodeURIComponent(query)}&limit=5`, {
          headers: { Authorization: `Bearer ${token}` },
          signal: activeController.signal
        });
        const payload = await response.json().catch(() => ({}));
        if (currentRequest !== requestId) return;

        if (!response.ok || !payload.success) {
          throw new Error(payload.error || 'Ошибка поиска');
        }

        renderResults(payload.data || {});
      } catch (error) {
        if (error.name === 'AbortError') return;
        if (currentRequest !== requestId) return;
        flatResults = [];
        setEmpty('Не удалось выполнить поиск. Попробуйте снова.');
      }
    }

    input.addEventListener('input', (event) => {
      const value = event.target.value || '';
      clearBtn.classList.toggle('is-visible', value.trim().length > 0);
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => runSearch(value), 280);
    });

    input.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        hideDropdown();
        input.blur();
      }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        if (dropdown.classList.contains('hidden') || !flatResults.length) return;
        event.preventDefault();
        const maxIndex = flatResults.length - 1;
        if (event.key === 'ArrowDown') {
          activeIndex = activeIndex >= maxIndex ? 0 : activeIndex + 1;
        } else {
          activeIndex = activeIndex <= 0 ? maxIndex : activeIndex - 1;
        }
        highlightActiveResult();
        const activeElement = dropdown.querySelector(`[data-result-index="${activeIndex}"]`);
        if (activeElement) {
          activeElement.scrollIntoView({ block: 'nearest' });
        }
      }
      if (event.key === 'Enter') {
        event.preventDefault();
        if (activeIndex >= 0 && flatResults[activeIndex]) {
          window.location.href = flatResults[activeIndex].route;
          return;
        }
        const query = input.value.trim();
        if (!query) return;
        window.location.href = `/search.html?q=${encodeURIComponent(query)}`;
      }
    });

    clearBtn.addEventListener('click', () => {
      input.value = '';
      clearBtn.classList.remove('is-visible');
      hideDropdown();
      input.focus();
    });

    document.addEventListener('click', (event) => {
      if (!block.contains(event.target)) {
        hideDropdown();
      }
    });

    input.addEventListener('focus', () => {
      if (input.value.trim().length >= 2) {
        runSearch(input.value);
      }
    });

    dropdown.addEventListener('mousemove', () => {
      if (activeIndex !== -1) {
        activeIndex = -1;
        highlightActiveResult();
      }
    });
  }

  function applyInitialVisualState() {
    applyTheme(currentTheme());
    applyLayout(currentLayout());
    createGlobalSidebar();
    setupLogoutDelegation();
  }

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

    attachGlobalSearch();
  });
})();
