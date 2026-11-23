/* NovelArchiver — app shell (library, novel page, settings).
   Runs unchanged on the desktop (Wails: window.go.main.App, window.runtime)
   and on Android (android-shim.js provides the same API).
   Views are hash routes: #/  #/novel/<slug>  #/settings */
(function () {
  'use strict';

  // ---------------------------------------------------------------- utils
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const esc = (v) => String(v == null ? '' : v).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const num = (n) => (Number(n) || 0).toLocaleString('en-US');
  const plural = (n, one, many) => `${num(n)} ${n === 1 ? one : (many || one + 's')}`;
  const store = {
    get(k, d) { try { const v = localStorage.getItem(k); return v == null ? d : JSON.parse(v); } catch (e) { return d; } },
    set(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) {} },
    raw(k, v) { try { localStorage.setItem(k, v); } catch (e) {} },
  };
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  function mb(bytes) {
    const b = Number(bytes) || 0;
    if (b >= 1 << 30) return (b / (1 << 30)).toFixed(1) + ' GB';
    if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + ' MB';
    if (b > 0) return Math.max(1, Math.round(b / 1024)) + ' KB';
    return '0 MB';
  }
  // Go durations ("1h2m3s", "45s", "2m0s") -> "about 1 h 2 min"
  function eta(s) {
    if (!s || !/\d/.test(s)) return '';
    let sec = 0;
    const re = /([\d.]+)(h|ms|m|s)/g;
    let m;
    while ((m = re.exec(s))) sec += parseFloat(m[1]) * ({ h: 3600, m: 60, s: 1, ms: 0.001 }[m[2]]);
    if (sec < 1) return '';
    if (sec < 60) return 'under a minute';
    const h = Math.floor(sec / 3600), min = Math.round((sec % 3600) / 60);
    if (h) return `about ${h} h${min ? ' ' + min + ' min' : ''}`;
    return `about ${min} min`;
  }
  function ago(t) {
    if (!t) return '';
    const d = new Date(t);
    if (isNaN(d) || d.getFullYear() < 2000) return '';
    const s = (Date.now() - d.getTime()) / 1000;
    if (s < 60) return 'just now';
    if (s < 3600) return plural(Math.round(s / 60), 'minute') + ' ago';
    if (s < 86400) return plural(Math.round(s / 3600), 'hour') + ' ago';
    if (s < 86400 * 30) return plural(Math.round(s / 86400), 'day') + ' ago';
    return d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
  }
  function took(sec) {
    sec = Number(sec) || 0;
    if (sec < 60) return 'took ' + Math.max(1, sec) + ' s';
    if (sec < 3600) return 'took ' + Math.round(sec / 60) + ' min';
    return 'took ' + (sec / 3600).toFixed(1) + ' h';
  }
  function hostOf(u) { try { return new URL(u).host.replace(/^www\./, ''); } catch (e) { return ''; } }
  function listNums(ns, max = 4) {
    if (!ns || !ns.length) return '';
    if (ns.length === 1) return String(num(ns[0]));
    const sorted = ns.slice().sort((a, b) => a - b);
    let contiguous = true;
    for (let i = 1; i < sorted.length; i++) if (sorted[i] !== sorted[i - 1] + 1) contiguous = false;
    if (contiguous) return `${num(sorted[0])} to ${num(sorted[sorted.length - 1])}`;
    const shown = sorted.slice(0, max).map(num).join(', ');
    return sorted.length > max ? `${shown} and ${sorted.length - max} more` : shown;
  }

  // ---------------------------------------------------------------- icons
  const P = {
    search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
    link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
    arrow: '<path d="M5 12h14M12 5l7 7-7 7"/>',
    x: '<path d="M18 6 6 18M6 6l12 12"/>',
    back: '<path d="m15 18-6-6 6-6"/>',
    down: '<path d="m6 9 6 6 6-6"/>',
    book: '<path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>',
    refresh: '<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>',
    download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5"/><path d="M12 15V3"/>',
    more: '<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>',
    external: '<path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    alert: '<circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>',
    lock: '<rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
    pencil: '<path d="M21.17 6.81a1 1 0 0 0-3.98-3.98L3.84 16.17a2 2 0 0 0-.5.83l-1.32 4.35a.5.5 0 0 0 .62.62l4.35-1.32a2 2 0 0 0 .83-.5z"/>',
    image: '<rect width="18" height="18" x="3" y="3" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/>',
    shield: '<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/><path d="m9 12 2 2 4-4"/>',
    moon: '<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>',
    sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2m-7.07-17.07 1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/>',
    sepia: '<path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>',
    monitor: '<rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8M12 17v4"/>',
    pause: '<rect x="6" y="4" width="4" height="16" rx="1" fill="currentColor" stroke="none"/><rect x="14" y="4" width="4" height="16" rx="1" fill="currentColor" stroke="none"/>',
    clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
    wifi: '<path d="M12 20h.01M2 8.82a15 15 0 0 1 20 0M5 12.86a10 10 0 0 1 14 0M8.5 16.43a5 5 0 0 1 7 0"/>',
  };
  const icon = (name, size = 16, sw = 2) => `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${sw}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${P[name]}</svg>`;
  const spinner = (size = 16, tone = 'var(--accent)') => `<svg class="spinner" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity=".22" stroke-width="3"/><path d="M21 12a9 9 0 0 0-9-9" stroke="${tone}" stroke-width="3" stroke-linecap="round"/></svg>`;
  const LOGO = '<svg width="40" height="40" viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="15" fill="#1D1B18"/><path d="M16 15a3 3 0 0 1 3-3h3v40h-3a3 3 0 0 1-3-3z" fill="#CDBFA6"/><path d="M22 12h20a4 4 0 0 1 4 4v32a4 4 0 0 1-4 4H22z" fill="#F7F3EC"/><path d="M32 12h8v20l-4-3-4 3z" fill="#A8431F"/></svg>';

  // ---------------------------------------------------------------- covers
  // The site's poster always wins. Without one (or if it fails to load) we
  // draw a cloth-bound cover whose colour comes from the title, so the same
  // book looks the same everywhere.
  const TONES = { oxblood: '#6E2A2A', forest: '#2F4A3A', navy: '#23344F', ochre: '#7A5518', plum: '#4E2E4A', slate: '#3D4650', teal: '#1F4E52', umber: '#5A3F2B' };
  const TONE_LIST = Object.values(TONES);
  function toneOf(title) {
    let h = 0;
    const t = String(title || '');
    for (let i = 0; i < t.length; i++) h = (h * 31 + t.charCodeAt(i)) >>> 0;
    return TONE_LIST[h % TONE_LIST.length];
  }
  function fallbackCover(title, author, compact) {
    const initial = (String(title || 'N').replace(/^(the|a|an)\s+/i, '').trim()[0] || 'N').toUpperCase();
    const inner = compact
      ? `<div class="cover__mono">${esc(initial)}</div>`
      : `<div class="cover__frame"></div><div class="cover__body"><div class="cover__author">${esc(author || '')}</div><div class="cover__title">${esc(title || 'Untitled')}</div><svg class="cover__mark" viewBox="0 0 12 16" aria-hidden="true"><path d="M1 0h10v15l-5-4-5 4z" fill="#F3E9D2"/></svg></div>`;
    return `<div class="cover cover--fb${compact ? ' cover--sm' : ''}" style="background:${toneOf(title)}"><div class="cover__spine"></div><div class="cover__hinge"></div>${inner}</div>`;
  }
  function coverHTML(src, title, author, compact) {
    if (!src) return fallbackCover(title, author, compact);
    return `<div class="cover${compact ? ' cover--sm' : ''}" data-fb-title="${esc(title)}" data-fb-author="${esc(author || '')}" data-fb-compact="${compact ? 1 : ''}"><img src="${esc(src)}" alt="" loading="lazy" decoding="async"></div>`;
  }
  // A poster that fails to load becomes a fallback cover.
  document.addEventListener('error', (e) => {
    const img = e.target;
    if (!(img instanceof HTMLImageElement) || !img.parentElement || !img.parentElement.classList.contains('cover')) return;
    const box = img.parentElement;
    box.outerHTML = fallbackCover(box.dataset.fbTitle, box.dataset.fbAuthor, !!box.dataset.fbCompact);
  }, true);

  // ---------------------------------------------------------------- backend
  // Wait for the bridge. Never resolve a promise with the App object itself:
  // on Android it is a Proxy, and a value with a "then" looks like a promise.
  async function bridgeReady() {
    for (let i = 0; i < 100 && !(window.go && window.go.main && window.go.main.App); i++) await sleep(30);
    return !!(window.go && window.go.main && window.go.main.App);
  }
  async function api(name, ...args) {
    const ok = await bridgeReady();
    const app = ok ? window.go.main.App : null;
    if (!app || typeof app[name] !== 'function') throw new Error('This action isn\'t available in this version of the app.');
    try {
      return await app[name](...args);
    } catch (e) {
      throw new Error(friendlyErr(e));
    }
  }
  function friendlyErr(e) {
    let m = (e && (e.message || e)) || 'Something went wrong';
    m = String(m).replace(/^Error:\s*/, '');
    return m.charAt(0).toUpperCase() + m.slice(1);
  }
  function on(event, fn) {
    const hook = () => {
      if (window.runtime && window.runtime.EventsOn) { window.runtime.EventsOn(event, (d) => { if (typeof d === 'string') { try { d = JSON.parse(d); } catch (e) {} } fn(d || {}); }); return true; }
      return false;
    };
    if (!hook()) { let n = 0; const t = setInterval(() => { if (hook() || ++n > 1200) clearInterval(t); }, 50); }
  }

  // ---------------------------------------------------------------- state
  const DEFAULT_SETTINGS = { theme: 'system', chime: true, notify: true, speed: 'balanced', keepImages: true, revise: 'all', updateOnOpen: false, wifiOnly: true };
  const state = {
    platform: 'desktop',
    settings: Object.assign({}, DEFAULT_SETTINGS, store.get('na-settings', {})),
    novels: [],
    loaded: false,
    stats: null,
    activity: { current: null, queue: [], waiting: '' },
    results: store.get('na-results', []),
    log: [],
    showLog: false,
    showQueue: false,
    lastProgressAt: 0,
    lastStatusAt: 0,
    filter: store.get('na-filter', 'all'),
    status: 'any',
    sort: store.get('na-sort', 'read'),
    query: '',
    intake: { phase: 'idle', url: '', data: null, err: '', scope: 'all', from: '', to: '', speed: '', images: null, descOpen: false, token: 0 },
    novel: null,
    novelSlug: '',
    tocFilter: 'all',
    tocQuery: '',
    synopsisOpen: false,
  };
  const speedWorkers = { gentle: 2, balanced: 4, fast: 8 };

  // ---------------------------------------------------------------- theme
  const THEMES = ['light', 'sepia', 'dark', 'system'];
  const THEME_LABEL = { light: 'Light', sepia: 'Sepia', dark: 'Dark', system: 'Match system' };
  const THEME_ICON = { light: 'sun', sepia: 'sepia', dark: 'moon', system: 'monitor' };
  function applyTheme(t) {
    if (!THEMES.includes(t)) t = 'system';
    document.documentElement.dataset.theme = t;
    store.raw('na-theme', t);
    const next = THEMES[(THEMES.indexOf(t) + 1) % THEMES.length];
    const b = $('#themeBtn');
    if (b) { b.innerHTML = icon(THEME_ICON[t], 18); b.setAttribute('aria-label', `Theme: ${THEME_LABEL[t]}. Switch to ${THEME_LABEL[next]}`); b.title = `Theme: ${THEME_LABEL[t]}`; }
  }
  function cycleTheme() {
    const t = THEMES[(THEMES.indexOf(state.settings.theme) + 1) % THEMES.length];
    saveSettings({ theme: t });
    toast(`Theme: ${THEME_LABEL[t]}`, { timeout: 1800 });
  }
  async function saveSettings(patch) {
    state.settings = Object.assign({}, state.settings, patch);
    store.set('na-settings', state.settings);
    if ('theme' in patch) applyTheme(state.settings.theme);
    try { const s = await api('SaveSettings', state.settings); if (s) { state.settings = Object.assign({}, state.settings, s); store.set('na-settings', state.settings); } } catch (e) { /* older core: kept locally */ }
  }

  // ---------------------------------------------------------------- toasts
  function toast(msg, opts = {}) {
    const box = $('#toasts');
    const el = document.createElement('div');
    el.className = 'toast';
    el.setAttribute('role', opts.tone === 'bad' ? 'alert' : 'status');
    const dot = { ok: '#8FC79B', warn: '#E8B04A', bad: '#F07A6A' }[opts.tone];
    el.innerHTML = `${dot ? `<span class="dot" style="background:${dot}"></span>` : ''}<span class="msg">${esc(msg)}</span>${opts.action ? `<button type="button" class="act${opts.plain ? ' act--plain' : ''}">${esc(opts.action.label)}</button>` : ''}<button type="button" class="x" aria-label="Dismiss">${icon('x', 14)}</button>`;
    const close = () => { el.style.transition = 'opacity .15s'; el.style.opacity = '0'; setTimeout(() => el.remove(), 160); };
    if (opts.action) $('.act', el).addEventListener('click', () => { close(); opts.action.fn(); });
    $('.x', el).addEventListener('click', close);
    box.appendChild(el);
    while (box.children.length > 3) box.firstElementChild.remove();
    const t = opts.timeout === 0 ? 0 : (opts.timeout || (opts.action ? 8000 : 4500));
    if (t) setTimeout(close, t);
    return close;
  }

  // ---------------------------------------------------------------- notify & chime
  function notify(title, body) {
    if (!state.settings.notify) return;
    if (!document.hidden && document.hasFocus()) return;
    api('Notify', title, body).catch(() => {});
  }
  function chime() {
    if (!state.settings.chime) return;
    try {
      const C = window.AudioContext || window.webkitAudioContext;
      if (!C) return;
      const ctx = new C();
      [659.25, 987.77].forEach((f, i) => {
        const o = ctx.createOscillator(), g = ctx.createGain();
        o.type = 'sine'; o.frequency.value = f;
        const t = ctx.currentTime + i * 0.16;
        g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(0.12, t + 0.02); g.gain.exponentialRampToValueAtTime(0.0001, t + 0.7);
        o.connect(g); g.connect(ctx.destination); o.start(t); o.stop(t + 0.75);
      });
      setTimeout(() => ctx.close && ctx.close(), 1400);
    } catch (e) {}
  }

  // ---------------------------------------------------------------- overlays
  let overlayClose = null;
  function closeOverlay() { if (overlayClose) { const f = overlayClose; overlayClose = null; f(); } }
  function openOverlay(html, { onClose, top, label, instant } = {}) {
    closeOverlay();
    const prev = document.activeElement;
    const root = $('#overlay');
    root.innerHTML = `<div class="scrim${top ? ' scrim--top' : ''}${instant ? ' scrim--instant' : ''}">${html}</div>`;
    const scrim = root.firstElementChild;
    const dlg = scrim.firstElementChild;
    if (label && dlg) dlg.setAttribute('aria-label', label);
    scrim.addEventListener('mousedown', (e) => { if (e.target === scrim) closeOverlay(); });
    const trap = (e) => {
      if (e.key !== 'Tab') return;
      const f = $$('button:not([disabled]), [href], input:not([disabled]), select, [tabindex]:not([tabindex="-1"])', dlg).filter((x) => x.offsetParent !== null);
      if (!f.length) return;
      if (e.shiftKey && document.activeElement === f[0]) { e.preventDefault(); f[f.length - 1].focus(); }
      else if (!e.shiftKey && document.activeElement === f[f.length - 1]) { e.preventDefault(); f[0].focus(); }
    };
    dlg.addEventListener('keydown', trap);
    overlayClose = () => { root.innerHTML = ''; if (onClose) onClose(); if (prev && prev.focus) prev.focus(); };
    const first = $('[autofocus], input, .btn--primary, .btn--danger, button', dlg);
    if (first) setTimeout(() => first.focus(), 20);
    return dlg;
  }
  function confirmDialog({ title, body, ok = 'OK', cancel = 'Cancel', danger = false, extra = '', cover = '' }) {
    return new Promise((resolve) => {
      let done = false;
      const dlg = openOverlay(`<div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="dlgT" aria-describedby="dlgB">
        <div style="display:flex;gap:16px;align-items:flex-start">${cover ? `<div style="width:48px;flex:none">${cover}</div>` : ''}
          <div style="display:flex;flex-direction:column;gap:6px;min-width:0"><h2 id="dlgT">${esc(title)}</h2><p id="dlgB">${body}</p></div></div>
        ${extra}
        <div class="dialog__foot"><button type="button" class="btn" data-r="0">${esc(cancel)}</button><button type="button" class="btn ${danger ? 'btn--danger' : 'btn--primary'}" data-r="1">${esc(ok)}</button></div></div>`,
      { onClose: () => { if (!done) resolve(false); } });
      $$('[data-r]', dlg).forEach((b) => b.addEventListener('click', () => {
        done = true;
        const extraVals = {};
        $$('input', dlg).forEach((i) => { extraVals[i.name] = i.type === 'checkbox' ? i.checked : i.value; });
        closeOverlay();
        resolve(b.dataset.r === '1' ? (extra ? extraVals : true) : false);
      }));
    });
  }
  // Popup menu anchored to a button. items: [{label, sub, fn, danger}] or 'sep'
  function openMenu(anchor, items) {
    closeMenu();
    const m = document.createElement('div');
    m.className = 'menu';
    m.setAttribute('role', 'menu');
    m.innerHTML = items.map((it, i) => it === 'sep' ? '<hr>' : `<button type="button" role="menuitem" data-i="${i}" class="${it.danger ? 'danger' : ''}">${esc(it.label)}${it.sub ? `<small>${esc(it.sub)}</small>` : ''}</button>`).join('');
    document.body.appendChild(m);
    const r = anchor.getBoundingClientRect();
    const w = m.offsetWidth, h = m.offsetHeight;
    let left = Math.min(r.left, window.innerWidth - w - 12);
    let top = r.bottom + 6;
    if (top + h > window.innerHeight - 12) top = Math.max(12, r.top - h - 6);
    m.style.left = Math.max(12, left) + 'px';
    m.style.top = top + 'px';
    anchor.setAttribute('aria-expanded', 'true');
    $$('button', m).forEach((b) => b.addEventListener('click', () => { const it = items[+b.dataset.i]; closeMenu(); it.fn(); }));
    const first = $('button', m); if (first) first.focus();
    m.addEventListener('keydown', (e) => {
      const bs = $$('button', m), i = bs.indexOf(document.activeElement);
      if (e.key === 'ArrowDown') { e.preventDefault(); bs[(i + 1) % bs.length].focus(); }
      if (e.key === 'ArrowUp') { e.preventDefault(); bs[(i - 1 + bs.length) % bs.length].focus(); }
      if (e.key === 'Escape') { e.preventDefault(); closeMenu(); anchor.focus(); }
    });
    menuState = { el: m, anchor };
  }
  let menuState = null;
  function closeMenu() { if (menuState) { menuState.el.remove(); menuState.anchor.setAttribute('aria-expanded', 'false'); menuState = null; } }
  document.addEventListener('mousedown', (e) => { if (menuState && !menuState.el.contains(e.target) && !menuState.anchor.contains(e.target)) closeMenu(); });
  window.addEventListener('resize', closeMenu);
  window.addEventListener('scroll', closeMenu, true);

  // ---------------------------------------------------------------- library data
  async function loadLibrary() {
    try { state.novels = (await api('GetNovels')) || []; } catch (e) { state.novels = []; toast(e.message, { tone: 'bad' }); }
    state.loaded = true;
    if (route().name === 'library') { renderShelf(); }
    if (route().name === 'novel' && state.novelSlug) loadNovel(state.novelSlug, true);
  }
  async function loadActivity() {
    try {
      const a = await api('GetActivity');
      if (a) {
        state.activity = { current: a.current || null, queue: a.queue || [], waiting: a.waiting || '' };
        if (a.settings) { state.settings = Object.assign({}, state.settings, a.settings); store.set('na-settings', state.settings); applyTheme(state.settings.theme); }
      }
    } catch (e) { /* older core */ }
    renderJobsEverywhere();
  }
  function novelBySlug(slug) { return state.novels.find((n) => n.slug === slug); }
  function readInfo(n) {
    const total = n.totalChapters || 0;
    const last = n.lastRead || 0;
    const finished = total > 0 && last >= total;
    return {
      started: last > 0, finished, reading: last > 0 && !finished, last, total,
      pct: total ? Math.min(100, Math.round((last / total) * 100)) : 0,
    };
  }
  function bookStatus(n) {
    const s = String(n.status || '').toLowerCase();
    if (s.includes('complet') || s.includes('finish') || s.includes('end')) return 'completed';
    if (s.includes('hiatus') || s.includes('pause')) return 'hiatus';
    if (s.includes('ongoing') || s.includes('active') || s.includes('serial')) return 'ongoing';
    return s ? 'other' : '';
  }
  const STATUS_LABEL = { ongoing: 'Ongoing', completed: 'Completed', hiatus: 'On hiatus', other: '' };
  function isFresh(n) {
    if (!n.lastNew || !n.lastNewAt) return false;
    const at = new Date(n.lastNewAt).getTime();
    return Date.now() - at < 4 * 86400 * 1000 && (n.lastRead || 0) < n.totalChapters;
  }

  // ---------------------------------------------------------------- router
  function route() {
    const h = location.hash.replace(/^#\/?/, '');
    const parts = h.split('/');
    if (parts[0] === 'novel' && parts[1]) return { name: 'novel', slug: decodeURIComponent(parts[1]) };
    if (parts[0] === 'settings') return { name: 'settings' };
    return { name: 'library' };
  }
  let lastRoute = '';
  function render() {
    closeMenu();
    const r = route();
    const key = r.name + (r.slug || '');
    const main = $('#main');
    const sb = $('#settingsBtn'); if (r.name === 'settings') sb.setAttribute('aria-current', 'page'); else sb.removeAttribute('aria-current');
    if (r.name === 'settings') renderSettings(main);
    else if (r.name === 'novel') renderNovelPage(main, r.slug);
    else renderLibraryPage(main);
    if (key !== lastRoute) { window.scrollTo(0, 0); lastRoute = key; }
  }
  window.addEventListener('hashchange', render);

  // ================================================================ LIBRARY
  function renderLibraryPage(main) {
    main.innerHTML = `<div class="wrap stack gap-40">
      <section class="intake" id="intake" aria-labelledby="addH"></section>
      <section class="jobs" id="jobs" aria-label="Downloads" hidden></section>
      <section class="stack" style="gap:20px" id="shelf" aria-labelledby="libH"></section>
    </div>`;
    renderIntake();
    renderJobs($('#jobs'));
    renderShelf();
  }

  // ---------------------------------------------------------------- intake
  function intakeCount(d, it) {
    const ch = d.chapters || [];
    const free = ch.filter((c) => !c.locked);
    if (it.scope === 'from') return free.filter((c) => c.number >= (d.inputChapter || 1)).length;
    if (it.scope === 'range') {
      const f = parseInt(it.from, 10) || 1, t = parseInt(it.to, 10) || Infinity;
      return free.filter((c) => c.number >= f && c.number <= t).length;
    }
    return free.length;
  }
  function renderIntake() {
    const box = $('#intake');
    if (!box) return;
    const it = state.intake;
    const d = it.data;
    const looking = it.phase === 'looking';
    const hasText = it.url.trim().length > 0;
    const canPaste = !!(navigator.clipboard && navigator.clipboard.readText);
    let btn;
    if (looking) btn = `<button type="button" class="btn btn--ghost" data-act="intake-clear" aria-label="Clear link">${icon('x', 16)}</button>`;
    else if (it.phase === 'error') btn = `<button type="submit" class="btn">Try again</button>`;
    else if (hasText || !canPaste) btn = `<button type="submit" class="btn btn--primary">Find<span class="wide"> novel</span> ${icon('arrow', 16)}</button>`;
    else btn = `<button type="button" class="btn" data-act="paste">Paste</button>`;

    let panel = '';
    if (looking) {
      panel = `<div class="card skeleton" aria-live="polite">
        <div class="sk" style="width:88px;height:132px;flex:none;border-radius:6px"></div>
        <div style="flex:1;display:flex;flex-direction:column;gap:10px;padding-top:4px">
          <div style="display:flex;align-items:center;gap:8px;font-size:14px;color:var(--muted)">${spinner(16)} Reading ${esc(hostOf(normaliseUrl(it.url)) || 'the site')}…</div>
          <div class="sk" style="height:20px;width:78%"></div><div class="sk" style="height:12px;width:40%"></div>
          <div class="sk sk--soft" style="height:12px;width:92%;margin-top:8px"></div><div class="sk sk--soft" style="height:12px;width:70%"></div>
        </div></div>`;
    } else if (it.phase === 'error') {
      panel = `<div class="err-box" id="intakeErr" role="alert"><b>${esc(it.err.title)}</b><span>${esc(it.err.body)}</span></div>`;
    } else if (it.phase === 'found' && d) {
      panel = foundCard(d, it);
    }
    box.innerHTML = `
      <h1 class="page-h" id="addH">Add a novel</h1>
      <form class="linkbox${it.phase === 'error' ? ' is-error' : ''}" id="intakeForm" novalidate>
        <label>${icon(it.phase === 'error' ? 'alert' : 'link', 18)}
          <input type="url" id="novelUrl" placeholder="Paste a novel page or any chapter link" aria-label="Novel or chapter link" autocomplete="off" spellcheck="false" value="${esc(it.url)}" ${looking ? 'readonly' : ''} ${it.phase === 'error' ? 'aria-invalid="true" aria-describedby="intakeErr"' : ''}>
        </label>${btn}
      </form>
      ${panel || `<p class="hint">Works with NovelBin, NovelFull, Royal Road, WuxiaWorld, ScribbleHub and 5 more. Other sites open in reader mode.</p>`}`;
    const input = $('#novelUrl');
    input.addEventListener('input', () => {
      const was = state.intake.url.trim().length > 0;
      state.intake.url = input.value;
      if (state.intake.phase === 'error') { state.intake.phase = 'idle'; renderIntake(); focusUrl(); return; }
      if (was !== (input.value.trim().length > 0) && state.intake.phase === 'idle') { renderIntake(); focusUrl(); }
    });
    input.addEventListener('paste', () => setTimeout(() => {
      state.intake.url = input.value;
      if (/^\s*(https?:\/\/|www\.)/i.test(input.value) || /\.\w{2,}\//.test(input.value)) lookup(input.value);
    }, 30));
    $('#intakeForm').addEventListener('submit', (e) => { e.preventDefault(); if (state.intake.url.trim()) lookup(state.intake.url); });
    bindFound(box);
  }
  function focusUrl() { const i = $('#novelUrl'); if (i) { i.focus(); const v = i.value; i.setSelectionRange(v.length, v.length); } }
  function normaliseUrl(u) {
    u = String(u || '').trim();
    if (u && !/^https?:\/\//i.test(u)) u = 'https://' + u;
    return u;
  }
  async function lookup(raw) {
    const url = normaliseUrl(raw);
    if (!url) return;
    const it = state.intake;
    const token = ++it.token;
    Object.assign(it, { phase: 'looking', url: raw.trim(), data: null, err: '', scope: 'all', from: '', to: '', descOpen: false, speed: state.settings.speed, images: state.settings.keepImages });
    renderIntake();
    try {
      const d = await api('DiscoverNovel', url);
      if (token !== it.token) return;
      if (!d || !d.title) throw new Error('No novel found at that link.');
      it.data = d;
      it.lookupUrl = url;
      it.phase = 'found';
      it.scope = d.inputKind === 'chapter' && (d.inputChapter || 0) > 1 && !d.existingSlug ? 'from' : 'all';
      renderIntake();
      const first = $('.found .btn--primary');
      if (first && window.innerWidth > 640) first.focus();
    } catch (e) {
      if (token !== it.token) return;
      it.phase = 'error';
      it.err = explainLookupError(e.message, url);
      renderIntake();
    }
  }
  function explainLookupError(msg, url) {
    const m = String(msg || '');
    const host = hostOf(url) || 'that site';
    if (/no such host|dial tcp|connection refused|network|timeout|offline|unreachable/i.test(m)) return { title: `Can’t reach ${host}.`, body: 'Check your internet connection, then try again.' };
    if (/404|doesn't exist/i.test(m)) return { title: 'That page doesn’t exist.', body: 'Check the link, or paste the novel’s main page or any of its chapters.' };
    if (/bot check|refused|blocked/i.test(m)) return { title: `${host} refused the request.`, body: 'Open the site in your browser to check it works, then try again.' };
    if (/no chapters|not a novel|list|couldn't find|could not find|no novel/i.test(m)) return { title: 'That doesn’t look like a novel page.', body: 'Open the novel you want and paste its page, or any of its chapters.' };
    if (/browser fetch failed|failed to fetch/i.test(m)) return { title: `${host} didn’t answer.`, body: 'It may be down or blocking the app right now. Try again in a minute, or paste the link from a different mirror of the site.' };
    return { title: 'Couldn’t read that link.', body: explainJobError(m, url) };
  }
  function cancelIntake() {
    state.intake.token++;
    Object.assign(state.intake, { phase: 'idle', url: '', data: null, err: '' });
    renderIntake();
  }
  function foundCard(d, it) {
    const total = d.totalChapters || 0;
    const crawl = d.tocMode === 'crawl';
    const site = d.siteName || hostOf(d.url);
    const warnings = d.warnings || [];
    const verify = warnings.some((w) => /verify you are human|human check|real browser/i.test(w));
    const notes = warnings.map((w) => {
      if (/paid|locked/i.test(w)) return `<div class="note note--amber">${icon('lock', 16)}<span>${esc(w)}</span></div>`;
      if (/verify you are human|human/i.test(w)) return `<div class="note note--amber">${icon('shield', 16)}<span>${esc(w.replace(/the site browser window pops up so you can tick it once; it's remembered after that/i, 'a site window opens so you can tick it once. It’s remembered after that'))}</span></div>`;
      return `<div class="note">${icon('info', 16)}<span>${esc(w)}</span></div>`;
    });
    const poster = d.coverData || d.cover;
    let sub = `${esc(d.author || 'Unknown author')}`;
    sub += crawl ? ' · chapter count unknown' : ` · ${plural(total, 'chapter')}`;
    const head = `<div class="found__head">
        <div class="found__cover">${coverHTML(poster, d.title, d.author, false)}</div>
        <div class="found__meta">
          <span style="font-size:12px;color:var(--muted)">${esc(site)}${d.status ? ` · ${esc(d.status)}` : ''}</span>
          <span class="found__title" id="foundT">${esc(d.title)}</span>
          <span style="font-size:14px;color:var(--muted)">${sub}</span>
          ${d.description ? `<p class="found__desc${it.descOpen ? ' is-open' : ''}">${esc(d.description)}</p>` : ''}
        </div></div>`;

    // Already in the library: open it or update it.
    if (d.existingSlug) {
      return `<div class="sheet-scrim" data-act="intake-cancel"></div><div class="card found" role="dialog" aria-labelledby="foundT">${head}
        <div class="found__notes"><div class="note note--green">${icon('check', 16, 2.5)}<span><strong>You have this one.</strong> ${num(d.downloaded)} of ${plural(total, 'chapter')} are saved.</span></div>${notes.join('')}</div>
        <div class="found__foot"><button type="button" class="btn btn--ghost" data-act="intake-cancel">Cancel</button>
          <a class="btn" href="#/novel/${encodeURIComponent(d.existingSlug)}" data-act="intake-open">Open novel</a>
          <button type="button" class="btn btn--primary" data-act="intake-update">${icon('refresh', 16)} Update it</button></div></div>`;
    }

    const chapterLink = d.inputKind === 'chapter' && (d.inputChapter || 0) > 1;
    const count = intakeCount(d, it);
    let scope = '';
    if (!crawl) {
      if (chapterLink) {
        const pasted = (d.chapters || []).find((c) => c.number === d.inputChapter);
        scope = `<fieldset style="margin:0;padding:0;border:0;display:flex;flex-direction:column;gap:8px">
          <legend style="padding:0;margin-bottom:8px;font-size:14px;font-weight:500">You pasted chapter ${num(d.inputChapter)}${pasted && pasted.title ? ` · ${esc(pasted.title)}` : ''}. Download…</legend>
          <label class="choice"><input type="radio" name="scope" value="from" class="check" ${it.scope === 'from' ? 'checked' : ''}><span><b>From chapter ${num(d.inputChapter)} onward</b><small>${plural(intakeCount(d, { scope: 'from' }), 'chapter')} · pick up where you are</small></span></label>
          <label class="choice"><input type="radio" name="scope" value="all" class="check" ${it.scope === 'all' ? 'checked' : ''}><span><b>The whole novel</b><small>${plural(intakeCount(d, { scope: 'all' }), 'chapter')} from chapter 1</small></span></label>
          <label class="choice"><input type="radio" name="scope" value="range" class="check" ${it.scope === 'range' ? 'checked' : ''}><span><b>A range</b><small>Pick the first and last chapter</small></span></label>
        </fieldset>`;
      } else {
        scope = `<div class="opt-row"><span class="lbl">Chapters</span>
          <div class="seg" role="radiogroup" aria-label="Which chapters">
            <button type="button" role="radio" aria-checked="${it.scope !== 'range'}" data-scope="all">All ${num(intakeCount(d, { scope: 'all' }))}</button>
            <button type="button" role="radio" aria-checked="${it.scope === 'range'}" data-scope="range">A range</button>
          </div></div>`;
      }
      if (it.scope === 'range') {
        scope += `<div class="range">
          <label>From<input class="num" type="number" min="1" inputmode="numeric" id="rFrom" value="${esc(it.from || 1)}"></label>
          <label>To<input class="num" type="number" min="1" inputmode="numeric" id="rTo" value="${esc(it.to || total)}" placeholder="${total}"></label>
          <span class="tally" id="rTally">${plural(count, 'chapter')} · get the rest later with Update</span></div>`;
      }
    }
    const speed = it.speed || state.settings.speed;
    const opts = `<div class="found__opts">${scope}
      <div class="opt-row"><span class="lbl">Speed</span><div>
        <div class="seg" role="radiogroup" aria-label="Download speed">
          ${['gentle', 'balanced', 'fast'].map((s) => `<button type="button" role="radio" aria-checked="${speed === s}" data-speed="${s}">${s[0].toUpperCase() + s.slice(1)}</button>`).join('')}
        </div><div class="sub">${speedWorkers[speed]} at a time. A site never gets more than 3 requests at once.</div></div></div>
      <label class="opt-row" style="align-items:center"><span class="lbl" style="padding-top:0">Images</span>
        <input type="checkbox" class="check" id="optImages" ${it.images !== false ? 'checked' : ''}>
        <span style="font-size:14px">Keep chapter images <span class="muted">(text and [System] boxes are always kept)</span></span></label>
    </div>`;
    const label = crawl ? 'Start downloading' : count > 0 ? `Download ${plural(count, 'chapter')}` : 'Nothing to download';
    return `<div class="sheet-scrim" data-act="intake-cancel"></div><div class="card found" role="dialog" aria-labelledby="foundT">${head}
      ${notes.length ? `<div class="found__notes">${notes.join('')}</div>` : ''}
      ${opts}
      <div class="found__foot">
        <button type="button" class="btn btn--ghost" data-act="intake-cancel">Cancel</button>
        ${verify ? `<button type="button" class="btn" data-act="site-window">Open site window ${icon('external', 15)}</button>` : ''}
        <button type="button" class="btn btn--primary" data-act="intake-start" ${!crawl && count === 0 ? 'disabled' : ''}>${icon('download', 16)} ${label}</button>
      </div></div>`;
  }
  function bindFound(box) {
    $$('[data-scope]', box).forEach((b) => b.addEventListener('click', () => { state.intake.scope = b.dataset.scope; renderIntake(); if (b.dataset.scope === 'range') { const f = $('#rFrom'); if (f) f.focus(); } }));
    $$('input[name="scope"]', box).forEach((r) => r.addEventListener('change', () => { state.intake.scope = r.value; renderIntake(); }));
    $$('[data-speed]', box).forEach((b) => b.addEventListener('click', () => { state.intake.speed = b.dataset.speed; renderIntake(); }));
    const img = $('#optImages', box); if (img) img.addEventListener('change', () => { state.intake.images = img.checked; });
    const desc = $('.found__desc', box); if (desc) desc.addEventListener('click', () => { state.intake.descOpen = !state.intake.descOpen; desc.classList.toggle('is-open'); });
    ['rFrom', 'rTo'].forEach((id) => {
      const el = $('#' + id, box);
      if (!el) return;
      el.addEventListener('input', () => {
        state.intake[id === 'rFrom' ? 'from' : 'to'] = el.value;
        const n = intakeCount(state.intake.data, state.intake);
        const t = $('#rTally'); if (t) t.textContent = `${plural(n, 'chapter')} · get the rest later with Update`;
        const b = $('[data-act="intake-start"]'); if (b) { b.disabled = n === 0; b.lastChild.textContent = ' ' + (n > 0 ? `Download ${plural(n, 'chapter')}` : 'Nothing to download'); }
      });
    });
  }
  async function startIntake() {
    const it = state.intake, d = it.data;
    if (!d) return;
    const opts = { url: it.lookupUrl || normaliseUrl(it.url), startChapter: 0, endChapter: 0, concurrency: speedWorkers[it.speed || state.settings.speed] || 4, skipImages: it.images === false, fromPasted: false, force: false };
    if (it.scope === 'from') opts.fromPasted = true;
    if (it.scope === 'range') { opts.startChapter = parseInt(it.from, 10) || 1; opts.endChapter = parseInt(it.to, 10) || 0; if (opts.endChapter && opts.endChapter < opts.startChapter) { toast('The last chapter is before the first one.', { tone: 'bad' }); return; } }
    try {
      const r = await api('StartScrape', opts);
      const title = d.title;
      cancelIntake();
      if (r && r.status === 'queued') toast(`${title} is queued. It starts after the current download.`);
      else if (r && r.status === 'running') toast(`${title} is already downloading.`);
      loadActivity();
    } catch (e) { toast(e.message, { tone: 'bad' }); }
  }

  // ---------------------------------------------------------------- jobs
  function renderJobsEverywhere() {
    const lib = $('#jobs'); if (lib) renderJobs(lib);
    const nv = $('#novelJobs'); if (nv) renderJobs(nv, state.novelSlug);
    updateBadges();
  }
  let jobsRaf = 0;
  // Redraw on the next frame, or within 120 ms if frames are paused
  // (minimised window, background WebView).
  function scheduleJobs() {
    if (jobsRaf) return;
    const run = () => { if (!jobsRaf) return; cancelAnimationFrame(jobsRaf.f); clearTimeout(jobsRaf.t); jobsRaf = 0; renderJobsEverywhere(); };
    jobsRaf = { f: requestAnimationFrame(run), t: setTimeout(run, 120) };
  }

  function renderJobs(box, onlySlug) {
    const cur = state.activity.current;
    const queue = state.activity.queue || [];
    const results = state.results.filter((r) => !onlySlug || r.data.slug === onlySlug);
    const showCur = cur && (!onlySlug || cur.slug === onlySlug);
    const q = onlySlug ? queue.filter((j) => j.slug === onlySlug) : queue;
    const any = showCur || results.length || q.length;
    box.hidden = !any;
    if (!any) { box.innerHTML = ''; return; }
    const openLog = state.showLog;
    let html = onlySlug ? '' : `<h2 class="section-h">${cur || q.length ? 'Downloading' : 'Recent downloads'}</h2>`;
    if (showCur) html += currentCard(cur);
    else if (onlySlug && q.length) html += `<div class="card job" data-key="q-${q[0].id}"><div class="job__head"><span class="pill">${icon('clock', 12)} Queued</span><span class="muted" style="font-size:13px">Starts after the current download.</span><span class="spacer" style="margin-left:auto"></span><button type="button" class="btn btn--md" data-act="unqueue" data-id="${q[0].id}">Remove from queue</button></div></div>`;
    html += results.map(resultCard).join('');
    if (!onlySlug && queue.length) {
      html += `<div class="upnext"><b>Up next · ${queue.length}</b><span style="flex:1;min-width:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${queue.slice(0, 3).map((j) => esc(j.title) + (j.kind === 'update' ? ' · update' : '')).join(' · ')}</span>
        <button type="button" class="btn btn--link" data-act="toggle-queue" aria-expanded="${state.showQueue}">${state.showQueue ? 'Hide queue' : 'Manage queue'}</button></div>`;
      if (state.showQueue) {
        html += `<div class="card queue"><ul>${queue.map((j, i) => `<li><div style="width:28px;flex:none">${coverHTML(j.cover, j.title, '', true)}</div>
          <span class="q-title"><b>${esc(j.title)}</b><span>${i === 0 ? 'Next' : 'Then'} · ${j.kind === 'update' ? 'update' : 'download'}</span></span>
          <button type="button" class="icon-btn icon-btn--sm" data-act="unqueue" data-id="${j.id}" aria-label="Remove ${esc(j.title)} from the queue">${icon('x', 15)}</button></li>`).join('')}</ul>
          <div style="display:flex;justify-content:flex-end;padding:10px 14px;border-top:1px solid var(--line-2)"><button type="button" class="btn btn--md" data-act="stop-all">Stop all</button></div></div>`;
      }
    }
    box.innerHTML = html;
    settleJobs(box);
    if (openLog) { const l = $('.log', box); if (l) l.scrollTop = l.scrollHeight; }
  }
  // renderJobs rebuilds the cards' HTML on every progress event. Play the
  // entrance only the first time a card appears, and let the live progress
  // bar slide from where it was instead of restarting.
  const seenCards = new WeakMap();
  const barAt = new Map();
  function settleJobs(box) {
    let seen = seenCards.get(box);
    if (!seen) { seen = new Set(); seenCards.set(box, seen); }
    $$('.job[data-key]', box).forEach((el) => {
      if (!seen.has(el.dataset.key)) { seen.add(el.dataset.key); el.classList.add('job--enter'); }
    });
    $$('.bar[data-k] > i', box).forEach((i) => {
      const k = i.parentElement.dataset.k;
      const to = i.style.getPropertyValue('--p');
      const from = barAt.get(k);
      barAt.set(k, to);
      if (from == null || from === to) return;
      i.style.setProperty('--p', from);
      void i.offsetWidth; // commit the old value, then transition to the new one
      i.style.setProperty('--p', to);
    });
  }

  function jobHead(item, pill) {
    const n = novelBySlug(item.slug);
    const cover = (n && n.cover) || item.cover || '';
    const site = n ? n.site : hostOf(item.url);
    return `<div class="job__head"><div class="job__thumb">${coverHTML(cover, item.title, n && n.author, true)}</div>
      <div class="job__name"><b>${esc(item.title || 'Novel')}</b><span>${esc(site || '')}</span></div>${pill}</div>`;
  }
  function currentCard(cur) {
    const p = cur.progress;
    const waitingWifi = state.activity.waiting === 'wifi';
    const isUpdate = cur.kind === 'update';
    let pill, bar, line = '', notice = '', foot = '';
    const pctNum = p && p.total ? Math.max(0, Math.min(100, p.percentage || Math.round((p.current / p.total) * 100))) : 0;
    const crawlMode = p && p.eta === 'following links…';
    const logBtn = `<button type="button" class="btn btn--ghost btn--md spacer" data-act="toggle-log" aria-expanded="${state.showLog}" style="color:var(--muted);padding:0 6px">${state.showLog ? 'Hide log' : 'Show log'} ${icon('down', 14)}</button>`;
    const stop = `<button type="button" class="btn btn--md" data-act="stop">${icon('pause', 14)} ${isUpdate ? 'Stop' : 'Pause'}</button>`;
    if (cur.challenge) {
      pill = `<span class="pill pill--amber"><span class="dot"></span>Needs you</span>`;
      bar = `<div class="bar bar--still"><i style="--p:${(pctNum) / 100}"></i></div>`;
      line = `<div class="job__line"><b>${p ? `Waiting at chapter ${num((p.current || 0) + 1)}` : 'Waiting for you'}</b><span>${p ? num(p.current) + ' saved so far' : ''}</span></div>`;
      notice = `<div class="note note--amber">${icon('shield', 16)}<span><strong>${esc(hostOf(siteUrl(cur.challenge)) || cur.challenge)} wants to check you’re a person.</strong> Tick the box in the site window and the download continues by itself.</span></div>`;
      foot = `${logBtn}<button type="button" class="btn btn--md" data-act="stop">Cancel</button><button type="button" class="btn btn--primary btn--md" data-act="site-window" data-url="${esc(siteUrl(cur.challenge))}">Open site window ${icon('external', 14)}</button>`;
    } else if (waitingWifi) {
      pill = `<span class="pill"><span class="dot"></span>Waiting for Wi-Fi</span>`;
      bar = `<div class="bar bar--still"><i style="--p:${(pctNum) / 100}"></i></div>`;
      line = `<div class="job__line"><b>${p ? `Paused at ${num(p.current)} of ${num(p.total)}` : 'Paused'}</b><span>Continues on Wi-Fi</span></div>`;
      notice = `<div class="note">${icon('wifi', 16)}<span>“Only on Wi-Fi” is on in Settings, and you’re on mobile data.</span></div>`;
      foot = `${logBtn}${stop}`;
    } else if (!p || cur.phase === 'finding') {
      pill = `<span class="pill">${spinner(12, 'currentColor')} ${isUpdate ? 'Checking' : 'Finding chapters'}</span>`;
      bar = `<div class="bar bar--ind"><i></i></div>`;
      line = `<div class="job__line"><b>${isUpdate ? 'Checking details, poster and chapter list' : 'Reading the table of contents'}</b><span>${esc(trimMsg(cur.message))}</span></div>`;
      foot = `${logBtn}<button type="button" class="btn btn--md" data-act="stop">Cancel</button>`;
    } else if (cur.phase === 'revising' || p.stage === 'revising') {
      pill = `<span class="pill pill--rust">${spinner(12, 'currentColor')} Checking</span>`;
      bar = `<div class="bar" data-k="job-${cur.id}"><i style="--p:${(pctNum) / 100}"></i></div>`;
      line = `<div class="job__line"><b>Checked ${num(p.current)} of ${num(p.total)} saved chapters</b><span>${plural(p.scraped || 0, 'revision')} so far${eta(p.eta) ? ' · ' + eta(p.eta) + ' left' : ''}</span></div>`;
      notice = `<div class="note">${icon('info', 16)}<span>Only chapters whose text changed are replaced. Your place in the book stays where it is.</span></div>`;
      foot = `${logBtn}<button type="button" class="btn btn--md" data-act="stop">Stop checking</button><a class="btn btn--primary btn--md" href="#/novel/${encodeURIComponent(cur.slug)}">Open novel</a>`;
    } else {
      pill = `<span class="pill pill--rust">${spinner(12, 'currentColor')} ${isUpdate ? 'Updating' : 'Downloading'}</span>`;
      bar = crawlMode ? `<div class="bar bar--ind"><i></i></div>` : `<div class="bar" data-k="job-${cur.id}"><i style="--p:${(pctNum) / 100}"></i></div>`;
      const speed = (p.speed || '').replace('ch/s', 'chapters/s');
      const left = eta(p.eta);
      line = crawlMode
        ? `<div class="job__line"><b>${plural(p.current, 'chapter')} saved</b><span>Following “Next” links · ${esc(speed)}</span></div>`
        : `<div class="job__line"><b>${num(p.current)} of ${plural(p.total, 'chapter')}</b><span>${esc(speed)}${left ? ' · ' + left + ' left' : ''}</span></div>`;
      if (p.chapterNum) line += `<div class="job__line"><span>Saved chapter ${num(p.chapterNum)}${p.chapterTitle ? ': ' + esc(p.chapterTitle) : ''}</span>${p.failed ? `<span style="color:var(--bad)">${plural(p.failed, 'failed chapter')}</span>` : ''}</div>`;
      // A status newer than the last progress (site busy, retrying…)
      if (state.lastStatusAt > state.lastProgressAt + 3000 && cur.message) notice = `<div class="note">${icon('clock', 16)}<span>${esc(trimMsg(cur.message))}</span></div>`;
      foot = `${logBtn}${stop}`;
    }
    const log = state.showLog ? `<pre class="log" aria-label="Download log">${esc(state.log.slice(-200).join('\n') || 'Nothing logged yet.')}</pre>` : '';
    return `<div class="card job" data-key="cur-${cur.id}" aria-live="polite">${jobHead(cur, pill)}${bar}${line}${notice}${log}<div class="job__foot">${foot}</div></div>`;
  }
  function cap(m) { m = String(m || ''); return m.charAt(0).toUpperCase() + m.slice(1); }
  // Turn engine errors into something a reader can act on. Raw links are
  // shortened so they never dominate the card.
  function explainJobError(msg, url) {
    const m = String(msg || '');
    const site = hostOf(url) || (m.match(/https?:\/\/([^/\s)]+)/) || [])[1] || 'the site';
    if (/no chapter text found/i.test(m)) {
      const ch = (m.match(/chapter (\d+)/i) || [])[1];
      return `Couldn’t find the text of ${ch ? 'chapter ' + num(+ch) : 'a chapter'} on ${site.replace(/^www\./, '')}. If it showed a “Verify you are human” check, finish it in the site window, then press Try again.`;
    }
    if (/browser fetch failed|failed to fetch/i.test(m)) return `${site.replace(/^www\./, '')} didn’t answer from inside the site window. Check your connection, then try again.`;
    if (/verify you are human|check wasn't completed|challenge/i.test(m)) return `The “Verify you are human” check wasn’t finished in time. Press Try again and tick it in the site window.`;
    return cap(m.replace(/https?:\/\/\S{60,}/g, (u) => u.slice(0, 48) + '…'));
  }
  function trimMsg(m) { m = String(m || ''); return m.length > 140 ? m.slice(0, 137) + '…' : m; }

  function resultCard(r) {
    const d = r.data;
    const n = novelBySlug(d.slug);
    const dismiss = `<button type="button" class="btn btn--md" data-act="dismiss" data-rid="${r.id}">Dismiss</button>`;
    const readHref = (ch) => `/library/${encodeURIComponent(d.slug)}/${ch || ''}`;
    if (r.type === 'error') {
      const upd = d.kind === 'update';
      const head = jobHead(d, `<span class="pill pill--red"><span class="dot"></span>${upd ? 'Update failed' : 'Stopped'}</span>`);
      let saved = '';
      if (upd) saved = `<div class="job__line"><b>Couldn’t update</b><span>${d.total ? `${num(d.ok)} of ${plural(d.total, 'chapter')} are saved and readable` : ''}</span></div>`;
      else if (d.total) saved = `<div class="job__line"><b>Stopped at ${num(d.ok || 0)} of ${num(d.total)}</b><span>${d.ok ? plural(d.ok, 'chapter') + ' are saved' : ''}</span></div>`;
      return `<div class="card job" data-key="r-${r.id}">${head}${d.total && !upd ? `<div class="bar bar--bad"><i style="--p:${(Math.round(((d.ok || 0) / d.total) * 100)) / 100}"></i></div>` : ''}${saved}
        <div class="note note--red">${icon('alert', 16)}<span>${esc(explainJobError(d.message, d.url))}</span></div>
        <div class="job__foot"><span class="spacer"></span>${dismiss}<button type="button" class="btn btn--primary btn--md" data-act="resume" data-rid="${r.id}">Try again</button></div></div>`;
    }
    if (r.type === 'stopped') {
      const head = jobHead(d, `<span class="pill"><span class="dot"></span>Paused</span>`);
      return `<div class="card job" data-key="r-${r.id}">${head}${d.total ? `<div class="bar bar--still"><i style="--p:${(Math.round(((d.ok || 0) / d.total) * 100)) / 100}"></i></div>` : ''}
        <div class="job__line"><b>${d.kind === 'update' ? 'Update paused' : d.total ? `Paused at ${num(d.ok)} of ${num(d.total)}` : 'Paused'}</b><span>${d.kind === 'update' && d.total ? `${num(d.ok)} of ${plural(d.total, 'chapter')} saved and readable` : 'Everything so far is saved and readable'}</span></div>
        <div class="job__foot"><span class="spacer"></span>${dismiss}<button type="button" class="btn btn--primary btn--md" data-act="resume" data-rid="${r.id}">Resume</button></div></div>`;
    }
    // done
    const gaps = (d.failed || 0) + (d.locked || 0);
    if (d.kind === 'update') {
      const rows = [];
      const nw = d.newChapters || [];
      if (nw.length) rows.push(['NEW', 'var(--ok)', `${plural(nw.length, 'chapter')} · ${listNums(nw)}`, `<a class="btn btn--link" href="${readHref(nw[0])}">Read</a>`]);
      if ((d.revised || []).length) rows.push(['REVISED', 'var(--accent)', `${plural(d.revised.length, 'chapter')} · ${listNums(d.revised)}`, `<a class="btn btn--link" href="#/novel/${encodeURIComponent(d.slug)}">Show</a>`]);
      if (d.coverChanged) rows.push(['POSTER', 'var(--ink-3)', 'New poster from the site', '']);
      if ((d.details || []).length) rows.push(['DETAILS', 'var(--ink-3)', detailText(d), '']);
      if (d.fixed) rows.push(['FIXED', 'var(--ink-3)', `${plural(d.fixed, 'chapter')} that failed last time`, '']);
      if (d.failed) rows.push(['FAILED', 'var(--bad)', `${plural(d.failed, 'chapter')} didn’t download`, `<button type="button" class="btn btn--link" data-act="update" data-slug="${esc(d.slug)}">Retry</button>`]);
      if (!rows.length) {
        const checked = d.checked ? `all ${num(d.checked)} saved chapters` : `the chapter list`;
        return `<div class="card job" data-key="r-${r.id}"><div class="job__head"><div class="job__thumb">${coverHTML((n && n.cover) || '', d.title, n && n.author, true)}</div>
          <div class="job__name"><b>Nothing new for ${esc(d.title)}</b><span>Checked details, poster and ${checked} · just now</span></div>
          <button type="button" class="icon-btn icon-btn--sm" data-act="dismiss" data-rid="${r.id}" aria-label="Dismiss">${icon('x', 15)}</button></div></div>`;
      }
      const pill = `<span class="pill pill--green"><span class="dot"></span>Updated</span>`;
      return `<div class="card job" data-key="r-${r.id}">${jobHead(d, pill)}<div class="job__line"><span>Updated ${ago(r.at)} · ${took(d.seconds)}</span></div>
        <ul class="receipt">${rows.map(([t, c, txt, act]) => `<li><span class="tag" style="color:${c}">${t}</span><span class="txt">${txt}</span>${act}</li>`).join('')}</ul>
        <div class="job__foot"><span class="spacer"></span>${dismiss}${nw.length ? `<a class="btn btn--primary btn--md" href="${readHref(nw[0])}">Read Ch. ${num(nw[0])}</a>` : `<a class="btn btn--primary btn--md" href="#/novel/${encodeURIComponent(d.slug)}">Open novel</a>`}</div></div>`;
    }
    const pill = gaps ? `<span class="pill pill--amber"><span class="dot"></span>Done with gaps</span>` : `<span class="pill pill--green"><span class="dot"></span>Done</span>`;
    const headline = gaps ? `${num(d.ok)} of ${plural(d.total, 'chapter')} saved` : `${plural(d.ok, 'chapter')} saved`;
    const sub = gaps ? 'You can start reading now' : `${mb(d.sizeBytes)} · ${took(d.seconds)}`;
    const gapNote = gaps ? `<div class="note note--red">${icon('alert', 16)}<span>${[d.failed ? `${plural(d.failed, 'chapter')} didn’t download.` : '', d.locked ? `${plural(d.locked, 'paid chapter')} ${d.locked === 1 ? 'was' : 'were'} skipped; ${d.locked === 1 ? 'it stays' : 'they stay'} locked on the site.` : ''].filter(Boolean).join(' ')}</span></div>` : '';
    const first = n && n.lastRead ? n.lastRead : '';
    const read = `<a class="btn ${d.failed ? '' : 'btn--primary '}btn--md" href="${readHref(first)}">${first ? 'Continue' : 'Start reading'}</a>`;
    return `<div class="card job" data-key="r-${r.id}">${jobHead(d, pill)}<div class="bar bar--ok"><i style="--p:${(d.total ? Math.round((d.ok / d.total) * 100) : 100) / 100}"></i></div>
      <div class="job__line"><b>${headline}</b><span>${sub}</span></div>${gapNote}
      <div class="job__foot"><span class="spacer"></span>${dismiss}${d.failed ? `${read}<button type="button" class="btn btn--primary btn--md" data-act="update" data-slug="${esc(d.slug)}">Retry ${num(d.failed)}</button>` : `<button type="button" class="btn btn--md" data-act="epub" data-slug="${esc(d.slug)}">Export EPUB</button>${read}`}</div></div>`;
  }
  function detailText(d) {
    const parts = (d.details || []).map((k) => (k === 'status' && d.status ? `status is now ${d.status}` : `${k} updated`));
    return cap(parts.join(', '));
  }
  function addResult(type, data) {
    const r = { id: Date.now() + Math.random().toString(36).slice(2, 6), type, data, at: new Date().toISOString() };
    state.results = [r].concat(state.results.filter((x) => !(x.data.slug && x.data.slug === data.slug))).slice(0, 6);
    store.set('na-results', state.results);
    const clean = type === 'done' && !(data.failed || data.locked) && (data.kind !== 'update' || isEmptyUpdate(data));
    if (clean) setTimeout(() => dismissResult(r.id), 10000);
  }
  function isEmptyUpdate(d) { return !(d.newChapters || []).length && !(d.revised || []).length && !d.coverChanged && !(d.details || []).length && !d.fixed && !d.failed; }
  function dismissResult(id) {
    const before = state.results.length;
    state.results = state.results.filter((r) => r.id !== id);
    if (state.results.length !== before) { store.set('na-results', state.results); renderJobsEverywhere(); }
  }

  // ---------------------------------------------------------------- shelf
  function badgeFor(n) {
    const cur = state.activity.current;
    if (cur && cur.slug === n.slug) {
      if (cur.challenge) return `<span class="book__badge book__badge--amber"><span class="dot" style="background:#8A5A00"></span>Needs you</span>`;
      const p = cur.progress;
      if (cur.phase === 'revising') return `<span class="book__badge"><span class="dot"></span>Checking ${p ? p.percentage + '%' : ''}</span>`;
      if (!p || cur.phase === 'finding') return `<span class="book__badge">${spinner(11, '#E0794F')} ${cur.kind === 'update' ? 'Checking' : 'Finding chapters'}</span>`;
      return `<span class="book__badge"><span class="dot"></span>${cur.kind === 'update' ? 'Updating' : 'Downloading'} ${p.percentage || 0}%</span>`;
    }
    if ((state.activity.queue || []).some((j) => j.slug === n.slug)) return `<span class="book__badge"><span class="dot" style="background:#B5AC9E"></span>Queued</span>`;
    if (n.failedCount) return `<span class="book__badge book__badge--red">${icon('alert', 12, 2.4)} ${num(n.failedCount)} failed</span>`;
    if (isFresh(n)) return `<span class="book__badge book__badge--green"><span class="dot" style="background:#2F6B3F"></span>${plural(n.lastNew, 'new chapter')}</span>`;
    return '';
  }
  function updateBadges() {
    $$('.book[data-slug]').forEach((el) => {
      const n = novelBySlug(el.dataset.slug);
      if (!n) return;
      const html = badgeFor(n);
      const slot = $('.book__badge-slot', el);
      if (slot && slot.dataset.h !== html) { slot.innerHTML = html; slot.dataset.h = html; }
    });
  }
  function filteredBooks() {
    const q = state.query.trim().toLowerCase();
    let list = state.novels.slice();
    const f = state.filter;
    list = list.filter((n) => {
      const r = readInfo(n);
      if (f === 'reading' && !r.reading) return false;
      if (f === 'new' && r.started) return false;
      if (f === 'finished' && !r.finished) return false;
      if (f === 'attention' && !n.failedCount) return false;
      if (state.status !== 'any' && bookStatus(n) !== state.status) return false;
      if (q && !(`${n.title} ${n.author} ${n.site}`.toLowerCase().includes(q))) return false;
      return true;
    });
    const t = (x) => (x ? new Date(x).getTime() || 0 : 0);
    const sorts = {
      read: (a, b) => (t(b.lastReadAt) || t(b.updatedAt) / 1e3) - (t(a.lastReadAt) || t(a.updatedAt) / 1e3),
      added: (a, b) => t(b.addedAt) - t(a.addedAt),
      title: (a, b) => a.title.localeCompare(b.title),
      chapters: (a, b) => (b.totalChapters || 0) - (a.totalChapters || 0),
    };
    list.sort(sorts[state.sort] || sorts.read);
    return list;
  }
  function bookCard(n) {
    const r = readInfo(n);
    const href = `#/novel/${encodeURIComponent(n.slug)}`;
    const st = STATUS_LABEL[bookStatus(n)] || '';
    let meta;
    if (r.finished) meta = `Finished · ${num(r.total)} ch.`;
    else if (r.started) meta = `Ch. ${num(r.last)} of ${num(r.total)}`;
    else meta = n.totalChapters ? `${plural(n.totalChapters, 'chapter')} · not started` : 'Not started';
    if (st) meta += ` · ${st}`;
    const cont = r.started && !r.finished ? `/library/${encodeURIComponent(n.slug)}/${r.last}` : `/library/${encodeURIComponent(n.slug)}/`;
    const contLabel = r.started && !r.finished ? `Continue · Ch. ${num(r.last)}` : r.finished ? 'Read again' : 'Start reading';
    const badge = badgeFor(n);
    return `<article class="book" data-slug="${esc(n.slug)}">
      <a class="book__cover" href="${href}" aria-label="Open ${esc(n.title)}">${coverHTML(n.cover, n.title, n.author, false)}<span class="book__badge-slot" data-h="${esc(badge)}">${badge}</span></a>
      <div class="book__hover" aria-hidden="true"><a href="${cont}" tabindex="-1">${icon('book', 14)} ${contLabel}</a></div>
      <button type="button" class="book__more" data-act="book-menu" data-slug="${esc(n.slug)}" aria-label="More actions for ${esc(n.title)}" aria-haspopup="menu" aria-expanded="false">${icon('more', 16, 2.4)}</button>
      <div class="book__info"><a class="book__title" href="${href}">${esc(n.title)}</a><span class="book__author">${esc(n.author || n.site || '')}</span></div>
      <div class="stack" style="gap:6px"><div class="bar bar--thin" aria-hidden="true"><i style="--p:${(r.pct) / 100}"></i></div><span class="book__meta">${esc(meta)}</span></div>
    </article>`;
  }
  function renderShelf() {
    const box = $('#shelf');
    if (!box) return;
    if (!state.loaded) { box.innerHTML = `<div class="hint">Loading your library…</div>`; return; }
    const all = state.novels;
    if (!all.length) {
      box.innerHTML = `<div class="empty">
        <div class="shelf-art" aria-hidden="true"><i style="width:22px;height:78px;background:#23344F"></i><i style="width:26px;height:92px;background:#6E2A2A"></i><i style="width:18px;height:70px;background:#2F4A3A"></i><i style="width:24px;height:84px;background:#7A5518;position:relative"><i style="position:absolute;left:7px;top:0;width:10px;height:30px;background:#A8431F;border-radius:0"></i></i></div>
        <h3 id="libH">Your shelf is empty</h3>
        <p>Paste a link to a novel, or to any chapter you’re reading. It’s saved to this device and opens without internet.</p>
        <button type="button" class="btn btn--primary" data-act="focus-url">Paste a link</button></div>`;
      return;
    }
    const counts = { all: all.length, reading: 0, new: 0, finished: 0, attention: 0 };
    let hasStatus = false;
    let bytes = 0;
    all.forEach((n) => { const r = readInfo(n); if (r.reading) counts.reading++; if (!r.started) counts.new++; if (r.finished) counts.finished++; if (n.failedCount) counts.attention++; if (bookStatus(n)) hasStatus = true; bytes += n.sizeBytes || 0; });
    const chips = [['all', 'All'], ['reading', 'Reading'], ['new', 'Not started'], ['finished', 'Finished'], ['attention', 'Needs attention']]
      .filter(([k]) => k === 'all' || counts[k] > 0 || state.filter === k)
      .map(([k, l]) => `<button type="button" class="chip" data-filter="${k}" aria-pressed="${state.filter === k}">${k === 'attention' ? '<span class="dot"></span>' : ''}${l} <span class="n">${counts[k]}</span></button>`).join('');
    const list = filteredBooks();
    const grid = list.length ? `<div class="grid">${list.map(bookCard).join('')}</div>`
      : `<div class="empty" style="padding:32px"><h3 style="font-size:22px">No novels match</h3><p>${state.query ? `Nothing in your library matches “${esc(state.query)}”.` : 'Nothing in this view yet.'}</p><button type="button" class="btn" data-act="clear-filters">Show all novels</button></div>`;
    box.innerHTML = `<div class="lib-head">
        <div class="titles"><h2 id="libH">Library</h2><p>${plural(all.length, 'novel')} · ${mb(bytes)} on this device</p></div>
        <button type="button" class="btn btn--md" data-act="update-all">${icon('refresh', 15)} Update all</button>
        ${hasStatus ? `<label>Status <select class="select" id="statusSel">${[['any', 'Any'], ['ongoing', 'Ongoing'], ['completed', 'Completed'], ['hiatus', 'On hiatus']].map(([v, l]) => `<option value="${v}" ${state.status === v ? 'selected' : ''}>${l}</option>`).join('')}</select></label>` : ''}
        <label>Sort <select class="select" id="sortSel">${[['read', 'Last read'], ['added', 'Recently added'], ['title', 'Title'], ['chapters', 'Most chapters']].map(([v, l]) => `<option value="${v}" ${state.sort === v ? 'selected' : ''}>${l}</option>`).join('')}</select></label>
      </div>
      <div class="chips" role="group" aria-label="Filter library">${chips}</div>${grid}`;
    const ss = $('#sortSel'); if (ss) ss.addEventListener('change', () => { state.sort = ss.value; store.set('na-sort', ss.value); renderShelf(); });
    const st = $('#statusSel'); if (st) st.addEventListener('change', () => { state.status = st.value; renderShelf(); });
    $$('[data-filter]', box).forEach((b) => b.addEventListener('click', () => { state.filter = b.dataset.filter; store.set('na-filter', state.filter); renderShelf(); }));
  }

  function bookMenu(anchor, slug) {
    const n = novelBySlug(slug);
    if (!n) return;
    const items = [
      { label: 'Open novel page', fn: () => { location.hash = `#/novel/${encodeURIComponent(slug)}`; } },
      { label: 'Update', fn: () => updateNovel(slug) },
      { label: 'Export EPUB', fn: () => exportEpub(slug) },
    ];
    if (n.url) items.push({ label: 'Open original page', fn: () => openExternal(n.url) });
    items.push('sep', { label: 'Remove…', danger: true, fn: () => removeNovel(slug) });
    openMenu(anchor, items);
  }

  // ---------------------------------------------------------------- actions
  async function updateNovel(slug) {
    try {
      const r = await api('UpdateNovel', slug).catch(() => api('RetryNovel', slug));
      const n = novelBySlug(slug);
      if (r && r.status === 'queued') toast(`${n ? n.title : 'Update'} is queued.`);
      else if (r && r.status === 'running') toast(`${n ? n.title : 'It'} is already updating.`);
      loadActivity();
    } catch (e) { toast(e.message, { tone: 'bad' }); }
  }
  async function updateAll() {
    try {
      const r = await api('UpdateAll', false);
      toast(r && r.queued ? `Updating ${plural(r.queued, 'novel')}, one at a time.` : 'Everything is already queued.');
      loadActivity();
    } catch (e) { toast(e.message, { tone: 'bad' }); }
  }
  let exporting = false;
  async function exportEpub(slug) {
    if (exporting) return;
    exporting = true;
    const n = novelBySlug(slug) || (state.novel && state.novel.slug === slug ? state.novel : null);
    const close = toast(`Building the EPUB${n ? ' for ' + n.title : ''}…`, { timeout: 0 });
    try {
      const r = await api('ExportEPUB', slug);
      close();
      if (!r) return; // dialog cancelled
      const where = state.platform === 'android' ? `${r.path || 'Downloads'}` : '';
      toast(`Saved ${r.fileName} · ${r.sizeMb}${where ? ' · ' + where : ''}`, state.platform === 'android' ? { tone: 'ok' } : { tone: 'ok', action: { label: 'Show in folder', fn: () => api('ShowFileInFolder', r.path).catch(() => {}) } });
    } catch (e) { close(); toast(`Couldn’t save the EPUB: ${e.message}`, { tone: 'bad' }); }
    finally { exporting = false; }
  }
  async function removeNovel(slug) {
    const n = novelBySlug(slug) || state.novel;
    if (!n) return;
    const res = await confirmDialog({
      title: `Remove ${n.title}?`,
      body: `Deletes ${plural(n.cleanCount || 0, 'saved chapter')} and their images (${esc(n.sizeMb || mb(n.sizeBytes))}) from this device. Your reading place is lost too. This can’t be undone.`,
      ok: 'Remove novel', cancel: 'Keep it', danger: true,
      cover: coverHTML(n.cover, n.title, n.author, true),
      extra: `<label style="display:flex;align-items:center;gap:10px;font-size:14px"><input type="checkbox" class="check" name="epub"> Export an EPUB copy first</label>`,
    });
    if (!res) return;
    if (res.epub) { await exportEpub(slug); }
    try {
      await api('DeleteNovel', slug);
      toast(`Removed ${n.title}.`);
      state.results = state.results.filter((r) => r.data.slug !== slug); store.set('na-results', state.results);
      if (route().name === 'novel') location.hash = '#/';
      loadLibrary();
    } catch (e) { toast(e.message, { tone: 'bad' }); }
  }
  async function redownload(slug) {
    const n = novelBySlug(slug) || state.novel;
    const ok = await confirmDialog({ title: 'Download everything again?', body: `Every saved chapter of ${esc(n ? n.title : 'this novel')} is downloaded fresh and replaces your copy. Use this if chapters look broken. It can take a while.`, ok: 'Download again' });
    if (!ok) return;
    try { await api('RedownloadNovel', slug); loadActivity(); } catch (e) { toast(e.message, { tone: 'bad' }); }
  }
  function openExternal(url) { api('OpenExternalURL', url).catch(() => { window.open(url, '_blank'); }); }
  async function openSiteWindow(url) {
    try { await api('OpenSiteBrowser', url || ''); } catch (e) { toast(e.message, { tone: 'bad' }); }
  }

  // ================================================================ NOVEL PAGE
  function renderNovelPage(main, slug) {
    if (state.novelSlug !== slug) { state.novel = null; state.novelSlug = slug; state.tocFilter = 'all'; state.tocQuery = ''; state.synopsisOpen = false; state.tocAll = false; }
    main.innerHTML = `<div class="wrap stack" style="gap:28px" id="novelPage"><a class="back" href="#/">${icon('back', 16)} Library</a><div class="hint">Loading…</div></div>`;
    if (state.novel) paintNovel();
    loadNovel(slug);
  }
  async function loadNovel(slug, quiet) {
    try {
      const d = await api('GetNovel', slug);
      if (route().name !== 'novel' || route().slug !== slug) return;
      state.novel = d;
      paintNovel();
    } catch (e) {
      if (quiet) return;
      const p = $('#novelPage');
      if (p) p.innerHTML = `<a class="back" href="#/">${icon('back', 16)} Library</a><div class="empty"><h3>This novel isn’t in your library</h3><p>${esc(e.message)}</p><a class="btn" href="#/">Back to library</a></div>`;
    }
  }
  function paintNovel() {
    const d = state.novel, page = $('#novelPage');
    if (!d || !page) return;
    const r = readInfo(d);
    const okChapters = (d.chapters || []).filter((c) => c.state === 'ok');
    const firstOk = okChapters[0];
    const st = bookStatus(d);
    const readHref = r.started && !r.finished ? `/library/${encodeURIComponent(d.slug)}/${r.last}` : `/library/${encodeURIComponent(d.slug)}/${firstOk ? firstOk.number : 1}`;
    const readLabel = r.started && !r.finished ? `Continue · Ch. ${num(r.last)}` : r.finished ? 'Read again' : 'Start reading';
    const missing = (d.failedCount || 0) + (d.pendingCount || 0);
    const notes = [];
    if (d.failedCount) notes.push(`<div class="note note--red" style="align-items:center;flex-wrap:wrap">${icon('alert', 18)}<span style="flex:1 1 260px"><strong>${plural(d.failedCount, 'chapter')} didn’t download.</strong>${d.lockedCount ? ` ${plural(d.lockedCount, 'paid chapter')} ${d.lockedCount === 1 ? 'was' : 'were'} skipped; ${d.lockedCount === 1 ? 'it stays' : 'they stay'} locked on the site.` : ''}</span><button type="button" class="btn btn--danger btn--sm" data-act="update" data-slug="${esc(d.slug)}">Retry ${num(d.failedCount)}</button></div>`);
    else if (d.lockedCount) notes.push(`<div class="note">${icon('lock', 16)}<span>${plural(d.lockedCount, 'paid chapter')} ${d.lockedCount === 1 ? 'is' : 'are'} locked on ${esc(d.site)} and ${d.lockedCount === 1 ? 'wasn’t' : 'weren’t'} downloaded.</span></div>`);
    if (d.removedCount) notes.push(`<div class="note">${icon('info', 16)}<span>${plural(d.removedCount, 'chapter')} ${d.removedCount === 1 ? 'is' : 'are'} no longer on the site. Your ${d.removedCount === 1 ? 'copy is' : 'copies are'} kept.</span></div>`);
    const imgBytes = d.imageBytes || 0, total = d.sizeBytes || 0, text = Math.max(0, total - imgBytes);
    const imgPct = total ? Math.round((imgBytes / total) * 100) : 0;
    const delTo = Math.max(0, (r.last || 0) - 1);
    page.innerHTML = `<a class="back" href="#/">${icon('back', 16)} Library</a>
      <div id="novelJobs" class="jobs" hidden></div>
      <section class="novel-hero">
        <div class="novel-hero__cover">${coverHTML(d.cover, d.title, d.author, false)}${d.cover ? '' : `<p>No poster on ${esc(d.site || 'the site')} yet. Update swaps in the real one when it appears.</p>`}</div>
        <div class="novel-hero__head">
          <div class="novel-hero__src">${st ? `<span class="status-chip ${st === 'ongoing' ? 'status-chip--ongoing' : ''}">${esc(STATUS_LABEL[st] || d.status)}</span>` : ''}<span>${esc(d.site || '')}</span>${d.url ? `<span aria-hidden="true">·</span><a href="#" data-act="open-source">${esc(hostOf(d.url))}</a>` : ''}</div>
          <h1>${esc(d.title)}</h1>
          ${d.author ? `<span class="by">by ${esc(d.author)}</span>` : ''}
        </div>
        <div class="novel-hero__rest">
          <dl class="facts">
            <div><dt>Chapters</dt><dd>${num(d.totalChapters)}</dd></div>
            <div><dt>Saved</dt><dd>${num(d.cleanCount)}</dd></div>
            <div><dt>On disk</dt><dd>${esc(mb(total))}</dd></div>
            <div><dt>Last updated</dt><dd>${esc(ago(d.checkedAt) || ago(d.updatedAt) || '—')}</dd></div>
          </dl>
          ${d.description ? `<div><p class="synopsis${state.synopsisOpen ? ' is-open' : ''}" id="synopsis">${esc(d.description)}</p><button type="button" class="btn btn--link" data-act="synopsis" style="padding-left:0">${state.synopsisOpen ? 'Less' : 'More'}</button></div>` : ''}
          <div class="actions">
            <a class="btn btn--primary btn--lg" href="${readHref}" ${okChapters.length ? '' : 'aria-disabled="true"'}>${icon('book', 17)} ${readLabel}</a>
            <button type="button" class="btn btn--lg" data-act="update" data-slug="${esc(d.slug)}">${icon('refresh', 16)} Update</button>
            <button type="button" class="btn btn--lg" data-act="epub" data-slug="${esc(d.slug)}" ${okChapters.length ? '' : 'disabled'}>${icon('download', 16)} Export EPUB</button>
            <button type="button" class="icon-btn icon-btn--line" style="width:48px;height:48px" data-act="novel-menu" aria-label="More actions" aria-haspopup="menu" aria-expanded="false">${icon('more', 18, 2.4)}</button>
          </div>
        </div>
      </section>
      <div class="two-col">
        <section class="main" aria-labelledby="chH">
          <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
            <h2 class="h2-serif" id="chH" style="margin-right:auto">Chapters</h2>
            <label class="field" style="width:240px;max-width:100%;height:40px">${icon('search', 15)}<input type="search" id="tocQ" placeholder="Number or title" aria-label="Find a chapter" value="${esc(state.tocQuery)}"></label>
          </div>
          ${notes.join('')}
          <div class="chips" role="group" aria-label="Show chapters">
            <button type="button" class="chip" data-toc="all" aria-pressed="${state.tocFilter === 'all'}">All <span class="n">${num(d.totalChapters)}</span></button>
            <button type="button" class="chip" data-toc="unread" aria-pressed="${state.tocFilter === 'unread'}">Unread <span class="n">${num(Math.max(0, d.totalChapters - (r.last || 0)))}</span></button>
            ${missing || d.lockedCount ? `<button type="button" class="chip" data-toc="missing" aria-pressed="${state.tocFilter === 'missing'}">Missing <span class="n">${num(missing + (d.lockedCount || 0))}</span></button>` : ''}
            ${d.revisedCount ? `<button type="button" class="chip" data-toc="revised" aria-pressed="${state.tocFilter === 'revised'}">Revised <span class="n">${num(d.revisedCount)}</span></button>` : ''}
            ${r.started ? `<button type="button" class="btn btn--link" data-act="jump" style="margin-left:auto">Jump to where you are</button>` : ''}
          </div>
          <ol class="card toc" id="toc"></ol>
        </section>
        <aside class="side" aria-labelledby="stH">
          <h2 class="h2-serif" id="stH">Storage</h2>
          <div class="card storage">
            <div class="storage__big"><b>${esc(mb(total))}</b><span class="muted" style="font-size:13px">${plural(d.cleanCount || 0, 'chapter')}</span></div>
            <div class="split" aria-hidden="true"><i style="width:${100 - imgPct}%;background:var(--ink)"></i><i style="width:${imgPct}%;background:#D4A58E"></i></div>
            <div class="legend"><span><i style="background:var(--ink)"></i>Text ${esc(mb(text))}</span><span><i style="background:#D4A58E"></i>Images ${esc(mb(imgBytes))}</span></div>
            <button type="button" class="btn" data-act="del-images" ${imgBytes ? '' : 'disabled'}>${icon('image', 16)} ${imgBytes ? `Delete images · frees ${esc(mb(imgBytes))}` : 'No images saved'}</button>
          </div>
          <div class="card storage">
            <h3 style="font-size:15px;font-weight:600">Free up chapters you’ve read</h3>
            <div class="pair"><label>From<input class="num" type="number" min="1" id="delFrom" value="1"></label><label>To<input class="num" type="number" min="1" id="delTo" value="${delTo || ''}" placeholder="${num(d.totalChapters)}"></label></div>
            <button type="button" class="btn" data-act="del-range">Delete these chapters</button>
            <p class="muted" style="font-size:12px;line-height:1.5">They stay in the list. Update downloads them again if you want them back.</p>
          </div>
        </aside>
      </div>`;
    renderJobs($('#novelJobs'), d.slug);
    paintToc();
    const q = $('#tocQ'); if (q) q.addEventListener('input', () => { state.tocQuery = q.value; paintToc(); });
    $$('[data-toc]', page).forEach((b) => b.addEventListener('click', () => { state.tocFilter = b.dataset.toc; $$('[data-toc]', page).forEach((x) => x.setAttribute('aria-pressed', x === b)); paintToc(); }));
  }
  function paintToc() {
    const d = state.novel, ol = $('#toc');
    if (!d || !ol) return;
    const last = d.lastRead || 0;
    const q = state.tocQuery.trim().toLowerCase();
    let rows = d.chapters || [];
    if (state.tocFilter === 'unread') rows = rows.filter((c) => c.number >= last);
    if (state.tocFilter === 'missing') rows = rows.filter((c) => c.state !== 'ok');
    if (state.tocFilter === 'revised') rows = rows.filter((c) => c.revised);
    if (q) rows = rows.filter((c) => String(c.number).startsWith(q) || String(c.title || '').toLowerCase().includes(q));
    const LIMIT = state.tocAll ? Infinity : 400;
    let start = 0;
    if (!q && state.tocFilter === 'all' && last && rows.length > LIMIT) start = Math.max(0, rows.findIndex((c) => c.number >= last) - 40);
    const shown = rows.slice(start, start + LIMIT);
    if (!rows.length) { ol.innerHTML = `<li><span class="t is-dim">No chapters match.</span></li>`; return; }
    ol.innerHTML = (start > 0 ? `<li class="toc-more"><button type="button" class="btn btn--link" data-act="toc-all">Show chapters 1–${num(rows[start - 1].number)}</button></li>` : '') + shown.map((c) => {
      const ok = c.state === 'ok';
      const cur = last && c.number === last;
      const read = ok && last && c.number < last;
      let s = '';
      if (cur) s = `<span class="s s--here"><svg width="10" height="13" viewBox="0 0 12 16" aria-hidden="true"><path d="M1 0h10v15l-5-4-5 4z" fill="currentColor"/></svg>You’re here</span>`;
      else if (c.removed) s = `<span class="s">Removed from site · kept</span>`;
      else if (c.revised && ok) s = `<span class="s s--ok">${icon('pencil', 12, 2.2)} Revised ${esc(shortDate(c.revised))}</span>`;
      else if (c.state === 'failed') s = `<span class="s s--bad" title="${esc(c.error || '')}">Didn’t download</span><button type="button" class="btn btn--sm" data-act="update" data-slug="${esc(d.slug)}">Retry</button>`;
      else if (c.state === 'locked') s = `<span class="s">${icon('lock', 13)} Paid on site</span>`;
      else if (!ok) s = `<span class="s">Not saved</span>`;
      else if (read) s = `<span class="s">Read</span>`;
      const title = esc(c.title || `Chapter ${c.number}`);
      const t = ok ? `<a class="t" href="${esc(c.readerUrl)}">${title}</a>` : `<span class="t is-dim">${title}</span>`;
      return `<li class="${cur ? 'is-current' : ''}${read ? ' is-read' : ''}" data-n="${c.number}"><span class="n">${num(c.number)}</span>${t}${s}</li>`;
    }).join('') + (rows.length > start + LIMIT ? `<li class="toc-more"><button type="button" class="btn btn--link" data-act="toc-more">Show ${num(Math.min(LIMIT, rows.length - start - LIMIT))} more</button></li>` : '');
  }
  function shortDate(s) { const d = new Date(s); return isNaN(d) ? s : d.toLocaleDateString(undefined, { day: 'numeric', month: 'short' }); }
  function novelMenu(anchor) {
    const d = state.novel;
    if (!d) return;
    const first = (d.chapters || []).find((c) => c.state === 'ok');
    const items = [];
    if (first) items.push({ label: 'Read from chapter 1', fn: () => { location.href = `/library/${encodeURIComponent(d.slug)}/${first.number}`; } });
    if (d.url) items.push({ label: 'Download a range…', sub: 'Pick chapters, speed and images.', fn: () => { location.hash = '#/'; setTimeout(() => { lookup(d.url).then(() => { state.intake.scope = 'range'; renderIntake(); }); }, 30); } });
    items.push({ label: 'Download everything again…', sub: 'Replaces every saved copy. Use if chapters look broken.', fn: () => redownload(d.slug) });
    if (d.url) items.push({ label: 'Open original page', fn: () => openExternal(d.url) });
    if (state.platform === 'desktop') items.push({ label: 'Show files in folder', fn: () => api('ShowNovelFolder', d.slug).catch((e) => toast(e.message, { tone: 'bad' })) });
    items.push('sep', { label: 'Remove from library…', danger: true, fn: () => removeNovel(d.slug) });
    openMenu(anchor, items);
  }
  async function deleteImages() {
    const d = state.novel; if (!d) return;
    const ok = await confirmDialog({ title: 'Delete this novel’s images?', body: `Frees ${esc(mb(d.imageBytes))}. The text stays readable; pictures inside chapters will be missing until you download everything again.`, ok: 'Delete images', danger: true });
    if (!ok) return;
    try { await api('DeleteImages', d.slug); toast(`Deleted images · freed ${mb(d.imageBytes)}`); loadNovel(d.slug); loadLibrary(); } catch (e) { toast(e.message, { tone: 'bad' }); }
  }
  async function deleteRange() {
    const d = state.novel; if (!d) return;
    const f = parseInt($('#delFrom').value, 10), t = parseInt($('#delTo').value, 10);
    if (!f || !t || t < f) { toast('Enter the first and last chapter to delete.', { tone: 'bad' }); return; }
    const count = (d.chapters || []).filter((c) => c.state === 'ok' && c.number >= f && c.number <= t).length;
    if (!count) { toast('None of those chapters are saved.'); return; }
    const ok = await confirmDialog({ title: `Delete chapters ${num(f)}–${num(t)}?`, body: `Removes ${plural(count, 'saved chapter')} from this device. ${f <= (d.lastRead || 0) && t >= (d.lastRead || 0) ? '<strong>This includes the chapter you’re on.</strong> ' : ''}They stay in the list, and Update can download them again.`, ok: 'Delete chapters', danger: true });
    if (!ok) return;
    const before = d.sizeBytes || 0;
    try {
      const r = await api('DeleteChapters', d.slug, f, t);
      toast(`Deleted ${plural(r.deleted || 0, 'chapter')} · freed ${mb(Math.max(0, before - (r.sizeBytes || 0)))}`);
      loadNovel(d.slug); loadLibrary();
    } catch (e) { toast(e.message, { tone: 'bad' }); }
  }

  // ================================================================ SETTINGS
  const SITES = ['NovelBin', 'NovelFull', 'ReadNovelFull', 'Royal Road', 'WuxiaWorld', 'FreeWebNovel', 'NovelFire', 'Webnovel', 'ScribbleHub', 'EmpireNovel', 'Any site in reader mode'];
  async function renderSettings(main) {
    try { state.stats = await api('GetSystemStats'); } catch (e) {}
    try { const s = await api('GetSettings'); if (s) { state.settings = Object.assign({}, state.settings, s); store.set('na-settings', state.settings); } } catch (e) {}
    if (route().name !== 'settings') return;
    const s = state.settings, st = state.stats || {};
    const seg = (key, opts, label) => `<div class="seg" role="radiogroup" aria-label="${esc(label)}">${opts.map(([v, l]) => `<button type="button" role="radio" aria-checked="${s[key] === v}" data-set="${key}" data-val="${v}">${l}</button>`).join('')}</div>`;
    const sw = (key, label) => `<span class="switch"><input type="checkbox" role="switch" data-set="${key}" aria-label="${esc(label)}" ${s[key] ? 'checked' : ''}><span></span></span>`;
    const row = (title, hint, control) => `<div class="set-row"><span class="set-row__txt"><b>${title}</b>${hint ? `<span>${hint}</span>` : ''}</span>${control}</div>`;
    const desktop = state.platform === 'desktop';
    main.innerHTML = `<div class="wrap settings">
      <nav aria-label="Settings sections" class="only-wide-nav">
        ${[['appearance', 'Appearance'], ['downloads', 'Downloads'], ['updates', 'Updates'], ['storage', 'Storage'], ['about', 'About']].map(([id, l], i) => `<a href="#/settings" data-jump="${id}" class="${i === 0 ? 'is-on' : ''}">${l}</a>`).join('')}
      </nav>
      <div class="settings__body">
        <div class="stack" style="gap:4px"><a class="back" href="#/">${icon('back', 16)} Library</a><h1 class="page-h" style="font-size:40px">Settings</h1></div>
        <section class="set-group" id="appearance" aria-labelledby="sA"><h2 class="section-h" id="sA">Appearance</h2><div class="card">
          ${row('Theme', 'The reader follows it, unless you pick a reader theme with Aa.', seg('theme', [['light', 'Light'], ['sepia', 'Sepia'], ['dark', 'Dark'], ['system', 'System']], 'Theme'))}
          ${row('Chime when a download finishes', 'A soft two-note sound. Never plays for errors.', sw('chime', 'Chime when a download finishes'))}
          ${row('Notify me when a site needs me', 'A system notification when you’re in another app, so a paused download doesn’t sit unnoticed.', sw('notify', 'System notifications'))}
        </div></section>
        <section class="set-group" id="downloads" aria-labelledby="sD"><h2 class="section-h" id="sD">Downloads</h2><div class="card">
          ${row('Default speed', 'Gentle 2, Balanced 4, Fast 8 at a time. A site never gets more than 3 requests at once.', seg('speed', [['gentle', 'Gentle'], ['balanced', 'Balanced'], ['fast', 'Fast']], 'Default speed'))}
          ${row('Keep chapter images', 'The default for new novels. You can change it each time you add one.', sw('keepImages', 'Keep chapter images'))}
          ${state.platform === 'android' ? row('Only on Wi-Fi', 'Pause on mobile data, carry on when you’re back on Wi-Fi.', sw('wifiOnly', 'Only on Wi-Fi')) : ''}
        </div></section>
        <section class="set-group" id="updates" aria-labelledby="sU"><h2 class="section-h" id="sU">Updates</h2><div class="card">
          ${row('Check saved chapters for revisions', 'Every chapter is the full check, and the slowest: about 10–15 minutes for 2,000 chapters.', seg('revise', [['all', 'Every chapter'], ['recent', 'Last 100'], ['off', 'Off']], 'Revision check'))}
          ${row('Update all novels when the app opens', 'Skips novels updated in the last 12 hours, and completed novels.', sw('updateOnOpen', 'Update all when the app opens'))}
        </div></section>
        <section class="set-group" id="storage" aria-labelledby="sS"><h2 class="section-h" id="sS">Storage</h2><div class="card">
          ${desktop && st.libraryPath ? row('Library folder', esc(st.libraryPath), `<button type="button" class="btn btn--md" data-act="open-library">Open folder</button>`) : ''}
          ${row(`${esc(mb(st.totalBytes || 0))} used`, `${plural(st.totalNovels || 0, 'novel')} · ${plural(st.totalCleanChapters || 0, 'chapter')}. Free up space from each novel’s page.`, '')}
          ${row('Back up library', 'One file with every novel, poster, reading place and these settings. Restore it on a new PC or phone.', `<div style="display:flex;gap:6px;flex-wrap:wrap"><button type="button" class="btn btn--md" data-act="backup">Back up…</button><button type="button" class="btn btn--ghost btn--md" data-act="restore">Restore…</button></div>`)}
        </div></section>
        <section class="set-group" id="about" aria-labelledby="sAb"><h2 class="section-h" id="sAb">About</h2><div class="card storage">
          <div style="display:flex;align-items:center;gap:12px">${LOGO}<span style="display:flex;flex-direction:column"><b>NovelArchiver</b><span class="muted" style="font-size:13px"><button type="button" class="btn btn--link" style="padding:0;font-size:13px" data-act="shortcuts">Keyboard shortcuts</button></span></span></div>
          <div class="sites">${SITES.map((x) => `<span>${esc(x)}</span>`).join('')}</div>
          <p class="muted" style="font-size:13px;line-height:1.6">For keeping a personal offline copy of stories you read. Please don’t redistribute chapters, and support the authors and translators where you can.</p>
        </div></section>
      </div></div>`;
    $$('[data-set]', main).forEach((el) => {
      const key = el.dataset.set;
      if (el.tagName === 'INPUT') el.addEventListener('change', () => saveSettings({ [key]: el.checked }));
      else el.addEventListener('click', () => {
        saveSettings({ [key]: el.dataset.val });
        $$(`[data-set="${key}"]`, main).forEach((b) => b.setAttribute('aria-checked', b === el));
      });
    });
    $$('[data-jump]', main).forEach((a) => a.addEventListener('click', (e) => {
      e.preventDefault();
      const t = $('#' + a.dataset.jump); if (t) t.scrollIntoView({ behavior: 'smooth', block: 'start' });
      $$('[data-jump]', main).forEach((x) => x.classList.toggle('is-on', x === a));
    }));
  }

  // ---------------------------------------------------------------- backup
  let backupDlg = null;
  function backupProgress(title, done, total) {
    const pct = total ? Math.min(100, Math.round((done / total) * 100)) : 0;
    const body = `<div style="display:flex;align-items:center;gap:12px">${spinner(18)}<b style="flex:1">${esc(title)}</b><span class="muted" style="font-size:13px">${esc(mb(done))} of ${esc(mb(total))}</span></div><div class="bar"><i style="--p:${(pct) / 100}"></i></div>`;
    if (backupDlg && document.body.contains(backupDlg)) { backupDlg.innerHTML = body; return; }
    backupDlg = openOverlay(`<div class="dialog" role="dialog" aria-modal="true" aria-label="${esc(title)}" aria-live="polite">${body}</div>`, { onClose: () => { backupDlg = null; } });
  }
  async function backup() {
    let label = 'Backing up your library…';
    const off = (d) => { if (!d.restore) backupProgress(label, d.done, d.total); };
    backupListener = off;
    try {
      const r = await api('BackupLibrary');
      backupListener = null;
      if (backupDlg) closeOverlay();
      if (!r) return;
      toast(`Backup saved · ${r.fileName} · ${r.sizeMb || mb(r.bytes)}`, state.platform === 'desktop' ? { tone: 'ok', action: { label: 'Show in folder', fn: () => api('ShowFileInFolder', r.path).catch(() => {}) } } : { tone: 'ok' });
    } catch (e) { backupListener = null; if (backupDlg) closeOverlay(); toast(`Backup failed: ${e.message}`, { tone: 'bad' }); }
  }
  let backupListener = null;
  async function restore() {
    let info;
    try { info = await api('ChooseBackup'); } catch (e) { toast(e.message, { tone: 'bad' }); return; }
    if (!info) return;
    const made = info.createdAt ? new Date(info.createdAt).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : '—';
    const ok = await confirmDialog({
      title: 'Restore this backup?',
      body: `<span style="display:flex;gap:24px;flex-wrap:wrap;margin:4px 0 10px"><span><span class="muted" style="font-size:12px;display:block">Made</span><b>${esc(made)}</b></span><span><span class="muted" style="font-size:12px;display:block">Novels</span><b>${num(info.novels)}</b></span><span><span class="muted" style="font-size:12px;display:block">Chapters</span><b>${num(info.chapters)}</b></span><span><span class="muted" style="font-size:12px;display:block">Size</span><b>${esc(info.sizeMb || mb(info.bytes))}</b></span></span>${info.newHere ? `${plural(info.newHere, 'novel')} ${info.newHere === 1 ? 'is' : 'are'} new to this device. ` : ''}${info.merged ? `${plural(info.merged, 'novel')} you already have will be merged: missing chapters are added, and whichever reading place is further along is kept.` : ''} Nothing on this device is deleted.`,
      ok: 'Restore',
    });
    if (!ok) return;
    backupListener = (d) => { if (d.restore) backupProgress('Restoring…', d.done, d.total); };
    try {
      const r = await api('RestoreBackup', info.path);
      backupListener = null; if (backupDlg) closeOverlay();
      toast(`Restored · ${plural(r.added || 0, 'novel')} added${r.merged ? `, ${num(r.merged)} merged` : ''}`, { tone: 'ok' });
      loadLibrary();
    } catch (e) { backupListener = null; if (backupDlg) closeOverlay(); toast(`Restore failed: ${e.message}`, { tone: 'bad' }); }
  }

  // ================================================================ PALETTE & SHORTCUTS
  function paletteItems(q) {
    q = q.trim().toLowerCase();
    const items = [];
    const novels = state.novels.filter((n) => !q || `${n.title} ${n.author}`.toLowerCase().includes(q)).slice(0, 6);
    novels.forEach((n) => {
      const r = readInfo(n);
      items.push({ group: 'Novels', title: r.reading ? `Continue ${n.title}` : `Open ${n.title}`, sub: r.reading ? `Chapter ${num(r.last)}` : `${plural(n.totalChapters, 'chapter')}`, thumb: coverHTML(n.cover, n.title, n.author, true),
        run: () => { if (r.reading) location.href = `/library/${encodeURIComponent(n.slug)}/${r.last}`; else location.hash = `#/novel/${encodeURIComponent(n.slug)}`; } });
    });
    const next = THEMES[(THEMES.indexOf(state.settings.theme) + 1) % THEMES.length];
    const actions = [
      { title: 'Add a novel', key: '/', run: () => goAdd() },
      { title: 'Update all novels', run: updateAll },
      { title: `Theme: switch to ${THEME_LABEL[next]}`, key: 'T', run: cycleTheme },
      { title: 'Settings', run: () => { location.hash = '#/settings'; } },
      { title: 'Back up library', run: backup },
      { title: 'Keyboard shortcuts', key: '?', run: shortcuts },
    ];
    actions.filter((a) => !q || a.title.toLowerCase().includes(q)).forEach((a) => items.push(Object.assign({ group: 'Actions' }, a)));
    return items;
  }
  function palette() {
    let sel = 0, items = [];
    const dlg = openOverlay(`<div class="dialog palette" role="dialog" aria-modal="true" aria-label="Search novels and actions">
      <label class="palette__in">${icon('search', 18)}<input type="text" id="palQ" placeholder="Search novels and actions" aria-label="Search novels and actions" autocomplete="off" role="combobox" aria-expanded="true" aria-controls="palList"><kbd>Esc</kbd></label>
      <div class="palette__list" id="palList" role="listbox"></div></div>`, { top: true, instant: true });
    const input = $('#palQ', dlg), list = $('#palList', dlg);
    const paint = () => {
      items = paletteItems(input.value);
      if (sel >= items.length) sel = Math.max(0, items.length - 1);
      let g = '';
      list.innerHTML = items.length ? items.map((it, i) => {
        const head = it.group !== g ? `<span class="palette__group">${it.group}</span>` : '';
        g = it.group;
        return `${head}<button type="button" class="palette__item${i === sel ? ' is-on' : ''}" role="option" aria-selected="${i === sel}" data-i="${i}">${it.thumb ? `<span class="th">${it.thumb}</span>` : ''}<span class="lbl"><b>${esc(it.title)}</b>${it.sub ? `<span>${esc(it.sub)}</span>` : ''}</span>${it.key ? `<kbd>${it.key}</kbd>` : i === sel ? '<kbd>Enter</kbd>' : ''}</button>`;
      }).join('') : `<div class="palette__empty">Nothing matches “${esc(input.value)}”.</div>`;
      $$('[data-i]', list).forEach((b) => {
        b.addEventListener('click', () => { const it = items[+b.dataset.i]; closeOverlay(); it.run(); });
        b.addEventListener('mousemove', () => { if (sel !== +b.dataset.i) { sel = +b.dataset.i; paint(); } });
      });
      const on = $('.is-on', list); if (on) on.scrollIntoView({ block: 'nearest' });
    };
    input.addEventListener('input', () => { sel = 0; paint(); });
    input.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowDown') { e.preventDefault(); sel = Math.min(items.length - 1, sel + 1); paint(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); sel = Math.max(0, sel - 1); paint(); }
      else if (e.key === 'Enter') { e.preventDefault(); const it = items[sel]; if (it) { closeOverlay(); it.run(); } }
    });
    paint();
    input.focus();
  }
  function shortcuts() {
    const rows = (list) => list.map(([l, k]) => `<div><span>${l}</span><kbd>${k}</kbd></div>`).join('');
    openOverlay(`<div class="dialog" role="dialog" aria-modal="true" aria-labelledby="kT" style="width:min(620px,100%)">
      <div style="display:flex;align-items:center;justify-content:space-between"><h2 id="kT">Keyboard shortcuts</h2><button type="button" class="icon-btn icon-btn--sm" data-act="close-overlay" aria-label="Close">${icon('x', 16)}</button></div>
      <div class="keys"><div><h3>Library</h3>${rows([['Search novels and actions', 'Ctrl K'], ['Paste a link', '/'], ['Switch theme', 'T'], ['Show shortcuts', '?'], ['Close or clear', 'Esc']])}</div>
      <div><h3>Reader</h3>${rows([['Previous / next chapter', '← →'], ['Text size', '− +'], ['Serif or sans', 'F'], ['Novel page', 'H'], ['Reader theme', 'Aa']])}</div></div></div>`);
  }
  function goAdd() {
    if (route().name !== 'library') { location.hash = '#/'; setTimeout(focusUrl, 50); } else focusUrl();
    const i = $('#novelUrl'); if (i) i.select();
  }

  // ================================================================ GLOBAL EVENTS
  document.addEventListener('click', async (e) => {
    const t = e.target.closest('[data-act]');
    if (!t) return;
    const act = t.dataset.act;
    const slug = t.dataset.slug;
    switch (act) {
      case 'paste':
        try { const v = await navigator.clipboard.readText(); if (v) { state.intake.url = v.trim(); lookup(v); } else focusUrl(); } catch (er) { focusUrl(); toast(state.platform === 'android' ? 'Long-press the field and choose Paste.' : 'Press Ctrl+V to paste the link.'); }
        break;
      case 'intake-clear': case 'intake-cancel': e.preventDefault(); cancelIntake(); break;
      case 'intake-start': startIntake(); break;
      case 'intake-open': setTimeout(cancelIntake, 0); break;
      case 'intake-update': { const s = state.intake.data && state.intake.data.existingSlug; cancelIntake(); if (s) updateNovel(s); break; }
      case 'site-window': openSiteWindow(t.dataset.url || (state.intake.data && state.intake.data.url) || ''); break;
      case 'stop': api('CancelTask').catch((er) => toast(er.message, { tone: 'bad' })); break;
      case 'stop-all': api('CancelAll').then(loadActivity).catch((er) => toast(er.message, { tone: 'bad' })); break;
      case 'unqueue': api('RemoveQueued', +t.dataset.id).then(loadActivity).catch((er) => toast(er.message, { tone: 'bad' })); break;
      case 'toggle-log': state.showLog = !state.showLog; renderJobsEverywhere(); break;
      case 'toggle-queue': state.showQueue = !state.showQueue; renderJobsEverywhere(); break;
      case 'dismiss': dismissResult(t.dataset.rid); break;
      case 'resume': {
        const r = state.results.find((x) => x.id === t.dataset.rid);
        if (!r) break;
        dismissResult(r.id);
        const d = r.data;
        if (d.kind === 'update' && d.slug) updateNovel(d.slug);
        else api('StartScrape', { url: d.url, startChapter: 0, endChapter: 0, concurrency: speedWorkers[state.settings.speed] || 4, skipImages: !state.settings.keepImages, fromPasted: false, force: false })
          .then(loadActivity).catch((er) => toast(er.message, { tone: 'bad' }));
        break;
      }
      case 'update': updateNovel(slug); break;
      case 'update-all': updateAll(); break;
      case 'epub': exportEpub(slug); break;
      case 'book-menu': e.preventDefault(); bookMenu(t, slug); break;
      case 'novel-menu': novelMenu(t); break;
      case 'open-source': e.preventDefault(); if (state.novel && state.novel.url) openExternal(state.novel.url); break;
      case 'synopsis': state.synopsisOpen = !state.synopsisOpen; $('#synopsis').classList.toggle('is-open', state.synopsisOpen); t.textContent = state.synopsisOpen ? 'Less' : 'More'; break;
      case 'jump': { state.tocFilter = 'all'; state.tocQuery = ''; const q = $('#tocQ'); if (q) q.value = ''; $$('[data-toc]').forEach((x) => x.setAttribute('aria-pressed', x.dataset.toc === 'all')); paintToc(); const row = $('#toc .is-current'); if (row) row.scrollIntoView({ behavior: 'smooth', block: 'center' }); break; }
      case 'toc-all': case 'toc-more': state.tocAll = true; paintToc(); break;
      case 'del-images': deleteImages(); break;
      case 'del-range': deleteRange(); break;
      case 'open-library': api('OpenLibraryFolder').catch(() => {}); break;
      case 'backup': backup(); break;
      case 'restore': restore(); break;
      case 'shortcuts': shortcuts(); break;
      case 'close-overlay': closeOverlay(); break;
      case 'focus-url': goAdd(); break;
      case 'clear-filters': state.filter = 'all'; state.status = 'any'; state.query = ''; $('#librarySearch').value = ''; store.set('na-filter', 'all'); renderShelf(); break;
    }
  });
  // Links into the reader must navigate in place (Wails & Android).
  document.addEventListener('click', (e) => {
    const a = e.target.closest('a[href]');
    if (!a || a.target === '_blank') return;
    const href = a.getAttribute('href');
    if (a.getAttribute('aria-disabled') === 'true') { e.preventDefault(); return; }
    if (href.startsWith('/library/')) { e.preventDefault(); location.href = href; }
  });

  // Header search filters the shelf.
  const search = $('#librarySearch');
  search.addEventListener('input', () => {
    state.query = search.value;
    if (route().name !== 'library') location.hash = '#/';
    else renderShelf();
  });
  search.addEventListener('keydown', (e) => { if (e.key === 'Escape') { search.value = ''; state.query = ''; renderShelf(); search.blur(); $('#searchBox').classList.remove('is-open'); } });
  search.addEventListener('blur', () => { if (!search.value) $('#searchBox').classList.remove('is-open'); });
  const searchToggle = $('#searchToggle');
  const mq = window.matchMedia('(max-width: 640px)');
  const syncSearch = () => { searchToggle.hidden = !mq.matches; };
  (mq.addEventListener ? mq.addEventListener('change', syncSearch) : mq.addListener(syncSearch));
  syncSearch();
  searchToggle.addEventListener('click', () => { $('#searchBox').classList.add('is-open'); search.focus(); });
  $('#themeBtn').addEventListener('click', cycleTheme);

  document.addEventListener('keydown', (e) => {
    const typing = ['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement && document.activeElement.tagName);
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); if (overlayClose) closeOverlay(); else palette(); return; }
    if (e.key === 'Escape') {
      if (menuState) { closeMenu(); return; }
      if (overlayClose) { closeOverlay(); return; }
      if (state.intake.phase !== 'idle' && route().name === 'library') { cancelIntake(); return; }
      return;
    }
    if (typing || e.ctrlKey || e.metaKey || e.altKey || overlayClose) return;
    if (e.key === '/') { e.preventDefault(); goAdd(); }
    else if (e.key === '?') { e.preventDefault(); shortcuts(); }
    else if (e.key === 't' || e.key === 'T') { e.preventDefault(); cycleTheme(); }
  });

  // ================================================================ BACKEND EVENTS
  function logLine(m) { const ts = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }); state.log.push(`${ts}  ${m}`); if (state.log.length > 400) state.log.splice(0, 100); }
  on('queue', (a) => {
    const prevSlug = state.activity.current && state.activity.current.slug;
    state.activity = { current: a.current || null, queue: a.queue || [], waiting: a.waiting || '' };
    scheduleJobs();
    if (prevSlug && !(a.current && a.current.slug === prevSlug)) setTimeout(loadLibrary, 200);
  });
  on('started', (d) => {
    const c = state.activity.current;
    if (c) { c.phase = 'downloading'; if (d.slug) c.slug = d.slug; if (d.title) c.title = d.title; }
    logLine(`Started: ${d.title || ''}`);
    scheduleJobs();
    if (d.slug && !novelBySlug(d.slug)) loadLibrary();
  });
  on('status', (d) => {
    const c = state.activity.current;
    if (c) c.message = d.message;
    state.lastStatusAt = Date.now();
    if (d.message) logLine(d.message);
    scheduleJobs();
  });
  on('progress', (p) => {
    const c = state.activity.current;
    state.lastProgressAt = Date.now();
    if (c) { c.progress = p; c.challenge = ''; c.phase = p.stage === 'revising' ? 'revising' : 'downloading'; if (p.slug) c.slug = p.slug; if (p.title) c.title = p.title; }
    if (p.chapterNum && p.stage !== 'revising') logLine(`Ch. ${p.chapterNum} ${p.status === 'err' ? 'failed' : 'saved'}${p.chapterTitle ? ' — ' + p.chapterTitle : ''}`);
    scheduleJobs();
  });
  on('challenge', (d) => {
    const c = state.activity.current;
    const site = siteUrl(d.host);
    const name = hostOf(site) || 'The site';
    if (c) c.challenge = name;
    logLine(`Needs you: ${name}`);
    scheduleJobs();
    notify(`${name} needs you`, `Tick the check in the site window to keep downloading${c && c.title ? ' ' + c.title : ''}.`);
    toast(`${name} needs you to continue`, { tone: 'warn', action: { label: 'Open site window', fn: () => openSiteWindow(site) }, timeout: 12000 });
  });
  function siteUrl(h) { h = String(h || '').trim(); if (!h) return ''; return /^https?:\/\//i.test(h) ? h : 'https://' + h; }
  on('done', (d) => {
    logLine(`Finished: ${d.title || ''}`);
    addResult('done', d);
    scheduleJobs();
    loadLibrary();
    if (d.kind === 'update') {
      const nw = (d.newChapters || []).length;
      if (!d.auto || nw) notify(nw ? `${d.title}: ${plural(nw, 'new chapter')}` : `${d.title} is up to date`, nw ? 'Ready to read offline.' : 'Nothing changed on the site.');
      if (nw) chime();
    } else {
      chime();
      notify(`${d.title} is ready`, `${plural(d.ok || 0, 'chapter')} saved for offline reading.`);
      if (route().name !== 'library' || document.hidden) toast(`${d.title} is ready`, { tone: 'ok', action: { label: 'Start reading', fn: () => { location.href = `/library/${encodeURIComponent(d.slug)}/`; } } });
    }
  });
  on('stopped', (d) => { logLine(d.message || 'Stopped'); addResult('stopped', d); scheduleJobs(); loadLibrary(); });
  on('error', (d) => {
    logLine(`Error: ${d.message || ''}`);
    if (d.slug || d.url || d.title) addResult('error', d);
    else toast(cap(d.message) || 'Something went wrong', { tone: 'bad' });
    scheduleJobs();
    notify(`${d.title || 'Download'} stopped`, d.message || '');
    loadLibrary();
  });
  on('backup', (d) => { if (backupListener) backupListener(d); });

  // ================================================================ START
  async function start() {
    applyTheme(state.settings.theme);
    render();
    try { const p = await api('Platform'); if (p) state.platform = p; } catch (e) {}
    document.documentElement.dataset.platform = state.platform;
    await Promise.all([loadActivity(), loadLibrary()]);
    render();
  }
  window.addEventListener('focus', () => { if (route().name === 'library') loadLibrary(); });
  start();
})();
