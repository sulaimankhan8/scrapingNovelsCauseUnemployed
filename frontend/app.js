document.addEventListener('DOMContentLoaded', () => {
    // Top Intake Elements
    const novelUrlInput = document.getElementById('novelUrl');
    const previewBtn = document.getElementById('previewBtn');
    const scrapeForm = document.getElementById('scrapeForm');
    const clearUrlBtn = document.getElementById('clearUrlBtn');
    const toggleOptionsBtn = document.getElementById('toggleOptionsBtn');
    const optionsPanel = document.getElementById('optionsPanel');

    // Discovery Preview Elements
    const previewCard = document.getElementById('previewCard');
    const previewCover = document.getElementById('previewCover');
    const previewTitle = document.getElementById('previewTitle');
    const previewAuthor = document.getElementById('previewAuthor');
    const previewChapters = document.getElementById('previewChapters');
    const previewDesc = document.getElementById('previewDesc');
    const startFromPreviewBtn = document.getElementById('startFromPreviewBtn');

    // Progress Monitor Elements
    const progressSection = document.getElementById('progressSection');
    const progressNovelTitle = document.getElementById('progressNovelTitle');
    const stageBadge = document.getElementById('stageBadge');
    const progressPercent = document.getElementById('progressPercent');
    const progressSpeed = document.getElementById('progressSpeed');
    const progressBar = document.getElementById('progressBar');
    const progressCurrentStatus = document.getElementById('progressCurrentStatus');
    const progressCounts = document.getElementById('progressCounts');
    const statDownloaded = document.getElementById('statDownloaded');
    const statCached = document.getElementById('statCached');
    const statCleaned = document.getElementById('statCleaned');
    const statEta = document.getElementById('statEta');
    const chapterTicker = document.getElementById('chapterTicker');
    const completionBanner = document.getElementById('completionBanner');
    const completionTitle = document.getElementById('completionTitle');
    const completionDesc = document.getElementById('completionDesc');
    const completionReadLink = document.getElementById('completionReadLink');
    const completionDismissBtn = document.getElementById('completionDismissBtn');
    const logConsole = document.getElementById('logConsole');
    const toggleConsoleBtn = document.getElementById('toggleConsoleBtn');
    const clearConsoleBtn = document.getElementById('clearConsoleBtn');

    // Library Elements
    const libraryGrid = document.getElementById('libraryGrid');
    const librarySearch = document.getElementById('librarySearch');
    const clearSearchBtn = document.getElementById('clearSearchBtn');
    const tabFilterAll = document.getElementById('tabFilterAll');
    const tabFilterComplete = document.getElementById('tabFilterComplete');
    const tabCountAll = document.getElementById('tabCountAll');
    const tabCountComplete = document.getElementById('tabCountComplete');
    const shelfSortSelect = document.getElementById('shelfSortSelect');

    // Modals
    const redoModal = document.getElementById('redoModal');
    const redoModalTitle = document.getElementById('redoModalTitle');
    const closeRedoModal = document.getElementById('closeRedoModal');
    const redoScrapeBtn = document.getElementById('redoScrapeBtn');
    const redoCleanBtn = document.getElementById('redoCleanBtn');
    const redoForceBtn = document.getElementById('redoForceBtn');
    const redoLoadTopBtn = document.getElementById('redoLoadTopBtn');

    const detailsModal = document.getElementById('detailsModal');
    const closeDetailsModal = document.getElementById('closeDetailsModal');
    const detailsCover = document.getElementById('detailsCover');
    const detailsNovelTitle = document.getElementById('detailsNovelTitle');
    const detailsAuthor = document.getElementById('detailsAuthor');
    const detailsDesc = document.getElementById('detailsDesc');
    const detailsChapterBadge = document.getElementById('detailsChapterBadge');
    const detailsCatalogLink = document.getElementById('detailsCatalogLink');
    const detailsReadFirstLink = document.getElementById('detailsReadFirstLink');
    const chapterFilterInput = document.getElementById('chapterFilterInput');
    const chapterListContainer = document.getElementById('chapterListContainer');

    // SaaS Modal & Navigation Elements
    const cmdPaletteBtn = document.getElementById('cmdPaletteBtn');
    const commandPaletteModal = document.getElementById('commandPaletteModal');
    const cmdInput = document.getElementById('cmdInput');
    const cmdResults = document.getElementById('cmdResults');

    const shortcutsBtn = document.getElementById('shortcutsBtn');
    const shortcutsModal = document.getElementById('shortcutsModal');
    const closeShortcutsModal = document.getElementById('closeShortcutsModal');

    const soundToggleBtn = document.getElementById('soundToggleBtn');
    const soundIconOn = document.getElementById('soundIconOn');
    const soundIconOff = document.getElementById('soundIconOff');

    const themeToggle = document.getElementById('themeToggle');
    const toast = document.getElementById('toast');

    // State
    let currentMetadata = null;
    let sseSource = null;
    let loadedNovels = [];
    let activeRedoNovel = null;
    let currentActiveSlug = null;
    let currentFilter = 'all';
    let currentSort = 'recent';
    let activeNovelDetails = null;

    function escapeHtml(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }

    // ==========================================
    // Theme Manager
    // ==========================================
    const savedTheme = localStorage.getItem('novel_theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);

    themeToggle.addEventListener('click', () => {
        const cur = document.documentElement.getAttribute('data-theme');
        const next = cur === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', next);
        localStorage.setItem('novel_theme', next);
    });

    // ==========================================
    // Options Toggle & Clear Inputs
    // ==========================================
    toggleOptionsBtn.addEventListener('click', () => {
        const isHidden = optionsPanel.classList.toggle('hidden');
        toggleOptionsBtn.setAttribute('aria-expanded', !isHidden);
    });

    if (clearUrlBtn) {
        clearUrlBtn.addEventListener('click', () => {
            novelUrlInput.value = '';
            previewCard.classList.add('hidden');
            currentMetadata = null;
            novelUrlInput.focus();
            showToast("Intake cleared.");
        });
    }

    if (clearConsoleBtn) {
        clearConsoleBtn.addEventListener('click', () => {
            logConsole.innerHTML = '<div class="log-entry">[Stream cleared]</div>';
            showToast("Technical stream cleared.");
        });
    }

    toggleConsoleBtn.addEventListener('click', () => {
        logConsole.classList.toggle('hidden');
        toggleConsoleBtn.textContent = logConsole.classList.contains('hidden') ? 'Show' : 'Hide';
    });

    if (completionDismissBtn) {
        completionDismissBtn.addEventListener('click', () => {
            completionBanner.classList.add('hidden');
        });
    }

    // ==========================================
    // Helper: Toast & Log
    // ==========================================
    let toastTimeout;
    function showToast(message) {
        toast.textContent = message;
        toast.classList.remove('hidden');
        toast.style.opacity = '1';

        clearTimeout(toastTimeout);
        toastTimeout = setTimeout(() => {
            toast.style.opacity = '0';
            setTimeout(() => toast.classList.add('hidden'), 250);
        }, 3500);
    }

    function appendLog(line) {
        const entry = document.createElement('div');
        entry.className = 'log-entry';
        entry.textContent = line;
        logConsole.appendChild(entry);
        logConsole.scrollTop = logConsole.scrollHeight;
    }

    // ==========================================
    // Pipeline Stepper Controller
    // ==========================================
    const steps = ['discovering', 'scraping', 'cleaning', 'indexing', 'complete'];

    function setPipelineStage(stageName) {
        const stageIndex = steps.indexOf(stageName);
        if (stageIndex === -1) return;

        steps.forEach((s, idx) => {
            const stepEl = document.getElementById(`step-${s}`);
            if (!stepEl) return;

            if (idx < stageIndex) {
                stepEl.className = 'step-item completed';
            } else if (idx === stageIndex) {
                stepEl.className = 'step-item active';
            } else {
                stepEl.className = 'step-item';
            }

            // Connector lines
            const connector = stepEl.previousElementSibling;
            if (connector && connector.classList.contains('step-connector')) {
                connector.classList.toggle('completed', idx <= stageIndex);
            }
        });

        // Stage Badge
        stageBadge.className = `stage-badge ${stageName}`;
        if (stageName === 'discovering') stageBadge.textContent = 'STAGE 1: DISCOVERY';
        else if (stageName === 'scraping') stageBadge.textContent = 'STAGE 2: DOWNLOADING';
        else if (stageName === 'cleaning') stageBadge.textContent = 'STAGE 3: TYPESETTING';
        else if (stageName === 'indexing') stageBadge.textContent = 'STAGE 4: COMPILATION';
        else if (stageName === 'complete') stageBadge.textContent = 'STAGE 5: BOUND & READY';

        // Progress bar styling variant
        progressBar.classList.remove('cleaning', 'done');
        if (stageName === 'cleaning') progressBar.classList.add('cleaning');
        if (stageName === 'complete') progressBar.classList.add('done');
    }

    // Append chip to live chapter ticker
    function addTickerChip(number, title, status) {
        if (!chapterTicker) return;

        // Clear placeholder if present
        const emptyMsg = chapterTicker.querySelector('.ticker-empty');
        if (emptyMsg) emptyMsg.remove();

        const chip = document.createElement('div');
        chip.className = 'ticker-chip';

        const statusClass = status === 'ok' ? 'ok' : (status === 'cached' ? 'cached' : 'cleaned');
        const statusLabel = status === 'ok' ? 'DOWNLOADED' : (status === 'cached' ? 'CACHED' : 'TYPESET');

        chip.innerHTML = `
            <span class="chip-status ${statusClass}">${statusLabel}</span>
            <span class="chip-text">Ch. ${number}: ${escapeHtml(title || 'Chapter')}</span>
        `;

        chapterTicker.insertBefore(chip, chapterTicker.firstChild);

        // Keep at most 8 chips
        while (chapterTicker.children.length > 8) {
            chapterTicker.removeChild(chapterTicker.lastChild);
        }
    }

    // ==========================================
    // Native Desktop API & Bridge
    // ==========================================
    const DesktopAPI = {
        async ready() {
            if (window.go?.main?.App) return true;
            for (let i = 0; i < 25; i++) {
                await new Promise(r => setTimeout(r, 40));
                if (window.go?.main?.App) return true;
            }
            return false;
        },
        async getNovels() {
            await this.ready();
            if (window.go?.main?.App?.GetNovels) {
                try {
                    return (await window.go.main.App.GetNovels()) || [];
                } catch (e) {
                    console.error("GetNovels error:", e);
                    return [];
                }
            }
            try {
                const res = await fetch('/api/novels');
                if (!res.ok) return [];
                const data = await res.json();
                return data.novels || [];
            } catch (e) {
                return [];
            }
        },
        async getNovel(slug) {
            await this.ready();
            if (window.go?.main?.App?.GetNovel) {
                try {
                    return await window.go.main.App.GetNovel(slug);
                } catch (e) {
                    console.error("GetNovel error:", e);
                    return null;
                }
            }
            try {
                const res = await fetch(`/api/novels/${slug}`);
                if (!res.ok) return null;
                return await res.json();
            } catch (e) {
                return null;
            }
        },
        async getSystemStats() {
            await this.ready();
            if (window.go?.main?.App?.GetSystemStats) {
                try {
                    return await window.go.main.App.GetSystemStats();
                } catch (e) {
                    console.error("GetSystemStats error:", e);
                    return null;
                }
            }
            try {
                const res = await fetch('/api/system/stats');
                if (!res.ok) return null;
                return await res.json();
            } catch (e) {
                return null;
            }
        },
        async discover(url) {
            if (window.go?.main?.App?.DiscoverNovel) {
                const meta = await window.go.main.App.DiscoverNovel(url);
                return { success: true, metadata: meta };
            }
            const res = await fetch('/api/discover', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url })
            });
            return await res.json();
        },
        async startScrape(options) {
            if (window.go?.main?.App?.StartScrape) {
                const opts = {
                    url: options.url,
                    startChapter: parseInt(options.startChapter, 10) || 0,
                    endChapter: parseInt(options.endChapter, 10) || 0,
                    concurrency: parseInt(options.concurrency, 10) || 4,
                    skipImages: !!options.skipImages,
                    fromPasted: !!options.fromPasted,
                    force: !!options.force
                };
                return await window.go.main.App.StartScrape(opts);
            }
            const res = await fetch('/api/scrape', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(options)
            });
            return await res.json();
        },
        async cleanNovel(slug) {
            if (window.go?.main?.App?.CleanNovel) {
                return await window.go.main.App.CleanNovel(slug);
            }
            const res = await fetch(`/api/novels/${slug}/clean`, { method: 'POST' });
            return await res.json();
        },
        async auditNovel(slug) {
            if (window.go?.main?.App?.AuditNovel) {
                return await window.go.main.App.AuditNovel(slug);
            }
            const res = await fetch(`/api/novels/${slug}/audit`, { method: 'POST' });
            return await res.json();
        },
        async retryNovel(slug) {
            if (window.go?.main?.App?.RetryNovel) {
                return await window.go.main.App.RetryNovel(slug);
            }
            const res = await fetch(`/api/novels/${slug}/retry`, { method: 'POST' });
            return await res.json();
        },
        async deleteNovel(slug) {
            if (window.go?.main?.App?.DeleteNovel) {
                return await window.go.main.App.DeleteNovel(slug);
            }
            const res = await fetch(`/api/novels/${slug}`, { method: 'DELETE' });
            return await res.json();
        },
        async cancelTask() {
            if (window.go?.main?.App?.CancelTask) {
                return await window.go.main.App.CancelTask();
            }
        }
    };

    // ==========================================
    // Real-Time Event Consumer (Wails runtime & fallback)
    // ==========================================
    function setupEventListeners() {
        const handleStarted = (data) => {
            if (typeof data === 'string') { try { data = JSON.parse(data); } catch(e){} }
            currentActiveSlug = data.slug;
            progressSection.classList.remove('hidden');
            completionBanner.classList.add('hidden');
            progressNovelTitle.textContent = `Archiving: ${data.title}`;
            progressBar.style.width = '0%';
            progressPercent.textContent = '0%';
            setPipelineStage(data.stage || 'discovering');
            progressCurrentStatus.textContent = 'Connecting to source manifest...';
            appendLog(`[Started] Task initialized for: ${data.title}`);
            progressSection.scrollIntoView({ behavior: 'smooth' });
            renderLibraryCards(loadedNovels);
        };

        const handleStatus = (data) => {
            if (typeof data === 'string') { try { data = JSON.parse(data); } catch(e){} }
            progressCurrentStatus.textContent = data.message;
            appendLog(`[Status] ${data.message}`);
        };

        const handleProgress = (data) => {
            if (typeof data === 'string') { try { data = JSON.parse(data); } catch(e){} }
            progressSection.classList.remove('hidden');
            progressNovelTitle.textContent = `Archiving: ${data.novelTitle || data.title || data.slug || ''}`;

            if (data.stage === 'discovering') {
                setPipelineStage('discovering');
                progressBar.style.width = `${data.percentage || 15}%`;
                progressPercent.textContent = `${data.percentage || 15}%`;
                progressCurrentStatus.textContent = data.message || 'Extracting novel table of contents...';
            } else if (data.stage === 'scraping') {
                setPipelineStage('scraping');
                const percent = data.percentage || 0;
                progressBar.style.width = `${percent}%`;
                progressPercent.textContent = `${percent}%`;
                progressCounts.textContent = `${data.current} / ${data.total}`;
                progressCurrentStatus.textContent = `Downloading Ch. ${data.chapterNum}: ${data.chapterTitle || ''}`;
                
                if (data.speed) progressSpeed.textContent = data.speed;
                if (data.scraped !== undefined) statDownloaded.textContent = data.scraped;
                if (data.cached !== undefined) statCached.textContent = data.cached;
                if (data.eta) statEta.textContent = data.eta;

                if (data.chapterNum) {
                    addTickerChip(data.chapterNum, data.chapterTitle, data.status);
                    appendLog(`[Ch. ${String(data.chapterNum).padStart(4, '0')}] ${data.status === 'ok' ? 'Saved' : 'Cached'} — ${data.chapterTitle || ''}`);
                }
            } else if (data.stage === 'cleaning') {
                setPipelineStage('cleaning');
                const percent = data.percentage || 0;
                progressBar.style.width = `${percent}%`;
                progressPercent.textContent = `${percent}%`;
                progressCounts.textContent = `Typeset ${data.current} / ${data.total}`;
                progressCurrentStatus.textContent = `Formatting Ch. ${data.chapterNum}: ${data.chapterTitle || ''}`;
                
                if (data.cleaned !== undefined) statCleaned.textContent = data.cleaned;
                if (data.chapterNum) {
                    addTickerChip(data.chapterNum, data.chapterTitle, 'cleaned');
                    appendLog(`[Typeset] Ch. ${data.chapterNum}: ${data.chapterTitle || ''}`);
                }
            } else if (data.stage === 'indexing') {
                setPipelineStage('indexing');
                progressBar.style.width = '100%';
                progressPercent.textContent = '100%';
                progressCurrentStatus.textContent = data.message || 'Building master reader catalog...';
                appendLog(`[Catalog] Compiling searchable master index.`);
            }
        };

        const handleDone = (data) => {
            if (typeof data === 'string') { try { data = JSON.parse(data); } catch(e){} }
            currentActiveSlug = null;
            setPipelineStage('complete');
            progressBar.style.width = '100%';
            progressPercent.textContent = '100%';
            progressCurrentStatus.textContent = 'Offline volume bound and catalog compiled.';
            appendLog(`[Finished] "${data.title}" successfully archived.`);

            if (completionBanner) {
                completionTitle.textContent = `"${data.title}" Successfully Bound!`;
                const extra = [data.failed ? `${data.failed} failed (press Update to retry)` : '', data.locked ? `${data.locked} paid/locked skipped` : ''].filter(Boolean).join(', ');
                completionDesc.textContent = `${data.ok || 0} chapters saved for offline reading.` + (extra ? ` ${extra}.` : '');
                completionReadLink.href = data.catalogUrl || `/library/${data.slug}/`;
                completionBanner.classList.remove('hidden');
                completionBanner.scrollIntoView({ behavior: 'smooth' });
            }

            showToast(`"${data.title}" is bound and ready for offline reading.`);
            playCompletionChime();
            loadLibrary();
            refreshSystemStats();
        };

        const handleError = (data) => {
            if (typeof data === 'string') { try { data = JSON.parse(data); } catch(e){} }
            showToast(`Archival error: ${data.message || data}`);
            appendLog(`[Error] ${data.message || data}`);
        };

        const hookRuntime = () => {
            if (window.runtime && window.runtime.EventsOn) {
                window.runtime.EventsOn('started', handleStarted);
                window.runtime.EventsOn('status', handleStatus);
                window.runtime.EventsOn('progress', handleProgress);
                window.runtime.EventsOn('done', handleDone);
                window.runtime.EventsOn('error', handleError);
                window.runtime.EventsOn('challenge', (data) => {
                    progressCurrentStatus.textContent = data.message;
                    appendLog(`[Action needed] ${data.message}`);
                    showToast(data.message);
                });
                appendLog('[System] Wails native IPC event bus active.');
                return true;
            }
            return false;
        };

        if (!hookRuntime()) {
            let attempts = 0;
            const timer = setInterval(() => {
                attempts++;
                if (hookRuntime() || attempts > 20) {
                    clearInterval(timer);
                    if (!window.runtime) {
                        try {
                            const sse = new EventSource('/api/scrape/stream');
                            sse.addEventListener('started', (e) => handleStarted(JSON.parse(e.data)));
                            sse.addEventListener('status', (e) => handleStatus(JSON.parse(e.data)));
                            sse.addEventListener('progress', (e) => handleProgress(JSON.parse(e.data)));
                            sse.addEventListener('done', (e) => handleDone(JSON.parse(e.data)));
                            sse.addEventListener('error', (e) => handleError(e.data ? JSON.parse(e.data) : 'Stream error'));
                        } catch(err) {}
                    }
                }
            }, 100);
        }
    }

    setupEventListeners();

    const cancelTaskBtn = document.getElementById('cancelTaskBtn');
    if (cancelTaskBtn) {
        cancelTaskBtn.addEventListener('click', async () => {
            showToast("Cancelling archival task...");
            await DesktopAPI.cancelTask();
        });
    }

    // ==========================================
    // Inspect / Preview
    // ==========================================
    async function inspectNovel(url) {
        if (!url) {
            showToast("Please provide a novel URL to inspect.");
            return;
        }

        previewBtn.disabled = true;
        previewBtn.querySelector('span').textContent = "Inspecting...";

        try {
            const data = await DesktopAPI.discover(url);
            if (!data.success || !data.metadata) {
                throw new Error(data.error || "Could not discover novel metadata from this link.");
            }

            currentMetadata = data.metadata;
            previewTitle.textContent = currentMetadata.title;
            previewAuthor.textContent = `By ${currentMetadata.author || 'Unknown Author'}`;
            const site = currentMetadata.siteName ? ` · ${currentMetadata.siteName}` : '';
            previewChapters.textContent = currentMetadata.tocMode === 'crawl'
                ? `Chapters are found while downloading${site}`
                : `${currentMetadata.totalChapters} chapters${site}` + (currentMetadata.downloaded ? ` · ${currentMetadata.downloaded} already saved` : '');
            const warnEl = document.getElementById('previewWarnings');
            const warns = currentMetadata.warnings || [];
            warnEl.innerHTML = warns.map(w => `<li>${escapeHtml(w)}</li>`).join('');
            warnEl.classList.toggle('hidden', warns.length === 0);
            document.getElementById('openSiteBrowserBtn').classList.toggle('hidden', !window.go?.main?.App?.OpenSiteBrowser || warns.length === 0);
            const fromBtn = document.getElementById('startFromChapterBtn');
            const fromCh = currentMetadata.inputKind === 'chapter' ? (currentMetadata.inputChapter || 0) : 0;
            fromBtn.classList.toggle('hidden', fromCh <= 1);
            document.getElementById('startFromChapterLabel').textContent = `From chapter ${fromCh} onward`;
            document.getElementById('startFromPreviewLabel').textContent = currentMetadata.existingSlug ? 'Download missing & new chapters' : 'Download all chapters';
            previewDesc.textContent = currentMetadata.description || "No synopsis available for this title.";
            
            const fallbackCover = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="120" height="175"><rect width="100%" height="100%" fill="%23222933"/><text x="50%" y="50%" fill="%23a3acb9" font-family="serif" text-anchor="middle">Book</text></svg>';
            previewCover.src = currentMetadata.cover || fallbackCover;
            previewCover.alt = `Cover art for ${currentMetadata.title}`;

            previewCard.classList.remove('hidden');
            showToast("Novel metadata discovered.");
        } catch (err) {
            showToast(`Inspection failed: ${err.message}`);
        } finally {
            previewBtn.disabled = false;
            previewBtn.querySelector('span').textContent = "Inspect";
        }
    }

    previewBtn.addEventListener('click', () => {
        inspectNovel(novelUrlInput.value.trim());
    });

    novelUrlInput.addEventListener('paste', () => {
        setTimeout(() => {
            const val = novelUrlInput.value.trim();
            if (val.startsWith('http')) inspectNovel(val);
        }, 120);
    });

    // ==========================================
    // Start Scraping (Instant Zero-Lag Feedback)
    // ==========================================
    async function startScrape(targetUrl, extraOptions = {}) {
        if (!targetUrl) {
            showToast("Please enter a novel URL.");
            return;
        }

        const payload = {
            url: targetUrl,
            startChapter: extraOptions.startChapter !== undefined ? extraOptions.startChapter : (document.getElementById('startChapter').value || null),
            endChapter: extraOptions.endChapter !== undefined ? extraOptions.endChapter : (document.getElementById('endChapter').value || null),
            concurrency: extraOptions.concurrency !== undefined ? extraOptions.concurrency : (document.getElementById('concurrency').value || 4),
            skipImages: document.getElementById('skipImages').checked,
            fromPasted: !!extraOptions.fromPasted,
            force: extraOptions.force || false
        };

        const submitBtn = document.getElementById('submitBtn');
        submitBtn.disabled = true;
        submitBtn.querySelector('span').textContent = "Starting...";

        // INSTANT UI FEEDBACK: Immediately unhide progress monitor!
        progressSection.classList.remove('hidden');
        completionBanner.classList.add('hidden');
        setPipelineStage('discovering');
        progressBar.style.width = '10%';
        progressPercent.textContent = '10%';
        progressNovelTitle.textContent = currentMetadata ? `Archiving: ${currentMetadata.title}` : `Archiving Novel...`;
        progressCurrentStatus.textContent = 'Initiating archival task & connecting to source...';
        progressCounts.textContent = 'Connecting...';
        progressSection.scrollIntoView({ behavior: 'smooth' });

        try {
            const data = await DesktopAPI.startScrape(payload);
            if (data.error) {
                showToast(`Unable to start: ${data.error}`);
            } else {
                currentActiveSlug = data.slug;
                showToast("Archival pipeline initiated.");
            }
        } catch (err) {
            showToast(`Failed to start: ${err.message}`);
        } finally {
            submitBtn.disabled = false;
            submitBtn.querySelector('span').textContent = "Archive Novel";
        }
    }

    scrapeForm.addEventListener('submit', (e) => {
        e.preventDefault();
        startScrape(novelUrlInput.value.trim());
    });

    startFromPreviewBtn.addEventListener('click', () => {
        if (currentMetadata) {
            startScrape(currentMetadata.url || novelUrlInput.value.trim());
        }
    });

    // ==========================================
    // Load & Render Library Shelf
    // ==========================================
    async function loadLibrary() {
        try {
            const novels = await DesktopAPI.getNovels();
            loadedNovels = Array.isArray(novels) ? novels : (novels.novels || []);
            loadedNovels = novels;

            // Update Tab Counts
            if (tabCountAll) tabCountAll.textContent = novels.length;
            if (tabCountComplete) {
                const completeCount = novels.filter(n => n.hasIndex && (n.cleanCount >= (n.totalChapters || 1))).length;
                tabCountComplete.textContent = completeCount;
            }

            renderLibraryCards(novels);
            refreshSystemStats();
        } catch (err) {
            libraryGrid.innerHTML = `<div class="empty-state">Failed to load library: ${escapeHtml(err.message)}</div>`;
        }
    }

    function renderLibraryCards(novels) {
        let list = [...novels];

        // Apply Tab Filter
        if (currentFilter === 'complete') {
            list = list.filter(n => n.hasIndex && (n.cleanCount >= (n.totalChapters || 1)));
        }

        // Apply Search Filter
        const query = (librarySearch.value || '').toLowerCase().trim();
        if (query) {
            list = list.filter(n => 
                n.title.toLowerCase().includes(query) || (n.author && n.author.toLowerCase().includes(query))
            );
        }

        // Apply Sorting
        if (currentSort === 'chapters') {
            list.sort((a, b) => (b.cleanCount || b.totalChapters || 0) - (a.cleanCount || a.totalChapters || 0));
        } else if (currentSort === 'title') {
            list.sort((a, b) => a.title.localeCompare(b.title));
        }

        if (list.length === 0) {
            libraryGrid.innerHTML = `
                <div class="empty-state">
                    <h3>No volumes match your criteria</h3>
                    <p>${query ? `No archived titles match "${escapeHtml(query)}".` : 'No archived volumes found in this view.'}</p>
                </div>`;
            return;
        }

        libraryGrid.innerHTML = list.map(novel => {
            const coverSrc = novel.cover || 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="85" height="128"><rect width="100%" height="100%" fill="%23222933"/><text x="50%" y="50%" fill="%23a3acb9" font-family="serif" text-anchor="middle">Book</text></svg>';
            const safeTitle = escapeHtml(novel.title);
            const safeAuthor = escapeHtml(novel.author || 'Unknown Author');
            const total = novel.totalChapters || novel.cleanCount || novel.rawCount || 0;
            const bound = novel.cleanCount || novel.rawCount || 0;
            const isComplete = novel.hasIndex && bound >= total && total > 0;
            const percent = total > 0 ? Math.min(100, Math.round((bound / total) * 100)) : 0;
            const isCurrentlyActive = currentActiveSlug === novel.slug;

            // Reading Progress check
            let continueBtn = '';
            try {
                const chNum = parseInt(localStorage.getItem('na-last-' + novel.slug), 10);
                if (chNum) {
                    {
                        continueBtn = `
                            <a href="/library/${escapeHtml(novel.slug)}/${chNum}" class="btn-card btn-continue" title="Resume reading chapter ${chNum}">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                                <span>Continue Ch. ${chNum}</span>
                            </a>`;
                    }
                }
            } catch (e) {}

            return `
            <article class="novel-card" data-slug="${escapeHtml(novel.slug)}">
                ${isCurrentlyActive ? `<div class="card-active-indicator">⚡ ACTIVE ARCHIVAL IN PROGRESS</div>` : ''}
                <div class="novel-card-content">
                    <img src="${coverSrc}" alt="Cover of ${safeTitle}" class="novel-card-cover" loading="lazy">
                    <div class="novel-card-info">
                        <button type="button" class="novel-card-title-btn" onclick="openDetailsModal('${escapeHtml(novel.slug)}')">
                            <h3 class="novel-card-title">${safeTitle}</h3>
                        </button>
                        <div class="novel-card-author">By ${safeAuthor}</div>
                        <div class="card-meta-row">
                            <span class="novel-card-badge ${isComplete ? 'complete' : ''}">
                                ${isComplete ? `✓ ${bound} chapters` : `${bound} / ${total} chapters`}
                            </span>
                            <span class="novel-card-badge">${escapeHtml(novel.sizeMb || '')}</span>
                            ${novel.failedCount ? `<span class="novel-card-badge">${novel.failedCount} failed</span>` : ''}
                        </div>
                    </div>
                </div>
                <div class="novel-card-progress" title="${percent}% Complete">
                    <div class="novel-card-progress-fill" style="width: ${percent}%"></div>
                </div>
                <div class="novel-card-actions">
                    ${continueBtn ? continueBtn : ''}
                    <a href="${novel.catalogUrl}" class="btn-card btn-read" title="Open master reading catalog">
                        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>
                        <span>${continueBtn ? 'Catalog' : 'Read'}</span>
                    </a>
                    <button type="button" onclick="openDetailsModal('${escapeHtml(novel.slug)}')" class="btn-card btn-sub" title="Inspect table of contents & chapter directory">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
                        <span>Explore</span>
                    </button>
                    <button type="button" onclick="openRedoModal('${escapeHtml(novel.slug)}')" class="btn-card btn-redo" title="Re-do, sync, or re-clean this volume">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></svg>
                        <span>Re-do</span>
                    </button>
                    <button type="button" onclick="cleanNovel('${escapeHtml(novel.slug)}')" class="btn-card btn-sub" title="Download new and missing chapters">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m14 12-8.5 8.5a2.12 2.12 0 1 1-3-3L11 9"/><path d="M18 2a3 3 0 0 0-3 3l1 1-7 7 4 4 7-7 1 1a3 3 0 0 0 3-3V2z"/></svg>
                        <span>Update</span>
                    </button>
                    <button type="button" onclick="deleteNovel('${escapeHtml(novel.slug)}', '${safeTitle}')" class="btn-card btn-delete" title="Permanently delete this volume">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/></svg>
                        <span>Delete</span>
                    </button>
                </div>
            </article>`;
        }).join('');
    }

    // Filter Tabs
    tabFilterAll.addEventListener('click', () => {
        currentFilter = 'all';
        tabFilterAll.classList.add('active');
        tabFilterComplete.classList.remove('active');
        renderLibraryCards(loadedNovels);
    });

    tabFilterComplete.addEventListener('click', () => {
        currentFilter = 'complete';
        tabFilterComplete.classList.add('active');
        tabFilterAll.classList.remove('active');
        renderLibraryCards(loadedNovels);
    });

    // Sort Dropdown
    shelfSortSelect.addEventListener('change', (e) => {
        currentSort = e.target.value;
        renderLibraryCards(loadedNovels);
    });

    // Search Input
    librarySearch.addEventListener('input', () => {
        const query = librarySearch.value.trim();
        if (clearSearchBtn) clearSearchBtn.classList.toggle('hidden', !query);
        renderLibraryCards(loadedNovels);
    });

    if (clearSearchBtn) {
        clearSearchBtn.addEventListener('click', () => {
            librarySearch.value = '';
            clearSearchBtn.classList.add('hidden');
            renderLibraryCards(loadedNovels);
            librarySearch.focus();
        });
    }

    // ==========================================
    // Volume Inspector & Chapter Explorer Modal
    // ==========================================
    window.openDetailsModal = async (slug) => {
        const novel = loadedNovels.find(n => n.slug === slug);
        if (!novel) return;

        detailsNovelTitle.textContent = novel.title;
        detailsAuthor.textContent = `By ${novel.author || 'Unknown Author'}`;
        detailsDesc.textContent = novel.description || 'No synopsis available.';
        detailsChapterBadge.textContent = `${novel.cleanCount} / ${novel.totalChapters} chapters saved`;
        detailsCatalogLink.href = novel.catalogUrl;
        detailsReadFirstLink.href = `/library/${slug}/1`;
        document.getElementById('detailsSourceBadge').textContent = novel.site || 'Source';
        document.getElementById('detailsSize').textContent = novel.sizeMb || '';

        const fallbackCover = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="120" height="175"><rect width="100%" height="100%" fill="%23222933"/><text x="50%" y="50%" fill="%23a3acb9" font-family="serif" text-anchor="middle">Book</text></svg>';
        detailsCover.src = novel.cover || fallbackCover;

        detailsModal.classList.remove('hidden');
        chapterListContainer.innerHTML = '<div class="loading-spinner">Loading chapter directory...</div>';

        try {
            const data = await DesktopAPI.getNovel(slug);
            activeNovelDetails = data;
            renderChapterList(data.chapters || []);
        } catch (e) {
            chapterListContainer.innerHTML = `<div class="empty-state">Failed to load chapter directory: ${escapeHtml(e.message)}</div>`;
        }
    };

    function renderChapterList(chapters) {
        const query = (chapterFilterInput.value || '').toLowerCase().trim();
        const filtered = chapters.filter(c => 
            !query || String(c.number).includes(query) || (c.title && c.title.toLowerCase().includes(query))
        );

        if (filtered.length === 0) {
            chapterListContainer.innerHTML = '<div class="empty-state">No chapters match filter.</div>';
            return;
        }

        chapterListContainer.innerHTML = filtered.slice(0, 300).map(c => `
            <div class="chapter-row-item">
                <div class="chapter-row-title">
                    <span class="chapter-row-num">#${c.number}</span>
                    <span>${escapeHtml(c.title || `Chapter ${c.number}`)}</span>
                </div>
                ${c.isClean ? `
                    <a href="${c.readerUrl}" class="btn-read-ch">Read</a>
                ` : `<span class="badge-subtle" title="${escapeHtml(c.error || '')}">${c.state === 'locked' ? 'Paid' : c.state === 'failed' ? 'Failed' : 'Not saved'}</span>`}
            </div>
        `).join('');
    }

    chapterFilterInput.addEventListener('input', () => {
        if (activeNovelDetails && activeNovelDetails.chapters) {
            renderChapterList(activeNovelDetails.chapters);
        }
    });

    closeDetailsModal.addEventListener('click', () => {
        detailsModal.classList.add('hidden');
    });

    detailsModal.addEventListener('click', (e) => {
        if (e.target === detailsModal) detailsModal.classList.add('hidden');
    });

    // ==========================================
    // Actions: Clean, Audit, Delete
    // ==========================================
    window.cleanNovel = async (slug) => {
        showToast(`Checking for new and missing chapters…`);
        
        // Unhide progress monitor immediately so the user SEES the cleaning stage running!
        progressSection.classList.remove('hidden');
        completionBanner.classList.add('hidden');
        setPipelineStage('discovering');
        progressNovelTitle.textContent = `Updating: ${slug}`;
        progressBar.style.width = '0%';
        progressPercent.textContent = '0%';
        progressCurrentStatus.textContent = `Reading the chapter list…`;
        progressSection.scrollIntoView({ behavior: 'smooth' });

        try {
            const data = await DesktopAPI.cleanNovel(slug);
            showToast(`Update started.`);
        } catch (e) {
            showToast(`Clean failed: ${e.message}`);
        }
    };

    window.auditNovel = async (slug) => {
        showToast(`Auditing volume ${slug}...`);
        try {
            const data = await DesktopAPI.auditNovel(slug);
            if (data.invalidCount === 0) {
                alert(`✓ Verified: All ${data.validCount} chapters in this volume are verified and valid with zero corruption.`);
            } else {
                const retryNow = confirm(`Found ${data.invalidCount} challenged or missing chapters.\n\nWould you like NovelArchiver to auto-retry fetching them now?`);
                if (retryNow) {
                    await DesktopAPI.retryNovel(slug);
                    showToast("Retrying corrupted chapters...");
                }
            }
        } catch (e) {
            showToast(`Audit failed: ${e.message}`);
        }
    };

    window.deleteNovel = async (slug, title) => {
        const confirmDelete = confirm(`⚠️ Permanently Delete Volume:\n\nAre you sure you want to remove "${title}" from your library?\n\nThis deletes every saved chapter and image of this novel from disk. It cannot be undone.`);
        if (!confirmDelete) return;

        showToast(`Removing "${title}" from library...`);
        try {
            const data = await DesktopAPI.deleteNovel(slug);
            if (data.success) {
                showToast(`Volume "${title}" was permanently removed.`);
                loadLibrary();
            } else {
                showToast(`Deletion failed: ${data.error || 'Unknown error'}`);
            }
        } catch (err) {
            showToast(`Error: ${err.message}`);
        }
    };

    // ==========================================
    // Re-do Modal Handlers
    // ==========================================
    window.openRedoModal = (slug) => {
        const novel = loadedNovels.find(n => n.slug === slug);
        if (!novel) return;
        activeRedoNovel = novel;
        redoModalTitle.textContent = `Re-do: ${novel.title}`;
        redoModal.classList.remove('hidden');
    };

    closeRedoModal.addEventListener('click', () => {
        redoModal.classList.add('hidden');
    });

    redoModal.addEventListener('click', (e) => {
        if (e.target === redoModal) redoModal.classList.add('hidden');
    });

    redoScrapeBtn.addEventListener('click', () => {
        if (!activeRedoNovel) return;
        redoModal.classList.add('hidden');
        const targetUrl = activeRedoNovel.url || activeRedoNovel.slug;
        startScrape(targetUrl, { force: false });
    });

    redoCleanBtn.addEventListener('click', () => {
        if (!activeRedoNovel) return;
        redoModal.classList.add('hidden');
        cleanNovel(activeRedoNovel.slug);
    });

    redoForceBtn.addEventListener('click', () => {
        if (!activeRedoNovel) return;
        const confirmForce = confirm(`Force Full Re-Download:\n\nThis will re-download every single chapter of "${activeRedoNovel.title}" fresh from the web and overwrite local files.\n\nAre you sure you wish to proceed?`);
        if (confirmForce) {
            redoModal.classList.add('hidden');
            const targetUrl = activeRedoNovel.url || activeRedoNovel.slug;
            startScrape(targetUrl, { force: true });
        }
    });

    redoLoadTopBtn.addEventListener('click', () => {
        if (!activeRedoNovel) return;
        redoModal.classList.add('hidden');
        const targetUrl = activeRedoNovel.url || activeRedoNovel.slug;
        novelUrlInput.value = targetUrl;
        optionsPanel.classList.remove('hidden');
        inspectNovel(targetUrl);
        novelUrlInput.scrollIntoView({ behavior: 'smooth' });
        showToast(`Loaded "${activeRedoNovel.title}" into Archival Settings.`);
    });

    // ==========================================
    // System Repository Statistics
    // ==========================================
    async function refreshSystemStats() {
        try {
            const data = await DesktopAPI.getSystemStats();
            if (data) {
                const elVolumes = document.getElementById('stripVolumes');
                const elChapters = document.getElementById('stripChapters');
                const elDisk = document.getElementById('stripDisk');
                if (elVolumes) elVolumes.textContent = (data.totalNovels || 0).toLocaleString();
                if (elChapters) elChapters.textContent = ((data.totalCleanChapters || data.totalRawChapters || 0)).toLocaleString();
                const sizeStr = data.sizeMb ? (String(data.sizeMb).includes('MB') ? data.sizeMb : `${data.sizeMb} MB`) : '0 MB';
                if (elDisk) elDisk.textContent = `~${sizeStr}`;
            }
        } catch (e) {
            console.error("Failed to load system stats:", e);
        }
    }

    // ==========================================
    // Concurrency Performance Presets
    // ==========================================
    document.querySelectorAll('.preset-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.preset-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            const workers = btn.getAttribute('data-workers');
            const concurrencyInput = document.getElementById('concurrency');
            if (concurrencyInput) concurrencyInput.value = workers;
            showToast(`Concurrency preset: ${workers} parallel workers`);
        });
    });

    // ==========================================
    // Web Audio Synthesized Chime & Sound Controls
    // ==========================================
    let soundEnabled = localStorage.getItem('novel_sound_enabled') !== 'false';
    function updateSoundUI() {
        if (soundIconOn && soundIconOff) {
            soundIconOn.classList.toggle('hidden', !soundEnabled);
            soundIconOff.classList.toggle('hidden', soundEnabled);
        }
    }
    updateSoundUI();

    if (soundToggleBtn) {
        soundToggleBtn.addEventListener('click', () => {
            soundEnabled = !soundEnabled;
            localStorage.setItem('novel_sound_enabled', soundEnabled);
            updateSoundUI();
            showToast(soundEnabled ? "Audio chime notifications enabled." : "Audio chime notifications muted.");
            if (soundEnabled) playCompletionChime();
        });
    }

    function playCompletionChime() {
        if (localStorage.getItem('novel_sound_enabled') === 'false') return;
        try {
            const AudioCtx = window.AudioContext || window.webkitAudioContext;
            if (!AudioCtx) return;
            const ctx = new AudioCtx();
            if (ctx.state === 'suspended') {
                ctx.resume();
            }
            const now = ctx.currentTime;
            const notes = [
                { freq: 523.25, time: now },       // C5
                { freq: 659.25, time: now + 0.12 }, // E5
                { freq: 783.99, time: now + 0.24 }  // G5
            ];

            notes.forEach(note => {
                const osc = ctx.createOscillator();
                const gain = ctx.createGain();
                osc.type = 'sine';
                osc.frequency.setValueAtTime(note.freq, note.time);
                gain.gain.setValueAtTime(0.001, note.time);
                gain.gain.exponentialRampToValueAtTime(0.18, note.time + 0.02);
                gain.gain.exponentialRampToValueAtTime(0.0001, note.time + 0.8);
                osc.connect(gain);
                gain.connect(ctx.destination);
                osc.start(note.time);
                osc.stop(note.time + 0.85);
            });
        } catch (e) {}
    }

    // ==========================================
    // Command Palette (Linear / Raycast Style)
    // ==========================================
    let selectedCmdIndex = 0;
    let filteredCmdItems = [];

    function buildCommandList(query = '') {
        const items = [];
        const q = query.toLowerCase().trim();

        // 1. Actions across all loaded volumes
        loadedNovels.forEach(novel => {
            const titleMatch = novel.title.toLowerCase().includes(q);
            const authorMatch = novel.author && novel.author.toLowerCase().includes(q);

            if (!q || titleMatch || authorMatch || q.includes('read') || q.includes('book') || q.includes('novel')) {
                // Continue reading option if available
                const rawLast = localStorage.getItem('novel_last_read_' + novel.slug);
                if (rawLast) {
                    try {
                        const parsed = JSON.parse(rawLast);
                        const chNum = parsed.num || parsed.chapterNum;
                        if (parsed && parsed.url && chNum) {
                            items.push({
                                type: 'volume',
                                title: `Continue: ${novel.title}`,
                                sub: `Resume Chapter ${chNum}: ${parsed.title || ''}`,
                                actionText: 'Resume',
                                action: () => { window.location.href = parsed.url; }
                            });
                        }
                    } catch (e) {}
                }

                items.push({
                    type: 'volume',
                    title: `Read ${novel.title}`,
                    sub: `Open master reading catalog (${novel.cleanCount || novel.totalChapters || 0} chapters)`,
                    actionText: 'Catalog',
                    action: () => { window.location.href = novel.catalogUrl; }
                });

                items.push({
                    type: 'volume',
                    title: `Explore Contents: ${novel.title}`,
                    sub: `Search and jump directly to any chapter`,
                    actionText: 'Explore',
                    action: () => openDetailsModal(novel.slug)
                });

                items.push({
                    type: 'volume',
                    title: `Typeset & Clean: ${novel.title}`,
                    sub: `Re-format prose and rebuild offline indexes`,
                    actionText: 'Clean',
                    action: () => cleanNovel(novel.slug)
                });
            }
        });

        // 2. Quick Global Actions
        const globalActions = [
            {
                title: 'Toggle Light / Dark Theme',
                sub: 'Switch between linen paper and midnight dark palette',
                keywords: ['theme', 'dark', 'light', 'mode', 'color'],
                actionText: 'Toggle',
                action: () => themeToggle.click()
            },
            {
                title: 'Toggle Audio Notifications',
                sub: 'Enable or mute completion chimes',
                keywords: ['sound', 'audio', 'mute', 'chime', 'bell', 'alert'],
                actionText: 'Toggle',
                action: () => soundToggleBtn.click()
            },
            {
                title: 'Focus Novel Intake URL Input',
                sub: 'Begin scraping and archiving a new web novel',
                keywords: ['url', 'new', 'scrape', 'archive', 'download', 'input', 'add'],
                actionText: 'Focus',
                action: () => {
                    novelUrlInput.focus();
                    novelUrlInput.select();
                }
            },
            {
                title: 'Toggle Archival Settings Drawer',
                sub: 'Configure concurrency, chapter range, and force mode',
                keywords: ['settings', 'options', 'concurrency', 'workers', 'config', 'drawer'],
                actionText: 'Settings',
                action: () => toggleOptionsBtn.click()
            },
            {
                title: 'Keyboard Shortcuts Help',
                sub: 'View all keyboard shortcuts and navigation hotkeys',
                keywords: ['help', 'shortcuts', 'keys', 'hotkeys', '?'],
                actionText: 'Help',
                action: () => {
                    shortcutsModal.classList.remove('hidden');
                }
            }
        ];

        globalActions.forEach(cmd => {
            const matches = !q || cmd.title.toLowerCase().includes(q) || cmd.sub.toLowerCase().includes(q) || cmd.keywords.some(k => k.includes(q));
            if (matches) {
                items.push({
                    type: 'action',
                    title: cmd.title,
                    sub: cmd.sub,
                    actionText: cmd.actionText,
                    action: cmd.action
                });
            }
        });

        return items;
    }

    function renderCommandResults(resetSelection = true) {
        filteredCmdItems = buildCommandList(cmdInput.value);
        if (resetSelection) selectedCmdIndex = 0;
        if (selectedCmdIndex >= filteredCmdItems.length) selectedCmdIndex = Math.max(0, filteredCmdItems.length - 1);

        if (filteredCmdItems.length === 0) {
            cmdResults.innerHTML = `
                <div class="empty-state" style="padding: var(--space-6); border: none;">
                    <p>No matching commands or volumes found for "${escapeHtml(cmdInput.value)}".</p>
                </div>`;
            return;
        }

        cmdResults.innerHTML = filteredCmdItems.map((item, idx) => {
            const isSel = idx === selectedCmdIndex;
            const iconSvg = item.type === 'volume'
                ? `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1-2.5-2.5Z"/></svg>`
                : `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`;

            return `
            <div class="cmd-item ${isSel ? 'selected' : ''}" data-index="${idx}">
                <div class="cmd-item-left">
                    <span class="cmd-item-icon">${iconSvg}</span>
                    <div class="cmd-item-info">
                        <span class="cmd-item-title">${escapeHtml(item.title)}</span>
                        <span class="cmd-item-sub">${escapeHtml(item.sub)}</span>
                    </div>
                </div>
                <span class="cmd-item-action">${escapeHtml(item.actionText)}</span>
            </div>`;
        }).join('');

        // Wire click handlers on result items
        cmdResults.querySelectorAll('.cmd-item').forEach(el => {
            el.addEventListener('click', () => {
                const idx = parseInt(el.getAttribute('data-index'), 10);
                if (filteredCmdItems[idx]) {
                    closeCommandPalette();
                    filteredCmdItems[idx].action();
                }
            });
        });

        // Ensure selected item is scrolled into view
        const selectedEl = cmdResults.querySelector('.cmd-item.selected');
        if (selectedEl) {
            selectedEl.scrollIntoView({ block: 'nearest' });
        }
    }

    function openCommandPalette() {
        commandPaletteModal.classList.remove('hidden');
        cmdInput.value = '';
        renderCommandResults(true);
        setTimeout(() => cmdInput.focus(), 60);
    }

    function closeCommandPalette() {
        commandPaletteModal.classList.add('hidden');
    }

    if (cmdPaletteBtn) {
        cmdPaletteBtn.addEventListener('click', openCommandPalette);
    }

    commandPaletteModal.addEventListener('click', (e) => {
        if (e.target === commandPaletteModal) closeCommandPalette();
    });

    cmdInput.addEventListener('input', () => {
        renderCommandResults(true);
    });

    cmdInput.addEventListener('keydown', (e) => {
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (filteredCmdItems.length > 0) {
                selectedCmdIndex = (selectedCmdIndex + 1) % filteredCmdItems.length;
                renderCommandResults(false);
            }
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            if (filteredCmdItems.length > 0) {
                selectedCmdIndex = (selectedCmdIndex - 1 + filteredCmdItems.length) % filteredCmdItems.length;
                renderCommandResults(false);
            }
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (filteredCmdItems[selectedCmdIndex]) {
                const item = filteredCmdItems[selectedCmdIndex];
                closeCommandPalette();
                item.action();
            }
        } else if (e.key === 'Escape') {
            closeCommandPalette();
        }
    });

    // ==========================================
    // Keyboard Shortcuts Help Modal
    // ==========================================
    if (shortcutsBtn) {
        shortcutsBtn.addEventListener('click', () => {
            shortcutsModal.classList.remove('hidden');
        });
    }

    if (closeShortcutsModal) {
        closeShortcutsModal.addEventListener('click', () => {
            shortcutsModal.classList.add('hidden');
        });
    }

    shortcutsModal.addEventListener('click', (e) => {
        if (e.target === shortcutsModal) shortcutsModal.classList.add('hidden');
    });

    // ==========================================
    // Global Keyboard Shortcuts
    // ==========================================
    document.addEventListener('keydown', (e) => {
        const isInput = ['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement?.tagName);

        // Command Palette: Ctrl+K or Cmd+K
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
            e.preventDefault();
            if (commandPaletteModal.classList.contains('hidden')) {
                openCommandPalette();
            } else {
                closeCommandPalette();
            }
            return;
        }

        // Escape: Close any open modal
        if (e.key === 'Escape') {
            commandPaletteModal.classList.add('hidden');
            shortcutsModal.classList.add('hidden');
            detailsModal.classList.add('hidden');
            redoModal.classList.add('hidden');
            return;
        }

        // Global shortcuts when not typing in an input
        if (!isInput) {
            if (e.key === '?') {
                e.preventDefault();
                shortcutsModal.classList.toggle('hidden');
            } else if (e.key === '/') {
                e.preventDefault();
                novelUrlInput.focus();
                novelUrlInput.select();
            } else if (e.key === 't' || e.key === 'T') {
                e.preventDefault();
                themeToggle.click();
            }
        }
    });

    // Safety link router: ensure all /novels/ and internal shelf links navigate in-process inside the Wails window
    document.addEventListener('click', (e) => {
        const a = e.target.closest('a');
        if (!a) return;
        const href = a.getAttribute('href');
        if (!href || href === '#' || href.startsWith('javascript:')) return;
        if (href.startsWith('/library/') || href === '/') {
            if (a.getAttribute('target') === '_blank') {
                a.removeAttribute('target');
            }
            e.preventDefault();
            window.location.href = href;
        }
    });

    // ==========================================
    // Storage management (details modal)
    // ==========================================
    document.getElementById('deleteRangeBtn').addEventListener('click', async () => {
        if (!activeNovelDetails) return;
        const from = parseInt(document.getElementById('deleteFrom').value, 10);
        const to = parseInt(document.getElementById('deleteTo').value, 10);
        if (!from || !to || to < from) { showToast('Enter a valid chapter range, e.g. 1 to 300.'); return; }
        if (!confirm(`Delete saved chapters ${from}–${to} of "${activeNovelDetails.title}"?\n\nThey stay in the list and can be downloaded again with Update.`)) return;
        try {
            const res = await window.go.main.App.DeleteChapters(activeNovelDetails.slug, from, to);
            showToast(`Deleted ${res.deleted} chapters.`);
            await loadLibrary();
            openDetailsModal(activeNovelDetails.slug);
            refreshSystemStats();
        } catch (e) { showToast(`Delete failed: ${e.message || e}`); }
    });
    document.getElementById('deleteImagesBtn').addEventListener('click', async () => {
        if (!activeNovelDetails) return;
        if (!confirm(`Delete all chapter images of "${activeNovelDetails.title}"? Text stays readable.`)) return;
        try {
            await window.go.main.App.DeleteImages(activeNovelDetails.slug);
            showToast('Images deleted.');
            await loadLibrary();
            openDetailsModal(activeNovelDetails.slug);
            refreshSystemStats();
        } catch (e) { showToast(`Delete failed: ${e.message || e}`); }
    });
    document.getElementById('detailsExportBtn').addEventListener('click', async () => {
        if (!activeNovelDetails) return;
        const btn = document.getElementById('detailsExportBtn');
        btn.disabled = true;
        try {
            const res = await window.go.main.App.ExportEPUB(activeNovelDetails.slug);
            if (res) showToast(`Saved ${res.fileName} (${res.chapters} chapters, ${res.sizeMb}) to ${res.path}`);
        } catch (e) {
            showToast(`Export failed: ${e.message || e}`);
        } finally {
            btn.disabled = false;
        }
    });
    document.getElementById('openSiteBrowserBtn').addEventListener('click', async () => {
        try { await window.go.main.App.OpenSiteBrowser(novelUrlInput.value.trim()); }
        catch (e) { showToast(`Couldn't open the site browser: ${e.message || e}`); }
    });
    document.getElementById('startFromChapterBtn').addEventListener('click', () => {
        startScrape(novelUrlInput.value.trim(), { fromPasted: true });
    });

    loadLibrary();
});
