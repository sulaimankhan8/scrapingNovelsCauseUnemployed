/* InkShelfs — manga, manhwa and manhua (desktop). Loaded before app.js;
   app.js calls Manga.init(window.NA) and then routes #/manga/<slug>,
   #/read/<slug>/<key> and #/sites here. Talks to Go through
   window.go.main.MangaApp; its events arrive as "manga:<name>".
   On Android there is no MangaApp, so enabled() stays false and the
   shell behaves exactly as before. */
(function () {
  'use strict';

  let NA = null;
  let ready = false;
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const esc = (v) => String(v == null ? '' : v).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const debounce = (fn, ms) => { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; };
  const enc = encodeURIComponent;
  const onPhone = () => NA && NA.state && NA.state.platform === 'android';

  const M = {
    lib: [], activity: { current: null, queue: [] }, settings: null,
    series: null, seriesSlug: '', chFilter: 'all', chQuery: '', chShowAll: false,
    chOrder: 'asc', select: null, reader: null, addSteps: null, sitesTab: 'novel',
  };

  // ---------------------------------------------------------------- backend
  const mg = () => window.go.main.MangaApp;
  function friendly(e) {
    let m = String((e && (e.message || e)) || 'Something went wrong').replace(/^Error:\s*/, '');
    return m.charAt(0).toUpperCase() + m.slice(1);
  }
  async function call(name, ...args) {
    try { return await mg()[name](...args); } catch (e) { throw new Error(friendly(e)); }
  }

  async function boot() {
    for (let i = 0; i < 200 && !(window.go && window.go.main && window.go.main.App); i++) await sleep(30);
    if (!(window.go && window.go.main && window.go.main.MangaApp)) return;
    try { ready = !!(await mg().Available()); } catch (e) { ready = false; }
    if (!ready) return;
    M.chOrder = NA.store.get('ma-chorder', 'asc');
    M.sitesTab = NA.store.get('ma-sitestab', 'novel');
    wireEvents();
    await Promise.all([loadLib(), loadActivity(), loadSettings()]);
    // Repaint the page with manga turned on, without disturbing a link the
    // user is already typing.
    const r = NA.route();
    const u = $('#novelUrl');
    if (r.name === 'library' && u && (u.value || document.activeElement === u)) NA.renderShelf();
    else NA.render();
  }
  async function loadLib() {
    try { M.lib = (await call('Library')) || []; } catch (e) { M.lib = []; }
  }
  async function loadActivity() {
    try { M.activity = (await call('Activity')) || { current: null, queue: [] }; } catch (e) {}
    paintJobs();
  }
  async function loadSettings() {
    try { M.settings = await call('GetSettings'); } catch (e) {}
    try { M.libFolder = await call('LibraryFolder'); } catch (e) {}
  }

  // ---------------------------------------------------------------- helpers
  const kindOf = (type) => {
    const t = String(type || '').split(/[,(]/)[0].trim();
    return /^(manga|manhwa|manhua)$/i.test(t) ? t.charAt(0).toUpperCase() + t.slice(1).toLowerCase() : 'Manga';
  };
  const sizeOf = (b) => NA.mb(b);
  // Several series download at once (one per site): every running job.
  const runningJobs = () => (M.activity.running && M.activity.running.length ? M.activity.running : (M.activity.current ? [M.activity.current] : []));
  const jobFor = (slug) => runningJobs().find((j) => j.slug === slug) || null;
  const busyWith = (slug) => !!jobFor(slug);
  // Manga, Manhwa or Manhua for a series in the library.
  const kindOfSlug = (slug) => { const it = (M.lib || []).find((x) => x.slug === slug); return kindOf(it && it.type); };
  const queuedFor = (slug) => (M.activity.queue || []).some((j) => j.slug === slug);
  const seriesHref = (slug) => `#/manga/${enc(slug)}`;
  const readHref = (slug, key) => `#/read/${enc(slug)}/${key}`;

  // Manga series shaped like novels, so the shell's filters, sorting and
  // search treat both kinds the same way.
  function normalize(it) {
    return {
      _manga: true, slug: it.slug, title: it.title, author: it.siteName, site: it.siteName, cover: it.cover,
      type: kindOf(it.type), totalChapters: it.total, lastRead: it.readNumber || 0, readKey: it.readKey, firstKey: it.firstKey,
      lastReadAt: it.readAt || '', addedAt: it.addedAt, updatedAt: it.readAt || it.addedAt,
      failedCount: it.failed, lastNew: it.newChapters || 0, lastNewAt: it.newChapters ? new Date().toISOString() : '',
      status: it.status, sizeBytes: it.bytes, downloaded: it.downloaded, url: it.url,
    };
  }

  function badge(n) {
    const cur = jobFor(n.slug);
    if (cur) {
      if (cur.kind === 'update') return `<span class="book__badge">${NA.spinner(11, '#E0794F')} Checking</span>`;
      return `<span class="book__badge"><span class="dot"></span>Saving ${cur.total ? `${cur.done}/${cur.total}` : ''}</span>`;
    }
    if (queuedFor(n.slug)) return '<span class="book__badge"><span class="dot" style="background:#B5AC9E"></span>Queued</span>';
    if (n.failedCount) return `<span class="book__badge book__badge--red">${NA.icon('alert', 12, 2.4)} ${NA.num(n.failedCount)} failed</span>`;
    if (n.lastNew) return `<span class="book__badge book__badge--green"><span class="dot" style="background:#2F6B3F"></span>${NA.plural(n.lastNew, 'new chapter')}</span>`;
    return '';
  }

  function card(n, i) {
    const title = n.title;
    const reading = n.readKey && n.lastRead;
    const cont = reading ? readHref(n.slug, n.readKey) : n.firstKey ? readHref(n.slug, n.firstKey) : '';
    const contLabel = reading ? `Continue · Ch. ${NA.num(n.lastRead)}` : 'Start reading';
    const pct = n.totalChapters ? Math.min(100, Math.round((n.lastRead / n.totalChapters) * 100)) : 0;
    const meta = reading ? `Ch. ${NA.num(n.lastRead)} of ${NA.num(n.totalChapters)}` : `${NA.num(n.downloaded)} of ${NA.num(n.totalChapters)} saved`;
    const cover = NA.coverHTML(n.cover, title, '', false, i < 12).replace('class="cover', 'class="cover cover--manga');
    return `<article class="book book--manga" data-mslug="${esc(n.slug)}" data-key="m-${esc(n.slug)}">
      <div class="book__art">
        <a class="book__cover" href="${seriesHref(n.slug)}" aria-label="Open ${esc(title)}">${cover}<span class="kind-tag">${esc(n.type)}</span><span class="book__badge-slot">${badge(n)}</span></a>
        ${cont ? `<div class="book__hover" aria-hidden="true"><a href="${cont}" tabindex="-1">${NA.icon('panels', 14)} ${contLabel}</a></div>` : ''}
        <button type="button" class="book__more" data-mact="card-menu" data-slug="${esc(n.slug)}" aria-label="More actions for ${esc(title)}" aria-haspopup="menu" aria-expanded="false">${NA.icon('more', 16, 2.4)}</button>
      </div>
      <div class="book__info"><a class="book__title" href="${seriesHref(n.slug)}">${esc(title)}</a><span class="book__author">${esc(n.type)} · ${esc(n.site)}</span></div>
      <div class="book__progress"><div class="bar bar--thin${reading ? '' : ' bar--none'}" aria-hidden="true"><i style="--p:${pct / 100}"></i></div><span class="book__meta">${esc(meta)}</span></div>
    </article>`;
  }

  function seriesMenu(anchor, slug) {
    const it = M.lib.find((x) => x.slug === slug);
    if (!it) return;
    const items = [{ label: 'Open series page', fn: () => { location.hash = seriesHref(slug); } }];
    if (it.readKey) items.push({ label: 'Continue reading', fn: () => { location.hash = readHref(slug, it.readKey); } });
    items.push({ label: 'Check for new chapters', fn: () => act(() => call('CheckUpdates', slug), 'Checking for new chapters…') });
    items.push({ label: 'Save as CBZ file…', sub: 'One file for comic readers', fn: () => exportCBZ(slug) });
    if (!onPhone()) items.push({ label: 'Show folder', fn: () => call('OpenFolder', slug) });
    items.push('sep');
    items.push({ label: 'Remove from library…', danger: true, fn: () => removeSeries(slug, it.title) });
    NA.openMenu(anchor, items);
  }

  async function act(fn, okMsg) {
    try { await fn(); if (okMsg) NA.toast(okMsg); } catch (e) { NA.toast(e.message, { tone: 'bad' }); }
  }
  async function exportCBZ(slug) {
    try {
      const r = await call('ExportCBZ', slug);
      if (r && r.uri) NA.savedFileToast(`Saved ${NA.plural(r.chapters, 'chapter')} as CBZ${r.path ? ' in ' + r.path : ''}`, r);
      else if (r) NA.toast(`Saved ${NA.plural(r.chapters, 'chapter')} (${sizeOf(r.bytes)}) as CBZ`, { tone: 'ok', action: { label: 'Show file', fn: () => NA.api('ShowFileInFolder', r.path).catch(() => {}) } });
    } catch (e) { NA.toast(e.message, { tone: 'bad' }); }
  }
  async function removeSeries(slug, title) {
    const it = M.lib.find((x) => x.slug === slug) || {};
    const ok = await NA.confirmDialog({ title: `Remove ${title}?`, body: `This deletes ${NA.plural(it.downloaded || 0, 'saved chapter')} (${sizeOf(it.bytes || 0)}) from this device. It can’t be undone.`, ok: 'Remove', danger: true });
    if (!ok) return;
    try {
      await call('DeleteSeries', slug);
      M.lib = M.lib.filter((x) => x.slug !== slug);
      if (NA.route().name !== 'library') location.hash = '#/'; else NA.renderShelf();
      NA.toast(`Removed ${title}`);
    } catch (e) { NA.toast(e.message, { tone: 'bad' }); }
  }

  // ---------------------------------------------------------------- add
  async function openAdd(url) {
    M.addSteps = ['Site recognised'];
    const dlg = NA.openOverlay(`<div class="dialog mg-add" role="dialog" aria-modal="true" aria-labelledby="mgAddT">
      <div class="mg-add__head"><h2 id="mgAddT">Add manga</h2><button type="button" class="icon-btn icon-btn--sm" data-mact="close" aria-label="Close">${NA.icon('x', 16)}</button></div>
      <div class="mg-link"><span class="mg-link__u">${esc(url)}</span><span class="mg-link__bar"><i></i></span></div>
      <div id="mgAddBody" aria-live="polite"></div></div>`, { label: 'Add manga', onClose: () => { M.addSteps = null; } });
    const body = $('#mgAddBody', dlg);
    const paintSteps = () => {
      if (!M.addSteps || !body.isConnected || body.dataset.done) return;
      body.innerHTML = `<div class="mg-add__wait">
        <div class="mg-sk mg-sk--cover"></div>
        <div style="flex:1;display:flex;flex-direction:column;gap:10px;min-width:0">
          <div class="mg-sk" style="height:20px;width:70%"></div><div class="mg-sk" style="height:12px;width:40%"></div>
          <ol class="mg-steps">${M.addSteps.map((s, i) => `<li class="${i === M.addSteps.length - 1 ? 'is-now' : 'is-done'}">${i === M.addSteps.length - 1 ? NA.spinner(14) : NA.icon('check', 14, 2.6)}<span>${esc(s)}</span></li>`).join('')}</ol>
        </div></div>`;
    };
    M.paintSteps = paintSteps;
    paintSteps();
    let pv;
    try { pv = await call('Preview', url); } catch (e) {
      if (!body.isConnected) return;
      body.dataset.done = '1';
      $('.mg-link', dlg).classList.add('is-idle');
      body.innerHTML = `<div class="err-box" role="alert"><b>Couldn’t read that link.</b><span>${esc(e.message)}</span></div>
        <div class="dialog__foot"><a class="btn" href="#/sites" data-mact="close">Supported sites</a><button type="button" class="btn btn--primary" data-mact="retry-add">Try again</button></div>`;
      $('[data-mact="retry-add"]', body).onclick = () => openAdd(url);
      return;
    }
    if (!body.isConnected) return;
    body.dataset.done = '1';
    M.addSteps = null;
    $('.mg-link', dlg).classList.add('is-idle');
    const n = pv.chapters.length;
    const start = pv.inputChapter || 0;
    const startTitle = start ? pv.chapters[start - 1].title : '';
    const warn = (pv.warnings || []).map((w) => `<div class="note note--amber">${esc(w)}</div>`).join('');
    const existing = pv.existingSlug ? `<div class="note note--green">Already in your library with ${NA.plural(pv.downloaded, 'chapter')} saved. Downloading adds to it.</div>` : '';
    body.innerHTML = `<div class="mg-found">
      <div class="mg-found__cover" id="mgPvCover">${NA.coverHTML('', pv.title, kindOf(pv.type), false, true)}</div>
      <div class="mg-found__txt">
        <div class="mg-chips"><span class="mg-kind">${esc(kindOf(pv.type))}</span><span class="pill">${esc(pv.siteName)}</span><span class="pill pill--green">${NA.plural(n, 'chapter')}</span></div>
        <h3 class="mg-found__title">${esc(pv.title)}</h3>
        ${pv.author || pv.status ? `<p class="muted">${[pv.author, pv.status].filter(Boolean).map(esc).join(' · ')}</p>` : ''}
        ${pv.description ? `<p class="mg-desc mg-clamp">${esc(pv.description)}</p>` : ''}
      </div></div>
      ${existing}${warn}
      <fieldset class="mg-range"><legend>Which chapters?</legend>
        ${start ? `<label class="mg-opt"><input type="radio" name="mrng" value="from" checked> From ${esc(startTitle)} (the one you pasted) to the latest <span class="muted">· ${NA.plural(n - start + 1, 'chapter')}</span></label>` : ''}
        <label class="mg-opt"><input type="radio" name="mrng" value="all" ${start ? '' : 'checked'}> All ${NA.plural(n, 'chapter')}</label>
        <label class="mg-opt"><input type="radio" name="mrng" value="first"> The first <input type="number" id="mFirst" min="1" max="${n}" value="${Math.min(5, n)}" aria-label="How many chapters"> chapters</label>
        <label class="mg-opt"><input type="radio" name="mrng" value="range"> Chapters <input type="number" id="mFrom" min="1" max="${n}" value="1" aria-label="From chapter"> to <input type="number" id="mTo" min="1" max="${n}" value="${n}" aria-label="To chapter"></label>
      </fieldset>
      <p class="mg-tip muted">Pages are saved to this device as images. A chapter is usually 5–30 MB, so a long series can take several GB.</p>
      <div class="dialog__foot"><button type="button" class="btn" data-mact="close">Cancel</button><button type="button" class="btn btn--primary" id="mgGo">Download</button></div>`;
    $$('.mg-opt input[type=number]', body).forEach((inp) => inp.addEventListener('focus', () => { inp.closest('.mg-opt').querySelector('input[type=radio]').checked = true; }));
    if (pv.cover) call('CoverPreview', url).then((src) => { const b = $('#mgPvCover', body); if (src && b) b.innerHTML = `<div class="cover cover--manga"><img src="${src}" alt=""></div>`; }).catch(() => {});
    $('#mgGo', body).onclick = async (ev) => {
      const v = $('input[name=mrng]:checked', body).value;
      const num = (id) => Math.max(1, Math.min(n, parseInt($(id, body).value, 10) || 1));
      let from = 0, to = 0;
      if (v === 'from') from = start;
      if (v === 'first') { from = 1; to = num('#mFirst'); }
      if (v === 'range') { from = num('#mFrom'); to = Math.max(from, num('#mTo')); }
      ev.target.disabled = true;
      ev.target.innerHTML = `${NA.spinner(14, '#fff')} Adding…`;
      try {
        const slug = await call('Add', url, from, to);
        NA.closeOverlay();
        await loadLib();
        location.hash = seriesHref(slug);
        NA.toast(`Downloading ${pv.title}. You can start reading as soon as the first chapter is saved.`, { tone: 'ok' });
      } catch (e) {
        ev.target.disabled = false;
        ev.target.textContent = 'Download';
        NA.toast(e.message, { tone: 'bad' });
      }
    };
    const go = $('#mgGo', body); if (go) go.focus();
  }

  // ---------------------------------------------------------------- jobs
  function jobLine(j, live) {
    if (!j) return '';
    const pct = j.total ? Math.round((j.done / j.total) * 100) : 0;
    const what = j.kind === 'update' ? 'Checking for new chapters' : (j.message || 'Waiting…');
    return `<div class="mg-job${live ? ' is-live' : ''}" data-key="mj-${j.id}">
      <div class="mg-job__head"><a href="${seriesHref(j.slug)}" class="mg-job__t">${esc(j.title)}</a><span class="mg-kind mg-kind--sm">${esc(kindOfSlug(j.slug))}</span>
        ${live ? `<button type="button" class="btn btn--sm" data-mact="stop" data-id="${j.id}">Stop</button>` : `<button type="button" class="btn btn--ghost btn--sm" data-mact="unqueue" data-id="${j.id}">Remove</button>`}</div>
      <span class="mg-job__msg">${esc(what)}${j.total ? ` · ${j.done} of ${j.total}` : ''}</span>
      ${live ? `<div class="bar${j.total ? '' : ' bar--ind'}"><i style="--p:${pct / 100}"></i></div>` : ''}
    </div>`;
  }
  function paintJobs() {
    if (!ready) return;
    const a = M.activity;
    const run = runningJobs(), q = a.queue || [];
    const busy = !!(run.length || q.length);
    const pill = $('#mjobsPill');
    if (pill) {
      pill.hidden = !busy;
      document.documentElement.classList.toggle('has-mjobs', busy);
      if (busy) {
        // One job: its title and progress. Several: how many, together.
        let txt, short;
        if (run.length === 1 && !q.length) {
          const c = run[0];
          txt = c.kind === 'update' ? `Checking ${c.title}` : `${c.title}${c.total ? ` · ${c.done}/${c.total}` : ''}`;
          short = c.total ? `${c.done}/${c.total}` : '1';
        } else {
          txt = `${run.length ? `${run.length} downloading` : 'Waiting'}${q.length ? ` · ${q.length} waiting` : ''}`;
          short = String(run.length + q.length);
        }
        NA.morph(pill, `${NA.spinner(14)}<span class="mjobs-pill__t">${esc(txt)}</span><span class="mjobs-pill__s" aria-hidden="true">${esc(short)}</span>`);
        pill.setAttribute('aria-label', `Manga downloads: ${txt}`);
      }
    }
    const box = $('#mjobs');
    if (box) {
      box.hidden = !busy;
      NA.morph(box, busy ? `${run.map((j) => jobLine(j, true)).join('')}${q.length ? `<p class="mg-job__more muted">${NA.plural(q.length, 'more manga job')} waiting</p>` : ''}` : '');
    }
    const sb = $('#mgBusyBtn');
    if (sb && M.series) {
      const j = jobFor(M.series.slug);
      if (j) { const l = $('.mg-busy__t', sb), t = j.total ? `Stop · ${j.done}/${j.total}` : 'Stop'; if (l && l.textContent !== t) l.textContent = t; }
    }
    const d = $('#mgJobsDlg');
    if (d) NA.morph($('.mg-jobs', d), jobsList());
  }
  function jobsList() {
    const run = runningJobs(), q = M.activity.queue || [];
    if (!run.length && !q.length) return '<p class="muted">Nothing is downloading.</p>';
    return `${run.map((j) => jobLine(j, true)).join('')}${q.map((j) => jobLine(j, false)).join('')}`;
  }
  function openJobs() {
    const dlg = NA.openOverlay(`<div class="dialog" role="dialog" aria-modal="true" aria-labelledby="mgJT" id="mgJobsDlg" style="width:min(560px,100%)">
      <h2 id="mgJT">Manga downloads</h2><div class="mg-jobs">${jobsList()}</div>
      <div class="dialog__foot"><button type="button" class="btn btn--ghost" data-mact="stop-all">Stop all</button><button type="button" class="btn btn--primary" data-mact="close">Close</button></div></div>`);
    return dlg;
  }

  // ---------------------------------------------------------------- series page
  let seriesTimer = null;
  async function renderSeries(main, slug, quiet) {
    // Back on a kept page for this series: refresh it in place.
    const keptSeries = $('#mgSeries', main);
    if (!quiet && keptSeries && keptSeries.dataset.slug === slug) quiet = true;
    if (!quiet) {
      M.select = null; M.chShowAll = false; M.chFilter = 'all'; M.chQuery = '';
      main.innerHTML = `<div class="mg-hero mg-hero--sk"><div class="wrap"><div class="mg-sk" style="width:180px;aspect-ratio:2/3;border-radius:8px"></div></div></div>`;
    }
    let d;
    try { d = await call('Series', slug); } catch (e) {
      if (NA.route().name !== 'manga') return;
      main.innerHTML = `<div class="wrap stack" style="gap:20px;padding-top:28px"><a class="back" href="#/">${NA.icon('back', 16)} Library</a><div class="empty"><h3>This series isn’t in your library</h3><p>${esc(e.message)}</p><a class="btn" href="#/">Back to library</a></div></div>`;
      return;
    }
    const r = NA.route();
    if (r.name !== 'manga' || r.slug !== slug) return;
    M.series = d; M.seriesSlug = slug;
    paintSeries(main);
  }
  function seriesStats(d) {
    const ch = d.chapters || [];
    const live = ch.filter((c) => !c.removed);
    return {
      ch, live,
      saved: ch.filter((c) => c.state === 'ok').length,
      failed: ch.filter((c) => c.state === 'failed').length,
      pending: live.filter((c) => c.state !== 'ok' && c.state !== 'failed'),
      bytes: ch.reduce((a, c) => a + (c.bytes || 0), 0),
      reading: ch.find((c) => c.key === d.readKey),
      firstSaved: ch.find((c) => c.state === 'ok'),
    };
  }
  function paintSeries(main) {
    const d = M.series;
    const slug = d.slug;
    const st = seriesStats(d);
    const kind = kindOf(d.type);
    const busy = busyWith(slug);
    let primary;
    if (st.reading) primary = `<a class="btn btn--lg mg-hero__go" href="${readHref(slug, st.reading.key)}">${NA.icon('panels', 16)} Continue · Ch. ${NA.num(st.reading.number)}</a>`;
    else if (st.firstSaved) primary = `<a class="btn btn--lg mg-hero__go" href="${readHref(slug, st.firstSaved.key)}">${NA.icon('panels', 16)} Start reading</a>`;
    else primary = `<button type="button" class="btn btn--lg mg-hero__go" disabled>${busy ? `${NA.spinner(16)} Saving the first chapter…` : 'Nothing saved yet'}</button>`;
    const next10 = busy ? [] : st.pending.slice(0, 10).map((c) => c.key);
    const job = jobFor(slug);
    const hero = `<div class="mg-hero" style="--tone:${NA.toneOf(d.title)}"><div class="wrap mg-hero__in">
      <a class="mg-back" href="#/">${NA.icon('back', 16)} Library</a>
      <div class="mg-hero__row">
        <div class="mg-hero__cover">${NA.coverHTML(d.cover, d.title, kind, false, true).replace('class="cover', 'class="cover cover--manga')}</div>
        <div class="mg-hero__txt">
          <div class="mg-chips"><span class="mg-kind mg-kind--light">${esc(kind)}</span><span class="mg-chip-l">${esc(d.siteName)}</span>${d.status ? `<span class="mg-chip-l">${esc(d.status)}</span>` : ''}</div>
          <h1 class="mg-hero__title">${esc(d.title)}</h1>
          ${d.author ? `<p class="mg-hero__by">${esc(d.author)}</p>` : ''}
          ${d.description ? `<p class="mg-hero__desc mg-clamp" id="mgDesc">${esc(d.description)}</p>` : ''}
          <div class="mg-hero__acts">
            ${primary}
            ${job ? `<button type="button" class="btn btn--lg mg-btn-o" id="mgBusyBtn" data-mact="stop" data-id="${job.id}" aria-label="Stop downloading ${esc(d.title)}">${NA.spinner(15)}<span class="mg-busy__t">${job.total ? `Stop · ${job.done}/${job.total}` : 'Stop'}</span></button>` : ''}
            ${next10.length ? `<button type="button" class="btn btn--lg mg-btn-o" data-mact="dl-next" data-keys="${next10.join(',')}">Download next ${next10.length}</button>` : ''}
            ${!busy && st.pending.length > 10 ? `<button type="button" class="btn btn--lg mg-btn-o" data-mact="dl-all">All ${NA.num(st.pending.length)} remaining</button>` : ''}
            ${st.failed ? `<button type="button" class="btn btn--lg mg-btn-o" data-mact="retry">Retry ${NA.num(st.failed)} failed</button>` : ''}
            <button type="button" class="btn btn--lg mg-btn-o" data-mact="check">${NA.icon('refresh', 15)} Check for new</button>
            <button type="button" class="icon-btn mg-btn-o" data-mact="series-menu" aria-label="More actions" aria-haspopup="menu" aria-expanded="false">${NA.icon('more', 18, 2.4)}</button>
          </div>
        </div>
        <dl class="mg-stats">
          <div><dt>Chapters</dt><dd>${NA.num(st.live.length)}</dd></div>
          <div><dt>Saved</dt><dd>${NA.num(st.saved)}</dd></div>
          <div><dt>On disk</dt><dd>${sizeOf(st.bytes)}</dd></div>
        </dl>
      </div></div></div>`;
    const sel = M.select;
    const tools = `<div class="mg-tools">
      <h2 class="section-h" style="margin:0">Chapters</h2>
      <label class="field mg-find">${NA.icon('search', 15)}<input type="search" id="mgFind" placeholder="Number or title" aria-label="Find a chapter" value="${esc(M.chQuery)}"></label>
      <div class="seg" role="group" aria-label="Filter chapters">${[['all', 'All'], ['ok', 'Saved'], ['todo', 'Not saved'], ['unread', 'Unread']].concat(st.failed ? [['failed', `Failed · ${st.failed}`]] : []).map(([k, l]) => `<button type="button" data-mfilter="${k}" aria-pressed="${M.chFilter === k}">${l}</button>`).join('')}</div>
      <div class="seg" role="group" aria-label="Order"><button type="button" data-morder="asc" aria-pressed="${M.chOrder === 'asc'}">Oldest first</button><button type="button" data-morder="desc" aria-pressed="${M.chOrder === 'desc'}">Newest first</button></div>
      <button type="button" class="btn btn--md" data-mact="select" aria-pressed="${!!sel}">${sel ? 'Done' : 'Select'}</button>
    </div>`;
    const selBar = sel ? `<div class="mg-selbar" role="region" aria-label="Selection"><b>${NA.plural(sel.size, 'chapter')} selected</b>
      <button type="button" class="btn btn--sm btn--ghost" data-mact="sel-all">Select all shown</button>
      <span style="flex:1"></span>
      <button type="button" class="btn btn--sm" data-mact="sel-free" ${sel.size ? '' : 'disabled'}>Delete saved pages</button>
      <button type="button" class="btn btn--sm btn--primary" data-mact="sel-dl" ${sel.size ? '' : 'disabled'}>Download</button></div>` : '';
    const existing = $('#mgSeries', main);
    const html = `${hero}<div class="wrap mg-body">${failedNote(st, busy)}${tools}${selBar}<div class="mg-chlist" id="mgList">${chapterRows(d, st)}</div></div>`;
    if (existing && existing.dataset.slug === d.slug) NA.morph(existing, html);
    else {
      main.innerHTML = `<div id="mgSeries" data-slug="${esc(d.slug)}">${html}</div>`;
      const desc = $('#mgDesc', main);
      if (desc && desc.scrollHeight > desc.clientHeight + 4) desc.insertAdjacentHTML('afterend', '<button type="button" class="btn btn--link mg-more" data-mact="more-desc">More</button>');
    }
  }
  function failedNote(st, busy) {
    if (!st.failed || busy) return '';
    return `<div class="note note--red" data-key="fnote">${NA.plural(st.failed, 'chapter')} couldn’t be saved. The reason is shown next to each one. Retry usually works once the site is reachable again.</div>`;
  }
  function chapterRows(d, st) {
    const q = M.chQuery.trim().toLowerCase();
    const readNum = st.reading ? st.reading.number : 0;
    let list = st.ch.filter((c) => {
      if (M.chFilter === 'ok' && c.state !== 'ok') return false;
      if (M.chFilter === 'todo' && (c.state === 'ok' || c.removed)) return false;
      if (M.chFilter === 'failed' && c.state !== 'failed') return false;
      if (M.chFilter === 'unread' && c.number <= readNum) return false;
      if (q && !(String(c.number) === q || c.title.toLowerCase().includes(q))) return false;
      return true;
    });
    if (M.chOrder === 'desc') list = list.slice().reverse();
    if (!list.length) return '<p class="mg-none">No chapters here.</p>';
    const limit = M.chShowAll || q ? list.length : 150;
    const job = jobFor(d.slug);
    const sel = M.select;
    const rows = list.slice(0, limit).map((c) => {
      const ok = c.state === 'ok';
      const here = c.key === d.readKey;
      const nowSaving = job && job.message && job.message.indexOf(c.title + ' — ') === 0;
      let state;
      if (nowSaving) state = `<span class="mg-st mg-st--busy">${NA.spinner(12)} ${esc(job.message.slice(c.title.length + 3))}</span>`;
      else if (ok) state = `<span class="mg-st">${NA.num(c.pages)} pages · ${sizeOf(c.bytes)}</span>`;
      else if (c.state === 'failed') state = `<span class="mg-st mg-st--bad" title="${esc(c.error)}">${esc(c.error)}</span><button type="button" class="btn btn--sm" data-mact="dl-one" data-key="${c.key}">Retry</button>`;
      else state = `<button type="button" class="btn btn--sm btn--ghost" data-mact="dl-one" data-key="${c.key}">Download</button>`;
      const title = `${esc(c.title)}${here ? `<small class="mg-tag">Reading${d.readPage ? ` · page ${d.readPage + 1}` : ''}</small>` : ''}${c.removed ? '<small class="mg-tag mg-tag--muted">No longer on the site</small>' : ''}`;
      const cls = `mg-ch${here ? ' is-here' : ''}${readNum && c.number < readNum ? ' is-read' : ''}${sel && sel.has(c.key) ? ' is-sel' : ''}`;
      const check = sel ? `<input type="checkbox" class="mg-ch__check" data-mkey="${c.key}" aria-label="Select ${esc(c.title)}" ${sel.has(c.key) ? 'checked' : ''}>` : '';
      return `<div class="${cls}" data-key="c${c.key}">${check}<span class="mg-ch__n">${c.number}</span>${ok && !sel ? `<a class="mg-ch__t" href="${readHref(d.slug, c.key)}">${title}</a>` : `<span class="mg-ch__t">${title}</span>`}<span class="mg-ch__s">${state}</span></div>`;
    }).join('');
    return rows + (list.length > limit ? `<button type="button" class="mg-showall" data-mact="show-all" data-key="more">Show all ${NA.num(list.length)} chapters</button>` : '');
  }
  const refreshSeries = debounce(() => {
    const r = NA.route();
    if (r.name === 'manga' && r.slug === M.seriesSlug) renderSeries($('#main'), r.slug, true);
  }, 500);

  async function seriesAction(t) {
    const d = M.series;
    if (!d) return;
    const slug = d.slug;
    const a = t.dataset.mact;
    if (a === 'dl-one') { t.disabled = true; t.textContent = 'Queued'; return act(() => call('DownloadKeys', slug, [+t.dataset.key])); }
    if (a === 'dl-next') return act(() => call('DownloadKeys', slug, t.dataset.keys.split(',').map(Number)), 'Downloading the next chapters.');
    if (a === 'dl-all') return act(() => call('Download', slug, 0, 0), 'Downloading every remaining chapter.');
    if (a === 'retry') return act(() => call('RetryFailed', slug), 'Trying the failed chapters again.');
    if (a === 'check') return act(() => call('CheckUpdates', slug), 'Checking for new chapters…');
    if (a === 'series-menu') {
      return NA.openMenu(t, [
        { label: 'Save as CBZ file…', sub: 'One file for comic readers', fn: () => exportCBZ(slug) },
        onPhone() ? null : { label: 'Show folder', fn: () => call('OpenFolder', slug) },
        { label: `Open on ${d.siteName}`, fn: () => NA.openExternal(d.url) },
        { label: 'Delete saved pages of read chapters…', sub: 'Frees space; they can be downloaded again', fn: () => freeRead() },
        'sep',
        { label: 'Remove from library…', danger: true, fn: () => removeSeries(slug, d.title) },
      ].filter(Boolean));
    }
    if (a === 'more-desc') { const el = $('#mgDesc'); el.classList.toggle('mg-clamp'); t.textContent = el.classList.contains('mg-clamp') ? 'More' : 'Less'; return; }
    if (a === 'show-all') { M.chShowAll = true; return paintSeries($('#main')); }
    if (a === 'select') { M.select = M.select ? null : new Set(); return paintSeries($('#main')); }
    if (a === 'sel-all') { $$('[data-mkey]').forEach((c) => M.select.add(+c.dataset.mkey)); return paintSeries($('#main')); }
    if (a === 'sel-dl') { const keys = [...M.select]; M.select = null; paintSeries($('#main')); return act(() => call('DownloadKeys', slug, keys), `Downloading ${NA.plural(keys.length, 'chapter')}.`); }
    if (a === 'sel-free') {
      const keys = [...M.select];
      const ok = await NA.confirmDialog({ title: `Delete saved pages of ${NA.plural(keys.length, 'chapter')}?`, body: 'The chapters stay in the list and can be downloaded again.', ok: 'Delete pages', danger: true });
      if (!ok) return;
      M.select = null;
      return act(async () => { const freed = await call('FreeKeys', slug, keys); NA.toast(`Freed ${sizeOf(freed)}.`, { tone: 'ok' }); });
    }
  }
  async function freeRead() {
    const d = M.series;
    const st = seriesStats(d);
    if (!st.reading) { NA.toast('Nothing read yet, so there’s nothing to free.'); return; }
    const keys = st.ch.filter((c) => c.state === 'ok' && c.number < st.reading.number).map((c) => c.key);
    if (!keys.length) { NA.toast('No saved chapters before the one you’re reading.'); return; }
    const ok = await NA.confirmDialog({ title: `Delete pages of ${NA.plural(keys.length, 'read chapter')}?`, body: `Everything before chapter ${st.reading.number}. They stay listed and can be downloaded again.`, ok: 'Delete pages', danger: true });
    if (!ok) return;
    act(async () => { const freed = await call('FreeKeys', d.slug, keys); NA.toast(`Freed ${sizeOf(freed)}.`, { tone: 'ok' }); });
  }

  // ---------------------------------------------------------------- reader
  const MODES = [['strip', 'Strip', 'Long strip (webtoon)'], ['rtl', 'R→L', 'Pages, right to left (manga)'], ['ltr', 'L→R', 'Pages, left to right'], ['spread', 'Two pages', 'Two pages side by side, right to left']];
  const stripW = () => +(NA.store.get('ma-stripw', 760));
  async function renderReader(main, slug, key) {
    leaveReader();
    main.innerHTML = `<div class="mg-reader"><div class="mg-rload">${NA.spinner(22, '#F0644A')}</div></div>`;
    let v;
    try { v = await call('Chapter', slug, key); } catch (e) {
      main.innerHTML = `<div class="mg-reader"><div class="mg-rempty"><h2>Couldn’t open this chapter</h2><p>${esc(e.message)}</p><a class="btn" href="${seriesHref(slug)}">Back to series</a></div></div>`;
      return;
    }
    const r0 = NA.route();
    if (r0.name !== 'read' || r0.slug !== slug || r0.key !== key) return;
    const R = M.reader = { slug, key, v, mode: v.readMode, page: Math.min(v.readPage || 0, Math.max(0, v.pages.length - 1)), cleanup: [], waitNext: 0 };
    call('SetReading', slug, key, R.page).catch(() => {});
    const save = debounce((p) => call('SetReading', slug, key, p).catch(() => {}), 600);
    R.save = save;
    main.innerHTML = `<div class="mg-reader" id="mgR" style="--strip-w:${stripW()}px" tabindex="-1">
      <div class="mg-rbar mg-rbar--top">
        <a class="icon-btn" href="${seriesHref(slug)}" aria-label="Back to series" title="Back (Esc)">${NA.icon('back', 18)}</a>
        <div class="mg-rtitle"><b>${esc(v.title)}</b><span>${esc(v.series)} · ${NA.num(v.number)} of ${NA.num(v.total)}</span></div>
        <div class="seg mg-modes" role="group" aria-label="Reading mode">${MODES.map(([id, l, t]) => `<button type="button" data-rmode="${id}" aria-pressed="${R.mode === id}" title="${t}">${l}</button>`).join('')}</div>
        <button type="button" class="icon-btn" data-rw="-1" aria-label="Narrower pages" title="Narrower">${NA.icon('minus', 18)}</button>
        <button type="button" class="icon-btn" data-rw="1" aria-label="Wider pages" title="Wider">${NA.icon('plus', 18)}</button>
      </div>
      <div id="mgRBody"></div>
      <div class="mg-rbar mg-rbar--bot">
        <button type="button" class="btn btn--sm mg-rbtn" data-rch="${v.prevKey}" ${v.prevKey ? '' : 'disabled'} aria-label="Previous chapter">${NA.icon('back', 14)} <span class="mg-rlbl">Previous</span></button>
        <input type="range" id="mgRSlider" min="1" max="${Math.max(1, v.pages.length)}" value="${R.page + 1}" aria-label="Page">
        <span class="mg-rcount" id="mgRCount"></span>
        <button type="button" class="btn btn--sm mg-rbtn" data-rch="${v.nextKey}" ${v.nextKey ? '' : 'disabled'} aria-label="Next chapter"><span class="mg-rlbl">Next</span> ${NA.icon('arrow', 14)}</button>
      </div></div>`;
    const root = $('#mgR');
    const body = $('#mgRBody');
    const slider = $('#mgRSlider');
    const count = $('#mgRCount');
    let uiTimer;
    const showUI = (on = true) => {
      root.classList.toggle('ui-off', !on);
      clearTimeout(uiTimer);
      if (on) uiTimer = setTimeout(() => root.classList.add('ui-off'), 2600);
    };
    R.cleanup.push(() => clearTimeout(uiTimer));
    const setCount = () => {
      const n = v.pages.length;
      const p = Math.min(R.page, n - 1);
      count.textContent = n ? (R.mode === 'spread' && p + 1 < n ? `${p + 1}–${p + 2} / ${n}` : `${p + 1} / ${n}`) : '';
      slider.value = p + 1;
    };
    const pageBox = (i, eager) => {
      const [w, h] = (v.dims && v.dims[i]) || [0, 0];
      const ratio = w && h ? `aspect-ratio:${w}/${h};` : '';
      return `<div class="mg-page" data-n="${i + 1}" style="${ratio}"><img src="${esc(v.pages[i])}" alt="Page ${i + 1}" data-i="${i}" ${w ? `width="${w}" height="${h}"` : ''} ${eager ? '' : 'loading="lazy"'} decoding="async"></div>`;
    };
    const endCard = () => {
      if (!v.nextKey) return `<div class="mg-end"><span class="mg-end__k">End of ${esc(v.title)}</span><h2>You’re caught up</h2><p>That’s the last chapter in your library for ${esc(v.series)}.</p><div class="mg-end__acts"><button type="button" class="btn mg-rbtn" data-mact="r-check">Check for new chapters</button><a class="btn mg-rbtn" href="${seriesHref(slug)}">Back to series</a></div></div>`;
      if (v.nextSaved) return `<div class="mg-end"><span class="mg-end__k">End of ${esc(v.title)}</span><h2>Next: chapter ${NA.num(v.number + 1)}</h2><p>Saved and ready.</p><div class="mg-end__acts"><a class="btn btn--primary mg-go" href="${readHref(slug, v.nextKey)}">Read next chapter ${NA.icon('arrow', 15)}</a></div></div>`;
      return `<div class="mg-end" id="mgEnd"><span class="mg-end__k">End of ${esc(v.title)}</span><h2>Next: chapter ${NA.num(v.number + 1)}</h2><p id="mgEndMsg">It isn’t saved yet.</p><div class="bar" id="mgEndBar" hidden><i style="--p:0"></i></div><div class="mg-end__acts"><button type="button" class="btn btn--primary mg-go" data-mact="r-getnext">Download next chapter</button><a class="btn mg-rbtn" href="${seriesHref(slug)}">Back to series</a></div></div>`;
    };
    let io = null;
    function paint() {
      if (io) { io.disconnect(); io = null; }
      $$('[data-rmode]', root).forEach((b) => b.setAttribute('aria-pressed', b.dataset.rmode === R.mode));
      slider.style.direction = R.mode === 'rtl' || R.mode === 'spread' ? 'rtl' : 'ltr';
      if (!v.pages.length) {
        body.innerHTML = `<div class="mg-rempty"><h2>This chapter isn’t saved yet</h2><p>Download it from the series page, then open it again.</p><div class="mg-end__acts"><button type="button" class="btn btn--primary" data-mact="r-getthis">Download this chapter</button><a class="btn mg-rbtn" href="${seriesHref(slug)}">Back to series</a></div></div>`;
        setCount();
        return;
      }
      if (R.mode === 'strip') {
        body.innerHTML = `<div class="mg-scroll" id="mgScroll"><div class="mg-strip">${v.pages.map((_, i) => pageBox(i, i <= R.page + 2)).join('')}</div>${endCard()}</div>`;
        const sc = $('#mgScroll');
        const imgs = $$('img', sc);
        imgs.forEach((im) => {
          const done = () => im.parentElement.classList.add('is-in');
          if (im.complete && im.naturalWidth) done(); else im.addEventListener('load', done, { once: true });
          im.addEventListener('error', () => im.parentElement.classList.add('is-bad'), { once: true });
        });
        // Pages keep their real height before loading, so jumping straight
        // to the saved page is exact.
        if (R.page > 0 && imgs[R.page]) requestAnimationFrame(() => imgs[R.page].parentElement.scrollIntoView({ block: 'start' }));
        io = new IntersectionObserver((ents) => {
          for (const en of ents) if (en.isIntersecting) { R.page = +en.target.dataset.i; setCount(); save(R.page); }
        }, { root: sc, rootMargin: '-45% 0px -54% 0px' });
        imgs.forEach((im) => io.observe(im));
        let lastY = 0;
        sc.addEventListener('scroll', () => { const y = sc.scrollTop; if (Math.abs(y - lastY) > 40) { root.classList.toggle('ui-off', y > lastY); lastY = y; } }, { passive: true });
        sc.addEventListener('click', (e) => { if (!e.target.closest('a,button')) showUI(root.classList.contains('ui-off')); });
      } else paintPage(0);
      setCount();
    }
    function paintPage(dir) {
      const n = v.pages.length;
      const atEnd = R.page >= n;
      const rtl = R.mode !== 'ltr';
      const slideCls = dir ? ` mg-turn mg-turn--${(dir > 0) === rtl ? 'l' : 'r'}` : '';
      let inner;
      if (atEnd) inner = `<div class="mg-paged mg-paged--end">${endCard()}</div>`;
      else if (R.mode === 'spread') {
        const a = pageBox(R.page, true), b = R.page + 1 < n ? pageBox(R.page + 1, true) : '';
        inner = `<div class="mg-paged mg-paged--spread${slideCls}">${b}${a}</div>`; // right-to-left: first page on the right
      } else inner = `<div class="mg-paged${slideCls}">${pageBox(R.page, true)}</div>`;
      const next = rtl ? 'l' : 'r';
      body.innerHTML = `${inner}
        <button type="button" class="mg-zone mg-zone--l" data-turn="${next === 'l' ? 1 : -1}" aria-label="${next === 'l' ? 'Next' : 'Previous'} page"></button>
        <button type="button" class="mg-zone mg-zone--r" data-turn="${next === 'r' ? 1 : -1}" aria-label="${next === 'r' ? 'Next' : 'Previous'} page"></button>`;
      $$('.mg-page img', body).forEach((im) => {
        const done = () => im.parentElement.classList.add('is-in');
        if (im.complete && im.naturalWidth) done(); else im.addEventListener('load', done, { once: true });
        im.addEventListener('error', () => im.parentElement.classList.add('is-bad'), { once: true });
      });
      const step = R.mode === 'spread' ? 2 : 1;
      for (let i = 1; i <= 2 * step; i++) if (v.pages[R.page + i]) { const im = new Image(); im.src = v.pages[R.page + i]; }
      if (!atEnd) { setCount(); save(R.page); }
    }
    function turn(d) {
      if (R.mode === 'strip') return;
      const step = R.mode === 'spread' ? 2 : 1;
      const np = R.page + d * step;
      if (np < 0) { if (R.page > 0) { R.page = 0; paintPage(d); } else if (v.prevKey) location.hash = readHref(slug, v.prevKey); return; }
      if (np >= v.pages.length) {
        if (R.page >= v.pages.length) { if (v.nextKey && v.nextSaved) location.hash = readHref(slug, v.nextKey); return; }
        R.page = v.pages.length; paintPage(d); return;
      }
      R.page = np;
      paintPage(d);
    }
    R.turn = turn;
    paint();
    showUI(true);

    root.addEventListener('click', async (e) => {
      const t = e.target.closest('button,[data-rmode]');
      if (!t) { if (R.mode !== 'strip' && e.target.closest('.mg-paged')) showUI(root.classList.contains('ui-off')); return; }
      if (t.dataset.rmode) { setMode(t.dataset.rmode); return; }
      if (t.dataset.rw) {
        const w = Math.max(420, Math.min(1600, stripW() + (+t.dataset.rw) * 120));
        NA.store.set('ma-stripw', w);
        root.style.setProperty('--strip-w', w + 'px');
        return;
      }
      if (t.dataset.rch) { location.hash = readHref(slug, t.dataset.rch); return; }
      if (t.dataset.turn) { turn(+t.dataset.turn); return; }
      const a = t.dataset.mact;
      if (a === 'r-getnext') {
        t.disabled = true; t.innerHTML = `${NA.spinner(14, '#fff')} Downloading…`;
        R.waitNext = v.nextKey;
        const bar = $('#mgEndBar'); if (bar) bar.hidden = false;
        act(() => call('DownloadKeys', slug, [v.nextKey]));
      } else if (a === 'r-getthis') {
        t.disabled = true; t.innerHTML = `${NA.spinner(14, '#fff')} Downloading…`;
        R.waitThis = true;
        act(() => call('DownloadKeys', slug, [key]));
      } else if (a === 'r-check') act(() => call('CheckUpdates', slug), 'Checking for new chapters…');
    });
    function setMode(m) {
      if (R.mode === m) return;
      R.mode = m;
      call('SetReadMode', slug, m).catch(() => {});
      if (R.page >= v.pages.length) R.page = Math.max(0, v.pages.length - 1);
      paint();
      showUI(true);
    }
    slider.addEventListener('input', () => {
      R.page = +slider.value - 1;
      if (R.mode === 'strip') { const im = $(`#mgScroll img[data-i="${R.page}"]`); if (im) im.parentElement.scrollIntoView({ block: 'start' }); }
      else paintPage(0);
      setCount();
    });
    root.addEventListener('mousemove', (e) => { if (e.clientY < 72 || e.clientY > innerHeight - 72) showUI(true); });
    // Touch (phones, touch screens): swipe sideways to turn pages, in the
    // reading direction; double-tap a page to zoom in and pan, again to fit.
    let t0 = null, lastTap = 0;
    root.addEventListener('touchstart', (e) => {
      if (e.touches.length !== 1) { t0 = null; return; }
      t0 = { x: e.touches[0].clientX, y: e.touches[0].clientY, at: Date.now() };
    }, { passive: true });
    root.addEventListener('touchend', (e) => {
      if (!t0 || R.mode === 'strip') { t0 = null; return; }
      const p = e.changedTouches[0];
      const dx = p.clientX - t0.x, dy = p.clientY - t0.y, dt = Date.now() - t0.at;
      const paged = $('.mg-paged', body);
      t0 = null;
      if (!paged) return;
      if (paged.classList.contains('is-zoom')) {
        if (Math.abs(dx) < 10 && Math.abs(dy) < 10 && Date.now() - lastTap < 300) { paged.classList.remove('is-zoom'); lastTap = 0; }
        else lastTap = Date.now();
        return;
      }
      if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.4 && dt < 700) {
        const rtl = R.mode !== 'ltr';
        turn((rtl ? dx > 0 : dx < 0) ? 1 : -1);
        e.preventDefault();
        return;
      }
      if (Math.abs(dx) < 10 && Math.abs(dy) < 10 && e.target.closest('.mg-page')) {
        if (Date.now() - lastTap < 300) {
          paged.classList.add('is-zoom');
          const r = paged.getBoundingClientRect();
          paged.scrollLeft = Math.max(0, (p.clientX - r.left) * 1.5);
          paged.scrollTop = Math.max(0, (p.clientY - r.top) * 1.5);
          lastTap = 0;
          e.preventDefault();
        } else lastTap = Date.now();
      }
    });
    const onKey = (e) => {
      if (NA.isOverlayOpen() || (e.target.matches && e.target.matches('input:not([type=range]),select,textarea'))) return;
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      const k = e.key;
      if (k === 'Escape') { location.hash = seriesHref(slug); return; }
      if (/^[1-4]$/.test(k)) { setMode(MODES[+k - 1][0]); return; }
      if (R.mode === 'strip') {
        if (k === 'ArrowRight' && v.nextKey) location.hash = readHref(slug, v.nextKey);
        if (k === 'ArrowLeft' && v.prevKey) location.hash = readHref(slug, v.prevKey);
        return;
      }
      const rtl = R.mode !== 'ltr';
      if (k === 'ArrowLeft') turn(rtl ? 1 : -1);
      else if (k === 'ArrowRight') turn(rtl ? -1 : 1);
      else if (k === ' ' || k === 'PageDown' || k === 'ArrowDown') turn(1);
      else if (k === 'PageUp' || k === 'ArrowUp') turn(-1);
      else return;
      e.preventDefault();
    };
    document.addEventListener('keydown', onKey);
    R.cleanup.push(() => document.removeEventListener('keydown', onKey));
    R.cleanup.push(() => { if (io) io.disconnect(); });
    root.focus({ preventScroll: true });
  }
  function leaveReader() {
    const R = M.reader;
    if (!R) return;
    M.reader = null;
    R.cleanup.forEach((f) => f());
    call('SetReading', R.slug, R.key, Math.min(R.page, Math.max(0, R.v.pages.length - 1))).catch(() => {});
  }
  // Live updates for the chapter hand-off at the end of a chapter.
  function readerProgress(j) {
    const R = M.reader;
    if (!R || !R.waitNext || j.slug !== R.slug) return;
    const next = (R.v && R.v.nextKey) || 0;
    if (!next) return;
    const m = /— page (\d+) of (\d+)/.exec(j.message || '');
    const msg = $('#mgEndMsg'), bar = $('#mgEndBar');
    if (m && msg) { msg.textContent = `Downloading · page ${m[1]} of ${m[2]}`; if (bar) { bar.hidden = false; $('i', bar).style.setProperty('--p', (+m[1] / +m[2]).toFixed(3)); } }
  }
  function readerSaved(ev) {
    const R = M.reader;
    if (!R || !ev || ev.slug !== R.slug) return;
    if (R.waitNext && ev.chapter === R.waitNext) {
      const at = R.page >= R.v.pages.length || R.mode === 'strip';
      const target = readHref(R.slug, R.waitNext);
      R.waitNext = 0;
      if (at && NA.route().name === 'read') location.hash = target; // opens by itself when ready
      else NA.toast('Next chapter saved.', { tone: 'ok', action: { label: 'Read it', fn: () => { location.hash = target; } } });
    }
    if (R.waitThis && ev.chapter === R.key) { R.waitThis = false; renderReader($('#main'), R.slug, R.key); }
  }

  // ---------------------------------------------------------------- sites
  // One line per novel site, from the live checks in internal/sites/README.md.
  const NOVEL_NOTES = {
    novelbin: 'Reads the site’s own chapter data, so lists are complete.',
    novelfull: 'Full chapter list, 50 per page, and every chapter.',
    readnovelfull: 'Whole chapter list in one request.',
    novelping: 'Fast chapter list and new-chapter checks.',
    royalroad: 'Original fiction. Hidden “stolen content” lines are removed.',
    wuxiaworld: 'Free chapters. Paid ones are skipped.',
    freewebnovel: 'Numbered chapters, very reliable.',
    novelfire: 'List over the internet, chapters in the site window.',
    webnovel: 'Free chapters. Locked ones are skipped.',
    empirenovel: 'Tick the check once; it’s remembered after that.',
    scribblehub: 'Follows “Next” from chapter 1.',
    wattpad: 'Stories and their parts; Paid Stories parts are skipped.',
    ao3: 'Works and chapters, Mature ones included. Members-only works need a login.',
    uukanshu: 'Traditional Chinese. Search finds novels by their Chinese title.',
    '69shuba': 'Simplified Chinese, fast updates. Paste its links; its search always asks you to verify.',
    syosetu: 'Free Japanese web novels. Search uses the site’s official API.',
    kakuyomu: 'Free Japanese web novels from Kadokawa.',
  };
  const LANG_PILL = { zh: 'Chinese · translated', ja: 'Japanese · translated', ko: 'Korean · translated' };
  const tested = (v) => {
    const m = /(\d{4})-(\d{2})-(\d{2})/.exec(String(v || ''));
    if (!m) return '';
    const d = new Date(+m[1], +m[2] - 1, +m[3]);
    return 'Tested ' + d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
  };
  const NOVEL_STATUS = {
    verified: ['direct', 'Works directly'], partial: ['window', 'Partly uses the site window'],
    'browser-only': ['window', 'Uses the site window'], 'human-check': ['check', 'May ask you to verify'],
  };
  async function renderSites(main) {
    main.innerHTML = `<div class="wrap mg-sites"><div class="stack" style="gap:6px"><a class="back" href="#/">${NA.icon('back', 16)} Library</a><h1 class="page-h">Supported sites</h1></div><div class="mg-sitegrid" aria-busy="true">${'<div class="mg-site mg-site--sk"><div class="mg-sk" style="height:40px;width:40px;border-radius:10px"></div><div class="mg-sk" style="height:14px;width:60%"></div><div class="mg-sk" style="height:12px;width:90%"></div></div>'.repeat(6)}</div></div>`;
    const [novels, manga] = await Promise.all([
      NA.api('NovelSites').catch(() => []),
      ready ? call('Sites').catch(() => []) : Promise.resolve([]),
    ]);
    if (NA.route().name !== 'sites') return;
    M.sites = { novels: novels || [], manga: manga || [] };
    paintSites(main);
  }
  function siteCard(s, kind) {
    let tone, label, kindLabel, note;
    if (kind === 'novel') {
      [tone, label] = NOVEL_STATUS[s.status] || ['direct', 'Works directly'];
      kindLabel = LANG_PILL[s.language] || '';
      note = [NOVEL_NOTES[s.id] || '', tested(s.verified)].filter(Boolean).join(' ');
    } else {
      tone = s.status === 'tested' ? 'direct' : 'check';
      label = tone === 'direct' ? 'Works directly' : 'May ask you to verify';
      kindLabel = s.type;
      note = tone === 'direct' ? 'Tested with a real download, 24 Nov 2026.' : 'Uses the site window. The check usually passes by itself; if not, tick it once. Tested 24 Nov 2026.';
    }
    const host = (s.domains && s.domains[0]) || String(s.home || '').replace(/^https?:\/\/(www\.)?/, '').replace(/\/.*$/, '');
    const needsWindow = tone !== 'direct';
    return `<article class="mg-site" data-key="s-${kind}-${esc(s.id)}">
      <div class="mg-site__head"><div style="min-width:0"><h3>${esc(s.name)}</h3><span class="muted">${esc(host)}</span></div></div>
      <div class="mg-chips">${kindLabel ? `<span class="pill">${esc(kindLabel)}</span>` : ''}<span class="mg-status mg-status--${tone}"><i></i>${esc(label)}</span></div>
      ${note ? `<p class="mg-site__note">${esc(note)}</p>` : ''}
      <div class="mg-site__acts">
        ${s.example ? `<button type="button" class="btn btn--sm" data-mact="try" data-url="${esc(s.example)}">Try an example</button>` : ''}
        ${s.home ? `<button type="button" class="btn btn--sm btn--ghost" data-mact="home" data-url="${esc(s.home)}">Open site</button>` : ''}
        ${needsWindow && s.home ? `<button type="button" class="btn btn--sm btn--ghost" data-mact="window" data-url="${esc(s.home)}">Site window</button>` : ''}
      </div></article>`;
  }
  function paintSites(main) {
    const S = M.sites;
    const tab = ready ? M.sitesTab : 'novel';
    const list = tab === 'manga' ? S.manga : S.novels;
    const tabs = ready ? `<div class="seg" role="tablist" aria-label="Kind of site">${[['novel', 'Novels', S.novels.length], ['manga', 'Manga & comics', S.manga.length]].map(([k, l, c]) => `<button type="button" role="tab" data-stab="${k}" aria-selected="${tab === k}" aria-pressed="${tab === k}">${l} <span class="n">${c}</span></button>`).join('')}</div>` : '';
    const not = tab === 'manga'
      ? 'Bato.to has closed. Comick no longer hosts images. Mangago and FanFox scramble their page lists. ManhwaTop, KunManga, ManhuaUS, ManhuaFast and ManhwaClan block the request that loads their chapter list.'
      : 'Other sites open in reader mode: the app looks for the chapter text and the Next link. It works on many simple sites, but it isn’t tested. You can add a profile for a site in the library’s “sites” folder (see the project README).';
    const html = `<div class="stack" style="gap:6px"><a class="back" href="#/">${NA.icon('back', 16)} Library</a>
        <div class="mg-sites__head"><div><h1 class="page-h">Supported sites</h1><p class="muted">Paste a series, novel or chapter link from any of these. If a site asks “Verify you are human”, a site window opens: tick the box once and the download carries on.</p></div>${tabs}</div></div>
      <div class="mg-legend"><span><i class="mg-dot mg-dot--direct"></i>Works directly</span><span><i class="mg-dot mg-dot--window"></i>Uses the site window</span><span><i class="mg-dot mg-dot--check"></i>May ask you to verify</span></div>
      <div class="mg-sitegrid">${list.map((s) => siteCard(s, tab)).join('') || '<p class="muted">No sites listed.</p>'}</div>
      <details class="card mg-notyet"><summary>${tab === 'manga' ? 'Manga sites that don’t work yet, and why' : 'Sites that aren’t listed'}</summary><p>${esc(not)}</p></details>`;
    const box = $('.mg-sites', main);
    if (box) NA.morph(box, html);
  }

  // ---------------------------------------------------------------- settings
  function settingsHTML(row, desktop) {
    const s = M.settings || {};
    const sel = (key, opts, label) => `<select class="select" data-mset="${key}" aria-label="${esc(label)}">${opts.map(([v, l]) => `<option value="${v}" ${String(s[key]) === String(v) ? 'selected' : ''}>${l}</option>`).join('')}</select>`;
    const sw = (key, label) => `<span class="switch"><input type="checkbox" role="switch" data-mset="${key}" aria-label="${esc(label)}" ${s[key] ? 'checked' : ''}><span></span></span>`;
    return `<section class="set-group" id="manga" aria-labelledby="sM"><h2 class="section-h" id="sM">Manga</h2><div class="card">
      ${row('Reading mode', 'How chapters open. A long strip scrolls up and down; pages turn with a swipe. You can change it per series in the reader.', sel('readMode', [['strip', 'Long strip (scroll)'], ['rtl', 'Pages, right to left'], ['ltr', 'Pages, left to right'], ['spread', 'Two pages']], 'Manga reading mode'))}
      ${row('Chapters at once', 'More is faster but busier for the site.', sel('chapterWorkers', [[1, '1'], [2, '2'], [3, '3'], [4, '4']], 'Chapters at once'))}
      ${row('Pages at once per chapter', 'Pages of one chapter download in parallel.', sel('pageWorkers', [[2, '2'], [4, '4'], [6, '6'], [8, '8']], 'Pages at once'))}
      ${row('Download new chapters automatically', 'When a check finds new chapters of a series you’re downloading, save them too.', sw('autoDownloadNew', 'Download new chapters automatically'))}
      ${row('Check manga for new chapters when the app opens', 'Looks at every series a few seconds after start. “Get new chapters” on the library does this any time.', sw('checkOnOpen', 'Check manga on open'))}
      ${row(`${sizeOf(M.lib.reduce((a, it) => a + (it.bytes || 0), 0))} used by manga`, `${NA.plural(M.lib.length, 'series', 'series')} · ${NA.plural(M.lib.reduce((a, it) => a + (it.downloaded || 0), 0), 'chapter')} saved. To free space, open a series and choose “Delete saved pages of read chapters”.`, '')}
      ${desktop ? row('Manga library folder', esc(M.libFolder || 'In your AppData folder, beside the novels'), `<button type="button" class="btn btn--md" data-mact="open-mlib">Open folder</button>`) : ''}
    </div></section>`;
  }
  function wireSettings(main) {
    $$('[data-mset]', main).forEach((el) => el.addEventListener('change', async () => {
      const k = el.dataset.mset;
      const val = el.type === 'checkbox' ? el.checked : (/Workers$/.test(k) ? +el.value : el.value);
      try { M.settings = await call('SaveSettings', Object.assign({}, M.settings, { [k]: val })); NA.toast('Saved', { timeout: 1500 }); } catch (e) { NA.toast(e.message, { tone: 'bad' }); }
    }));
  }

  // ---------------------------------------------------------------- palette
  function paletteItems(q) {
    q = String(q || '').trim().toLowerCase();
    return M.lib.filter((it) => !q || `${it.title} ${it.siteName}`.toLowerCase().includes(q)).slice(0, 6).map((it) => ({
      group: 'Manga',
      title: it.readKey ? `Continue ${it.title}` : `Open ${it.title}`,
      sub: it.readKey ? `Chapter ${NA.num(it.readNumber)}` : `${NA.num(it.downloaded)} of ${NA.plural(it.total, 'chapter')} saved`,
      thumb: NA.coverHTML(it.cover, it.title, '', true),
      run: () => { location.hash = it.readKey ? readHref(it.slug, it.readKey) : seriesHref(it.slug); },
    }));
  }

  // ---------------------------------------------------------------- events
  const refreshLib = debounce(async () => {
    await loadLib();
    const r = NA.route();
    if (r.name === 'library') NA.renderShelf();
  }, 400);
  const shelfBadges = debounce(() => { if (NA.route().name === 'library') NA.renderShelf(); }, 250);

  function wireEvents() {
    NA.on('manga:queue', (a) => {
      const before = runningJobs().map((j) => j.slug);
      const seriesWasBusy = !!(M.seriesSlug && busyWith(M.seriesSlug));
      M.activity = { current: a.current || null, running: a.running || (a.current ? [a.current] : []), queue: a.queue || [] };
      paintJobs();
      shelfBadges();
      // A series that stopped downloading: its card changes.
      const now = new Set(runningJobs().map((j) => j.slug));
      if (before.some((slug) => !now.has(slug))) refreshLib();
      // Redraw the open series page only when its own download starts or ends.
      if (M.seriesSlug && NA.route().name === 'manga' && busyWith(M.seriesSlug) !== seriesWasBusy) refreshSeries();
    });
    NA.on('manga:progress', (j) => {
      const run = M.activity.running || [];
      const i = run.findIndex((x) => x.id === j.id);
      if (i >= 0) run[i] = j;
      if (M.activity.current && M.activity.current.id === j.id) M.activity.current = j;
      paintJobs();
      readerProgress(j);
      shelfBadges();
      const r = NA.route();
      if (r.name === 'manga' && r.slug === j.slug && M.series) {
        const list = $('#mgList');
        if (list) NA.morph(list, chapterRows(M.series, seriesStats(M.series)));
      }
    });
    NA.on('manga:library', (ev) => {
      readerSaved(ev);
      refreshLib();
      if (ev && ev.slug === M.seriesSlug) refreshSeries();
    });
    NA.on('manga:status', (ev) => {
      if (M.addSteps && ev && ev.message) {
        const m = ev.message.replace(/…$/, '');
        if (M.addSteps[M.addSteps.length - 1] !== m) { M.addSteps.push(m); if (M.paintSteps) M.paintSteps(); }
      }
    });
    NA.on('manga:challenge', (ev) => {
      NA.notify('A site needs you', 'Tick the check in the site window to keep downloading manga.');
      NA.toast(ev.message || 'A site wants you to confirm you’re human.', { tone: 'warn', timeout: 15000, action: { label: 'Open site window', fn: () => NA.openSiteWindow('') } });
    });
    NA.on('manga:newChapters', (ev) => {
      NA.toast(`${ev.title}: ${NA.plural(ev.count, 'new chapter')}`, { tone: 'ok', action: { label: 'Open', fn: () => { location.hash = seriesHref(ev.slug); } } });
    });
    NA.on('manga:done', (ev) => {
      const j = ev.job || {};
      refreshLib();
      if (j.auto && !ev.error) return;
      if (j.kind === 'update' && !ev.error && /No new/.test(ev.message)) { NA.toast(`${j.title} is up to date`, { timeout: 2500 }); return; }
      if (ev.error) { NA.toast(`${j.title}: ${ev.message}`, { tone: 'bad', timeout: 10000 }); return; }
      if (j.kind === 'download' && j.done) {
        NA.chime();
        NA.notify(`${j.title} is ready`, ev.message);
        const r = NA.route();
        if (!(r.name === 'manga' && r.slug === j.slug) && !(r.name === 'read')) NA.toast(`${j.title}: ${ev.message}`, { tone: 'ok', action: { label: 'Open', fn: () => { location.hash = seriesHref(j.slug); } } });
      }
    });
  }

  // Clicks for everything manga.js draws (data-mact), anywhere in the page.
  document.addEventListener('click', (e) => {
    const t = e.target.closest('[data-mact]');
    if (t) {
      const a = t.dataset.mact;
      if (a === 'close') { e.preventDefault(); NA.closeOverlay(); if (t.getAttribute('href')) location.hash = t.getAttribute('href'); return; }
      if (a === 'card-menu') { e.preventDefault(); seriesMenu(t, t.dataset.slug); return; }
      if (a === 'stop') { (t.dataset.id ? call('CancelJob', +t.dataset.id) : call('CancelCurrent')).catch(() => {}); return; }
      if (a === 'stop-all') { call('CancelAll').catch(() => {}); return; }
      if (a === 'unqueue') { call('RemoveQueued', +t.dataset.id).catch(() => {}); return; }
      if (a === 'try') { NA.addLink(t.dataset.url); return; }
      if (a === 'home') { NA.openExternal(t.dataset.url); return; }
      if (a === 'window') { NA.openSiteWindow(t.dataset.url); return; }
      if (a === 'open-mlib') { call('OpenFolder', '').catch(() => {}); return; }
      if (NA.route().name === 'manga') seriesAction(t);
      return;
    }
    const f = e.target.closest('[data-mfilter]');
    if (f) { M.chFilter = f.dataset.mfilter; M.chShowAll = false; paintSeries($('#main')); return; }
    const o = e.target.closest('[data-morder]');
    if (o) { M.chOrder = o.dataset.morder; NA.store.set('ma-chorder', M.chOrder); paintSeries($('#main')); return; }
    const st = e.target.closest('[data-stab]');
    if (st) { M.sitesTab = st.dataset.stab; NA.store.set('ma-sitestab', M.sitesTab); paintSites($('#main')); }
  });
  document.addEventListener('change', (e) => {
    const c = e.target.closest('[data-mkey]');
    if (c && M.select) { if (c.checked) M.select.add(+c.dataset.mkey); else M.select.delete(+c.dataset.mkey); paintSeries($('#main')); }
  });
  document.addEventListener('input', debounce((e) => {
    if (e.target.id === 'mgFind') { M.chQuery = e.target.value; const list = $('#mgList'); if (list && M.series) NA.morph(list, chapterRows(M.series, seriesStats(M.series))); }
  }, 120));
  document.addEventListener('click', (e) => { if (e.target.closest('#mjobsPill')) openJobs(); });

  window.Manga = {
    init(na) { NA = na; boot(); },
    enabled: () => ready,
    items: () => M.lib.map(normalize),
    card,
    recognises: async (url) => { if (!ready) return false; try { return !!(await mg().Recognises(url)); } catch (e) { return false; } },
    openAdd,
    checkAll: async (auto) => { if (!ready) return 0; try { return (await mg().CheckAll(!!auto)) || 0; } catch (e) { return 0; } },
    paintJobs,
    renderSites,
    renderSeries: (main, slug) => renderSeries(main, slug, false),
    renderReader,
    leave(r) {
      if (M.reader && !(r.name === 'read' && r.slug === M.reader.slug && r.key === M.reader.key)) leaveReader();
      if (r.name !== 'manga') { M.seriesSlug = ''; M.series = null; M.select = null; }
    },
    settingsHTML,
    wireSettings,
    paletteItems,
  };
})();
