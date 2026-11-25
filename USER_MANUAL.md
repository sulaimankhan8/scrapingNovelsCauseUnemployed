# NovelArchiver — End-User Manual & Operator's Guide

Welcome to the **NovelArchiver** User Manual. This guide provides comprehensive, step-by-step instructions for operating your private desktop bibliotheca, archiving web novels with high concurrency, utilizing the editorial reader, and maintaining your offline collection.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [System Requirements & Installation](#2-system-requirements--installation)
3. [Launching NovelArchiver](#3-launching-novelarchiver)
4. [Interface Overview](#4-interface-overview)
5. [Archiving Novels: Step-by-Step](#5-archiving-novels-step-by-step)
6. [Monitoring Live Archival Progress](#6-monitoring-live-archival-progress)
7. [Managing Your Bibliotheca Shelf](#7-managing-your-bibliotheca-shelf)
8. [Re-do, Auditing & Maintenance Workflows](#8-re-do-auditing--maintenance-workflows)
9. [The Offline Reader Experience](#9-the-offline-reader-experience)
10. [Command Palette & Keyboard Shortcuts](#10-command-palette--keyboard-shortcuts)
11. [Portability, E-Readers & Backups](#11-portability-e-readers--backups)
12. [Troubleshooting & Frequently Asked Questions](#12-troubleshooting--frequently-asked-questions)

---

## 1. Introduction

**NovelArchiver** is a self-contained Windows desktop application that archives online web novels into beautiful, permanent offline books.

### How It Works & What Runs It
- **Compiled Go Machine Code:** The entire core engine—scraping orchestrator, HTTP keep-alive connection pooling, regex prose sanitization, filesystem manager, and JSON metadata indexer—is written in Go and compiled directly into native x86_64 machine code inside `NovelArchiver.exe`.
- **Microsoft Edge WebView2:** The UI window is rendered using the Windows Evergreen Chromium engine (Microsoft Edge WebView2). This provides a modern, GPU-accelerated browsing surface with zero memory bloat compared to Electron.
- **Embedded Frontend Assets:** All HTML, CSS, JavaScript, and icons are baked into the binary via Go's `//go:embed all:frontend`. The application never extracts them to temporary folders or disk cache.
- **Zero-Backend In-Process AssetServer:** Whenever you navigate to a novel's offline catalog or chapter, Wails' internal `AssetServer.Handler` intercepts the request and streams the file from `./novels/` straight into WebView2 memory using Go's `http.ServeFile`. **Zero TCP ports are opened, and no web server is running in the background.**
- **Wails Native IPC Bus:** Communication between Go goroutines and the frontend JavaScript runs via native Windows message loops. When a chapter downloads, Go emits an event (`runtime.EventsEmit`) that is delivered instantaneously to JavaScript (`window.runtime.EventsOn`) without polling or WebSockets.

### What is NOT Used (Strictly Zero Backend)
- **NO Node.js:** Node.js is not running, not embedded, and not required to run the app.
- **NO Express Server:** No server listening on `localhost:3000` or any other port.
- **NO Background Daemons:** Closing the desktop window terminates the process completely—no zombie processes left running in Task Manager.
- **NO External Database:** All data is stored in standard, human-readable files and JSON on your hard drive.

---

## 2. System Requirements & Installation

### Minimum Requirements
- **Operating System:** Windows 10 (64-bit, version 1809+) or Windows 11.
- **Runtime:** Microsoft Edge WebView2 Runtime (installed by default on Windows 10 and 11).
- **RAM:** ~45 MB under active scraping; ~30 MB at idle.
- **Storage:** ~12 MB disk space for the executable, plus storage for your archived novels (typically 10–18 MB per 1,000 chapters).

### Installation & Portability
No installer, setup wizard, or registry keys are required:
1. Place `NovelArchiver.exe` in any directory on your computer (e.g., `C:\Users\<You>\Desktop\scrapoing\` or a dedicated portable USB drive).
2. The `./novels/` library folder will automatically be created in the same directory as the executable.

---

## 2.1 How to Build NovelArchiver from Source

If you want to recompile or modify NovelArchiver yourself, follow this step-by-step build guide:

### Required Toolchain
1. **Go Compiler (1.22+):** Download and install from [go.dev/dl](https://go.dev/dl/). Ensure `go` is on your Windows `PATH`.
2. **GCC C-Compiler (for CGo):** Wails requires a C compiler on Windows to link native Win32 APIs and the WebView2 loader.
   - Recommended: Install **MSYS2** ([msys2.org](https://www.msys2.org/)) and run:
     ```bash
     pacman -S mingw-w64-x86_64-gcc
     ```
     Add `C:\msys64\mingw64\bin` to your Windows Environment `PATH`.
   - Alternatively, install **WinLibs** ([winlibs.com](https://winlibs.com/)).
   - Verify in PowerShell: `gcc --version`.
3. **Wails v2 CLI:** Install globally with:
   ```powershell
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
   Verify your toolchain readiness anytime with:
   ```powershell
   wails doctor
   ```

### Building the Binary
We have included automated build scripts for instant 1-click compilation:

#### Method A: 1-Click Batch Script
Double-click [`build.bat`](./build.bat) in the project folder. It will:
- Verify Go and GCC availability.
- Locate or install the Wails CLI.
- Run `wails build -clean`.
- Place the fresh `NovelArchiver.exe` into your workspace root.

#### Method B: PowerShell Script
Open PowerShell in the project directory and run:
```powershell
.\build.ps1
```

#### Method C: Manual Wails Command
```powershell
wails build
```
The compiled binary will be located at `build/bin/NovelArchiver.exe` (~11.6 MB).

---

## 2.2 Installing & Running on Android Devices (APK)

NovelArchiver is fully ported to Android as a native APK, bringing your private bibliotheca and high-throughput serial archiver directly to your mobile device:

### Android Requirements
- **OS Version:** Android 7.0 (Nougat, API 24) or newer (tested on Android 10, 11, 12, 13, 14, 15, and 16).
- **Architecture:** ARM64, ARMv7, x86_64 compatible.
- **Storage:** ~6 MB for the APK installation, plus storage for your archived novels.

### Step-by-Step Installation
1. Locate [`NovelArchiver.apk`](./NovelArchiver.apk) (~5.6 MB) in your project directory.
2. Transfer the `.apk` file to your Android phone or tablet via:
   - USB cable transfer.
   - Cloud drive (Google Drive, Dropbox, OneDrive).
   - Local chat or email attachment.
3. On your Android device, tap the `NovelArchiver.apk` file in your File Manager or Downloads folder.
4. When prompted by Android Security, tap **Settings** and toggle **Allow from this source** (standard for sideloaded open-source APKs).
5. Tap **Install** and open **NovelArchiver** from your home screen or app drawer.

### How It Works on Android
- **Zero Backend:** Uses Google's official `WebViewAssetLoader` to serve all frontend assets and novel chapters in-process with zero network ports.
- **Mobile Archival Engine:** The native Java background worker pool scrapes, sanitizes, and indexes web novels directly on your phone using multi-threading.
- **Touch-Optimized Reader:** Supports mobile swipe reading, theme switching, font scaling, and reading position memory.

### Re-Building the Android APK
To recompile the Android APK with your own changes:
- Double-click [`build_apk.bat`](./build_apk.bat) or run `.\build_apk.ps1`.
- The compiled APK is written to `android/app/build/outputs/apk/debug/app-debug.apk` and automatically copied to `NovelArchiver.apk`.

---

## 3. Launching NovelArchiver

You can start NovelArchiver in any of the following ways:

### Method 1: Double-Click
Double-click `NovelArchiver.exe` in Windows File Explorer.

### Method 2: Launcher Script
Double-click `run_desktop.bat` in the application directory.

### Method 3: Command Prompt / PowerShell
Open PowerShell or Command Prompt in the directory and run:
```powershell
.\NovelArchiver.exe
```

The application window will initialize at `1280x840` resolution with the dark bibliotheca theme enabled.

---

## 4. Interface Overview

The desktop window is divided into four main sections:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [Brand Logo] NovelArchiver             [Ctrl+K Search] [?] [Sound] [Theme]   │
├──────────────────────────────────────────────────────────────────────────────┤
│ [ 3 Volumes Bound ] [ 4,140 Offline Chapters ] [ ~38.4 MB Disk ] [ 100% Ready]│
├──────────────────────────────────────────────────────────────────────────────┤
│ ARCHIVAL INTAKE                                                              │
│ [ Paste novel URL...                                 ] [Clear] [Inspect]     │
│ [⚙️ Advanced Settings Toggle]                                                │
│ [ ARCHIVE NOVEL ]                                                            │
├──────────────────────────────────────────────────────────────────────────────┤
│ LIVE PIPELINE MONITOR (Shown during active jobs)                             │
│ 5-Stage Stepper: (1) Discovery ── (2) Download ── (3) Typesetting...         │
│ Progress Bar, Speed (ch/s), ETA, Live Chapter Ticker, Log Console            │
├──────────────────────────────────────────────────────────────────────────────┤
│ LOCAL REPOSITORY SHELF                                                       │
│ [Filter search...] [Tabs: All (3) | Complete (3)] [Sort: Recent/Title/Count] │
│ Volume Cards with Cover, Status, Chapters, and Action Buttons                │
└──────────────────────────────────────────────────────────────────────────────┘
```

1. **Navigation Bar:**
   - **Logo & Title:** NovelArchiver Private Bibliotheca.
   - **Quick Search (`Ctrl+K`):** Opens the Command Palette spotlight.
   - **Shortcuts (`?`):** Opens the keyboard shortcut cheat-sheet.
   - **Sound Chime:** Toggles audio notifications upon archival completion.
   - **Theme Switcher:** Toggles between Dark and Light mode.

2. **System Repository Statistics Strip:**
   - **Volumes Bound:** Total count of novels archived on disk.
   - **Offline Chapters:** Total count of typeset, ready-to-read offline chapters.
   - **Disk Storage:** Cumulative disk usage of all archived books.
   - **100% Offline Ready:** Confirms all catalog assets are stored locally.

3. **Archival Intake:**
   - URL input with automatic paste detection.
   - **Clear Button:** Resets the input and preview card.
   - **Inspect Button:** Fast metadata discovery without downloading.
   - **Advanced Options Accordion:** Chapter bounds, concurrency, clean-only, and force flags.

4. **Local Repository Shelf:**
   - Displays all archived books as cards with cover art, author, chapter counters, and action buttons.

---

## 5. Archiving Novels: Step-by-Step

### Step 1: Input URL or Search by Title

Paste a novel or chapter link, **or type the novel's title** and press Search. A title is searched on NovelFull, ReadNovelFull, NovelPing, FreeWebNovel, NovelFire, Wattpad, Archive of Our Own and WebNovel at the same time. Each novel shows once, with a row per site:

- **Chapter count** from the site's search page. **Count** reads the site's full chapter list (paid and locked chapters are counted separately), and **Compare counts** does that for every site in the card, so you can pick the site with the most chapters.
- **Preview** opens the page in the site window. **Get** opens the usual download card; closing it brings you back to the results.
- Royal Road and ScribbleHub are searched only when you tap **Search them too**, because their search asks you to tick a "verify you are human" check in the site window.

#### Chinese and Japanese novels

Turn on **中文 Chinese** or **日本語 Japanese** under the search results (or type the title in Chinese or Japanese) to also search UUKanshu and Shousetsuka ni Narou. Their copies appear in the same card as the English sites, tagged **中文 → EN** / **日本語 → EN**, so you can compare chapter counts; the original is often far ahead of the English translations. You can also paste links from 69shuba, UUKanshu, Syosetu and Kakuyomu.

These novels are saved as written and translated to English with Google Translate as each chapter downloads (free, no account). The title, description and chapter list are translated too. In the reader, **EN / 原文** switches between the English and the original; your choice is remembered. EPUB export is in English. If Google is busy, chapters are saved in the original and translated on the next update.

### Step 1b: Paste a Link
Copy the web novel URL from your browser (e.g., `https://novelbin.xyz/novel/endless-evolution-last-star` or any chapter link) and paste it into the **Archival Intake** box.

> [!TIP]
> Pasting an HTTP/HTTPS link automatically triggers an instant metadata inspection!

### Step 2: Inspect Discovery Preview
Click **Inspect** (or press Enter). Within milliseconds, NovelArchiver extracts:
- Novel Title
- Author Name
- Total Chapters Available
- Synopsis / Book Description
- High-resolution Book Cover Art

A preview card will appear displaying this information.

### Step 3: Configure Archival Settings (Optional)
Click **Advanced Archival Options** to customize parameters:
- **Chapter Range:**
  - `Start Chapter`: Enter a starting chapter number (e.g., `100`).
  - `End Chapter`: Enter an ending chapter number (e.g., `250`).
  - *Leave blank to download all available chapters.*
- **Concurrency (Parallel Workers):**
  - Choose one of the 4 quick presets:
    - **Eco (4 workers):** Low CPU usage, gentle on home connections.
    - **Balanced (8 workers):** Recommended default; fast and reliable.
    - **Turbo (16 workers):** High-speed multi-connection downloads.
    - **Max (32 workers):** Extreme throughput for fast CDN servers.
- **Clean & Typeset Prose Only:**
  - Check this box if raw chapters are already cached on disk and you only want to re-sanitize and regenerate the offline reader HTML.
- **Force Full Re-Download:**
  - Bypasses existing local cache and re-downloads fresh copies of all chapters from the web.

### Step 4: Initiate Archival
Click **Archive Novel** (or click **Archive All Discovered Chapters** on the preview card).

---

## 6. Monitoring Live Archival Progress

Once initiated, the **Live Pipeline Monitor** appears immediately:

### 1. The 5-Stage Visual Stepper
The stepper highlights the active phase of the archival pipeline:
1. **Discovery:** Ingesting table of contents, chapter indices, and metadata.
2. **Download:** High-concurrency worker pool fetching chapter prose.
3. **Typesetting:** Sanitizing raw HTML, removing watermarks, formatting paragraphs.
4. **Catalog:** Generating offline search index and table of contents.
5. **Ready:** Volume is fully compiled and ready to read offline.

### 2. Dual-Metric Progress Bar & Live Stats
- **Progress Bar & Percentage:** Shows cumulative job completion.
- **Chapter Counter:** Displays `Completed / Total` chapters.
- **Real-Time Speed:** Tracks throughput in chapters per second (e.g., `24.5 ch/s`).
- **Estimated Remaining (ETA):** Accurate dynamic time-to-completion countdown.
- **Network Fetches vs. Cache Hits:** Separates freshly downloaded chapters from previously cached ones.
- **Typeset Chapters:** Counter for completed reader formatting.

### 3. Live Chapter Feed Ticker
A dynamic chip ticker shows the last 8 processed chapters in real time:
- **Green "Saved" Chip:** Successfully fetched and written from the network.
- **Yellow "Cached" Chip:** Verified from local disk (instant cache hit).
- **Blue "Typeset" Chip:** Cleaned and typeset into editorial HTML.

### 4. Technical Stream Console
Click **Technical Stream** to view granular execution logs. Click **Clear Stream** to reset the console or **Hide** to minimize it.

### 5. Cancelling an Active Task
If you need to stop an archival task at any time, click the **Cancel** button located in the progress bar details strip. The Go engine will immediately cancel the active context and terminate worker goroutines cleanly.

### 6. Archival Completion Banner
When the job finishes:
- An audio chime will play (if sound is enabled).
- A completion banner will display: `"[Novel Title]" Successfully Bound!`.
- Click **Open Reader Catalog** to begin reading immediately.
- The library shelf and system stats update automatically.

---

## 7. Managing Your Bibliotheca Shelf

Your library shelf organizes all archived books on your local disk.

### Shelf Filters & Search
- **Filter Tabs:**
  - **All Volumes:** Displays every novel present in `./novels/`.
  - **100% Offline Complete:** Displays only volumes where all discovered chapters have been cleaned and indexed.
- **Live Search Input:** Type any part of a title, author name, or slug to filter the shelf instantly.
- **Sorting Dropdown:**
  - **Recently Bound:** Orders by latest modification.
  - **Title (A–Z):** Alphabetical order.
  - **Chapter Count:** Orders from largest book to smallest.

### Volume Card Anatomy
Each card on the shelf provides:
- **Book Cover Art:** With subtle elevation hover animation.
- **Title & Author Name.**
- **Status Badges:** Total chapters bound and offline readiness.
- **Action Buttons:**
  - **Read:** Opens the offline master catalog in your reader.
  - **Re-do (↺):** Opens the Re-do modal with granular re-archival options.
  - **Details (👁️):** Opens the volume directory modal with a searchable chapter list.
  - **Delete (🗑️):** Permanently removes the volume from disk.

---

## 8. Re-do, Auditing & Maintenance Workflows

NovelArchiver includes built-in tools for maintaining your offline library:

### The Re-do Modal
Click the **Re-do (↺)** button on any volume card to choose from 4 options:
1. **Fetch New Chapters (Differential Sync):**
   - Automatically checks for newly released chapters while skipping already downloaded ones. Perfect for ongoing web serials!
2. **Re-Clean & Re-Typeset Volume:**
   - Re-applies the HTML cleaner and builds fresh reader chapters from raw cache without touching the network.
3. **Force Full Re-Download:**
   - Prompts for confirmation and re-fetches every single chapter from the web, overwriting local copies.
4. **Load into Archival Settings:**
   - Populates the URL and metadata into the top intake panel so you can adjust chapter ranges or concurrency manually.

### Chapter Integrity Audit
To verify that your offline files are not corrupted or truncated:
1. Click **Details (👁️)** on any volume card.
2. Click **Audit Integrity** in the modal.
3. NovelArchiver inspects every raw chapter file on disk for file size and valid prose content.
4. If corrupt or missing chapters are detected, it prompts you to auto-retry fetching them immediately.

### Deleting a Volume
Click the red **Delete (🗑️)** icon on any card. NovelArchiver will display a confirmation dialog detailing the title and disk files that will be removed. Upon confirmation, the entire `./novels/<slug>/` directory is permanently deleted and the shelf updates.

---

## 9. The Offline Reader Experience

Every archived novel has an integrated, editorial-grade reading interface.

### The Master Catalog (`index.html`)
Opening a novel launches its master catalog:
- Displays high-resolution book cover, author, and full description.
- **Instant Search Box:** Type a chapter number or keyword to filter thousands of chapters in real time.
- **One-Click Reading:** Click any chapter card to jump straight to reading.
- **Return Link:** Click **← Back to Bibliotheca Shelf** to return to your main library dashboard.

### The Chapter Reader (`chapter_XXXX.html`)
Inside a chapter, enjoy an editorial reading environment:

#### 1. Typography & Layout
- Body text set in Google *Newsreader* (editorial book serif) with optimal line heights ($1.8$) and comfortable margins ($820\text{px}$ maximum container).
- First paragraph un-indented; subsequent paragraphs cleanly indented.
- Reading time estimate (e.g., `~7 min read`) and word counter displayed in the header.

#### 2. Theme Customization
Click the theme buttons in the top header:
- **☀️ Light Paper:** Clean, crisp white reading surface.
- **📜 Sepia Tone:** Warm, gentle parchment background designed to reduce eye strain.
- **🌙 Midnight Slate:** Deep dark theme (`#0e1216`) with subdued warm text (`#e6edf3`).

#### 3. Font Size & Style Controls
- **`A-` / `A+` Buttons:** Decreases or increases font size with instant preview.
- **`Font` Button:** Toggles between classic Book Serif (*Newsreader*) and Modern Clean Sans-Serif (*Plus Jakarta Sans*).

#### 4. Reading Progress Bar & Memory
- An accent reading bar at the top of the window dynamically tracks your scroll progress through the chapter ($0\%\dots 100\%$).
- Your current chapter and timestamp are automatically saved in `localStorage`.

#### 5. Navigation Options
- **Top Bar:** Quick link back to the novel's master catalog.
- **Bottom Navigation:** Prominent **Previous Chapter**, **Catalog**, and **Next Chapter** buttons.
- **Floating Pill Bar:** A persistent floating pill navigation bar rests at the bottom of the screen, allowing one-click chapter jumping even mid-page.
- **Keyboard Navigation:**
  - Press <kbd>→</kbd> for Next Chapter.
  - Press <kbd>←</kbd> for Previous Chapter.
  - Press <kbd>H</kbd> to return to Master Catalog.

---

## 10. Command Palette & Keyboard Shortcuts

NovelArchiver is built for keyboard power users:

### The Command Palette (`Ctrl + K`)
Press <kbd>Ctrl</kbd> + <kbd>K</kbd> anywhere in the application to open the Command Palette:
- **Search Novels:** Type any novel title to jump directly to its catalog.
- **Direct Actions:**
  - Type `theme` to toggle light/dark theme.
  - Type `focus` or `intake` to jump to the URL input.
  - Type `shortcuts` to open the shortcuts dialog.
- Use <kbd>↑</kbd> and <kbd>↓</kbd> arrows to navigate, <kbd>Enter</kbd> to select, and <kbd>Esc</kbd> to dismiss.

### Keyboard Shortcuts Reference

| Shortcut | Context | Action |
| :--- | :--- | :--- |
| <kbd>Ctrl</kbd> + <kbd>K</kbd> | Global | Open / Close Command Palette |
| <kbd>/</kbd> | Global | Focus URL Intake input |
| <kbd>T</kbd> | Global | Toggle Light / Dark Application Theme |
| <kbd>?</kbd> | Global | Open Keyboard Shortcuts Help Modal |
| <kbd>Esc</kbd> | Global | Dismiss any open modal dialog or palette |
| <kbd>→</kbd> | Reader | Jump to Next Chapter |
| <kbd>←</kbd> | Reader | Jump to Previous Chapter |
| <kbd>H</kbd> | Reader | Return to Novel Catalog Index |

---

## 11. Portability, E-Readers & Backups

One of NovelArchiver's greatest strengths is **complete data independence**.

### Reading Without NovelArchiver
All archived volumes are standard, clean HTML and CSS files stored in `./novels/`:
- You can double-click `./novels/<novel-slug>/clean/index.html` to read your books in Chrome, Firefox, Brave, Edge, or Safari without running NovelArchiver!

### Reading on Phones & Tablets
1. Copy a novel's `clean/` folder to your phone or tablet (via USB, Google Drive, Dropbox, or local network share).
2. Open `index.html` in your mobile browser.
3. The reader is 100% responsive, fitting phones (375px), tablets (768px), and large screens seamlessly.

### Converting to EPUB or E-Readers
Because the cleaned chapter files contain pure, valid HTML without ads or scripts, they can be imported directly into **Calibre** or converted to EPUB/MOBI for Kindle or Kobo with zero cleanup needed.

### Backing Up Your Library
To backup your entire library:
- Simply copy or zip the `./novels/` folder. All metadata, raw chapters, and cleaned reading volumes are contained inside.
- To restore on a new computer, place the `./novels/` folder next to `NovelArchiver.exe` and launch the app. All volumes will load instantly.

---

## 12. Troubleshooting & Frequently Asked Questions

### Q1: Is an internet connection required to run NovelArchiver?
**No.** An internet connection is only needed when downloading new chapters from the web. Once archived, your entire library is stored 100% locally on your hard drive and can be browsed and read completely offline.

### Q2: Why is there no backend or localhost:3000?
NovelArchiver was engineered in Go using Wails v2 specifically to eliminate fragile web backends. The Go binary manages scraping, file I/O, and asset delivery directly in memory, preventing port conflicts and background zombie processes.

### Q3: What should I do if a novel fails during download?
1. Check your internet connection.
2. In the **Advanced Options**, try reducing the concurrency preset from **Turbo** or **Max** down to **Balanced (8)** or **Eco (4)**. Some web hosts throttle aggressive multi-threaded requests.
3. Open the **Re-do Modal** on the novel card and select **Fetch New Chapters (Differential Sync)**. NovelArchiver will pick up right where it left off without re-downloading completed chapters.

### Q4: How do I change where novels are saved?
By default, novels are saved in a `./novels/` folder in the same directory where `NovelArchiver.exe` is located. To move your library to another drive (e.g., `D:\MyNovels\`), simply move `NovelArchiver.exe` and your `./novels/` folder together to that drive.

### Q5: How much disk space does a novel require?
On average, 1,000 chapters require approximately:
- **Raw cache:** ~10 MB
- **Cleaned reader chapters:** ~8 MB
- **Total:** ~18 MB per 1,000 chapters.
A massive 4,000-chapter web serial typically uses under 75 MB of total disk space.

---

*NovelArchiver is designed for offline reading and preservation. Happy reading!*
