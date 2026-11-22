# NovelArchiver 📚

> **Private Desktop Bibliotheca & High-Throughput Novel Archiver**  
> *Engineered with Go, Wails v2, and WebView2 — strictly zero backend, zero external web servers, and zero open TCP ports.*

[![Platform: Windows 10/11](https://img.shields.io/badge/Platform-Windows%2010%20%7C%2011-0078d4.svg?style=flat-square&logo=windows)](https://microsoft.com/windows)
[![Platform: Android APK](https://img.shields.io/badge/Platform-Android%20APK%20(7.0%2B)-3DDC84.svg?style=flat-square&logo=android)](NovelArchiver.apk)
[![Engine: Go 1.22+](https://img.shields.io/badge/Engine-Go%201.22%2B-00ADD8.svg?style=flat-square&logo=go)](https://golang.org)
[![Framework: Wails v2](https://img.shields.io/badge/Framework-Wails%20v2.17-df0000.svg?style=flat-square)](https://wails.io)
[![Architecture: Zero-Backend](https://img.shields.io/badge/Architecture-Zero--Backend%20In--Process-0ea5e9.svg?style=flat-square)](#architecture-highlights)
[![Typography: Editorial Book](https://img.shields.io/badge/Typography-Newsreader%20%2B%20Jakarta-c8411b.svg?style=flat-square)](#editorial-reading-experience)
[![License: MIT](https://img.shields.io/badge/License-MIT-emerald.svg?style=flat-square)](LICENSE)

---

## Overview

**NovelArchiver** is a standalone, production-grade desktop application that transforms volatile web serials into permanently preserved, editorial-quality offline books. 

Unlike traditional scrapers that rely on background Node.js processes, local HTTP servers (`localhost:3000`), or heavy Electron runtimes, **NovelArchiver compiles into a single, compact (~11.6 MB) Windows native executable**. All network discovery, concurrent fetching, content sanitization, and reader generation run natively inside compiled Go goroutines, serving all offline chapters and master catalogs in-process directly to Microsoft Edge WebView2 without opening any network ports.

---

## Key Highlights

- **Strictly In-Process (Zero Backend):** No background Node.js server, no Express process, and no listening TCP ports. Zero port conflicts, zero zombie processes, and zero network exposure.
- **High-Throughput Goroutine Worker Pool:** Parallel scraper utilizing Go channels with 8 to 32 worker routines, persistent HTTP keep-alive connection pooling, and sub-second chapter retrieval.
- **Aggressive Ad & Watermark Sanitization:** Strips intrusive pirate watermarks, affiliate links, clickbait footnotes, hidden styles, tracking scripts, and formatting glitches to produce pure, clean prose.
- **Editorial Typography & Reader:** Clean reading experience featuring Google Fonts *Newsreader*, *Plus Jakarta Sans*, and *JetBrains Mono*, customizable font sizing, Serif/Sans switching, and three color modes (*Paper Light*, *Sepia Tone*, *Midnight Slate*).
- **5-Stage Live Pipeline Stepper:** Real-time visibility into the exact phase of archival: **Discovery $\rightarrow$ Download $\rightarrow$ Typesetting $\rightarrow$ Catalog $\rightarrow$ Ready**, complete with dual metrics, speed tracking (ch/s), ETA calculation, and a live chapter ticker stream.
- **SaaS Command Palette (`Ctrl + K`):** Instant spotlight jump to any book in your collection, quick chapter selection, and one-key theme or action switching.
- **Differential Sync & Integrity Audit:** Re-sync new chapters incrementally without re-downloading existing chapters; audit volumes to identify and repair damaged or incomplete chapters automatically.
- **Self-Contained Portability:** All archived books are compiled into standard HTML and CSS inside `./novels/`. They can be read directly in NovelArchiver or copied to any e-reader, phone, or browser without external dependencies.

---

## User Interface Tour

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│ NovelArchiver — Private Bibliotheca                               [ Quick Search... Ctrl+K ]│
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  3 Volumes Bound   │   4,140 Offline Chapters   │   ~38.4 MB Disk Storage   │  100% Offline │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  ARCHIVAL INTAKE                                                                            │
│  [ https://novelbin.xyz/novel/endless-evolution-last-star             ] [ Clear ] [ Inspect]│
│  ┌───────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Chapter Range: [ 1 ] to [ 450 ]  │ Concurrency: [8 Workers ▼]  │ [✓] Clean Prose Only │  │
│  └───────────────────────────────────────────────────────────────────────────────────────┘  │
│  [  ARCHIVE NOVEL  ]                                                                        │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  LIVE PIPELINE MONITOR                                                                      │
│  (1) Discovery ─── [2] Download ─── (3) Typesetting ─── (4) Catalog ─── (5) Ready          │
│  ======================================>------------------ 68%   Speed: 24.2 ch/s  ETA: 14s │
│  Downloading Ch. 308: Void Incursion...                      [ Live Chapter Ticker Stream ] │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  LOCAL REPOSITORY SHELF                                [ Filter volumes... ] [ All | Ready] │
│  ┌────────────────────────┐  ┌────────────────────────┐  ┌────────────────────────┐         │
│  │ [Cover]                │  │ [Cover]                │  │ [Cover]                │         │
│  │ Endless Evolution      │  │ Lord of the Mysteries  │  │ Horror Game Developer  │         │
│  │ 450 Chapters Bound     │  │ 1,432 Chapters Bound   │  │ 2,258 Chapters Bound   │         │
│  │ [Read] [Re-do] [Delete]│  │ [Read] [Re-do] [Delete]│  │ [Read] [Re-do] [Delete]│         │
│  └────────────────────────┘  └────────────────────────┘  └────────────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Quick Start Guide

### Running the Desktop Executable (Windows)

1. Navigate to the project folder.
2. Double-click [`NovelArchiver.exe`](./NovelArchiver.exe) (or run [`run_desktop.bat`](./run_desktop.bat)).
3. The desktop window will open immediately with your local bibliotheca loaded.

Alternatively, launch from PowerShell or Command Prompt:

```powershell
.\NovelArchiver.exe
```

> [!NOTE]
> **System Requirements:** Windows 10 or 11 (64-bit). The application utilizes Microsoft Edge WebView2, which is pre-installed on modern Windows 10 and 11 installations.

---

### Installing on Android (Mobile APK)

NovelArchiver is also available as a standalone native Android application:

1. **Get the APK:** Transfer [`NovelArchiver.apk`](./NovelArchiver.apk) (~5.6 MB) to your Android phone or tablet (via USB, Google Drive, WhatsApp, or local network).
2. **Install:** Tap the APK on your device and select **Install** (allow "Install unknown apps" if prompted).
3. **Enjoy:** Launch NovelArchiver from your app drawer. It operates with the same **zero-backend guarantee** (in-process WebViewAssetLoader, zero background servers, local storage, touch-optimized typography, and live mobile scraper).

To re-compile the Android APK anytime from source:
```powershell
.\build_apk.ps1  # Or double-click build_apk.bat
```

---

## Archiving Your First Novel (3 Steps)

1. **Paste Link:** Paste any novel homepage or chapter URL (e.g., from NovelBin or supported mirrors) into the **Archival Intake** box.
2. **Inspect & Configure:** Click **Inspect** (or press Enter) to verify the cover art, synopsis, and chapter count. Customize the chapter range or concurrency preset if desired.
3. **Archive:** Click **Archive Novel**. Watch the live 5-stage progress monitor download, typeset, and index your volume in seconds. Once complete, click **Open Reader Catalog** or browse it directly from your shelf.

---

## Editorial Reading Experience

Each archived volume includes a standalone, beautifully styled reader:

| Feature | Description |
| :--- | :--- |
| **Typography** | Premium *Newsreader* serif for immersive reading, paired with *Plus Jakarta Sans* UI and *JetBrains Mono* metadata. |
| **Theme Palettes** | **☀️ Paper Light** (clean white), **📜 Sepia Tone** (gentle parchment), and **🌙 Midnight Slate** (low-contrast dark mode). |
| **Controls** | Instant font resizing (`A-` / `A+`), Serif / Sans font toggle, and reading progress indicator. |
| **Reading State** | Automatically preserves your exact chapter and reading progress in `localStorage`. |
| **Navigation** | Floating bottom pill navigation, keyboard shortcuts (`←` Prev, `→` Next, `H` Catalog), and master table of contents. |
| **Shelf Return** | 1-click **← Back to Bibliotheca Shelf** link returns you to your desktop dashboard. |

---

## Global Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| <kbd>Ctrl</kbd> + <kbd>K</kbd> | Open Command Palette / Quick Search |
| <kbd>/</kbd> | Focus the Archival URL Intake box |
| <kbd>T</kbd> | Toggle application theme (Dark / Light) |
| <kbd>?</kbd> | Show Keyboard Shortcuts help dialog |
| <kbd>Esc</kbd> | Dismiss any open modal dialog or palette |
| <kbd>←</kbd> / <kbd>→</kbd> | (In Reader) Jump to Previous / Next Chapter |
| <kbd>H</kbd> | (In Reader) Return to Master Catalog Index |

---

## Documentation Suite

Comprehensive manuals and technical deep-dives are available:

- 📖 **[User Manual](USER_MANUAL.md):** Complete end-to-end user guide covering every screen, button, setting, reader control, and maintenance task.
- ⚙️ **[Architecture & Developer Guide](ARCHITECTURE.md):** Technical specification detailing the Go core engine, concurrency model, in-process asset serving, IPC bridge, and build pipeline.

---

## What is Actually Used to Run It?

NovelArchiver was built with a strict **zero-backend philosophy**. When you launch `NovelArchiver.exe`, here is what is actually running:

### The Runtime Stack

| Component | What It Does | Where It Lives |
| :--- | :--- | :--- |
| **Compiled Go Binary** | Executes the concurrent scraper, HTTP keep-alive connection pool, regex sanitization engine, and filesystem I/O natively in compiled x86_64 machine code. | Inside `NovelArchiver.exe` |
| **Embedded Frontend SPA** | The HTML, CSS, and JS interface (`index.html`, `style.css`, `app.js`) are compiled directly into the binary's data section via Go's `//go:embed all:frontend`. | Inside `NovelArchiver.exe` |
| **Microsoft Edge WebView2** | The native Windows system browser component (Evergreen Chromium) that renders the desktop window and handles DOM layout and fonts. | Windows OS Component (`WebView2Loader.dll`) |
| **In-Process AssetServer** | Intercepts all requests for embedded UI files and local `./novels/` reader chapters, serving them directly from memory into WebView2 via `http.ServeFile`. | In-Process inside Go runtime (Zero network ports) |
| **Wails IPC Event Bus** | Facilitates bi-directional communication between Go goroutines and the frontend JavaScript runtime using native Windows message loops. | Memory Bridge (No WebSockets / No HTTP) |

### What is NOT Used (Strictly Zero Backend)

- ❌ **NO Node.js:** Node is completely absent at runtime. You do not need Node installed to run the application.
- ❌ **NO Express:** No web server framework is running in the background.
- ❌ **NO Localhost Ports:** Zero TCP ports (`localhost:3000`) are opened. Zero network port collisions and zero local network exposure.
- ❌ **NO External Database:** No SQLite, PostgreSQL, or Redis. All metadata is stored in clean, human-readable `metadata.json` files on disk.
- ❌ **NO External Browser Required:** The app opens in its own focused, native desktop window with a custom title bar and Windows 11 Mica backdrop.

---

## How to Build from Source

You can build `NovelArchiver.exe` from source using either the automated scripts or the Wails CLI:

### Prerequisites

To compile the application, you need the following development tools installed on your Windows machine:

1. **Go 1.22 or newer:** ([go.dev/dl](https://go.dev/dl/)) — Verify with `go version`.
2. **GCC C-Compiler (for CGo):** Wails requires a C compiler to link native Windows GUI headers.
   - Recommended: [MSYS2](https://www.msys2.org/) (`pacman -S mingw-w64-x86_64-gcc`) or [WinLibs](https://winlibs.com/).
   - Verify with `gcc --version`.
3. **Wails v2 CLI:** Install globally with:
   ```powershell
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
   Verify system readiness anytime with:
   ```powershell
   wails doctor
   ```
4. **WebView2 Runtime:** Pre-installed on Windows 10 (1809+) and Windows 11.

---

### Build Methods

#### Method 1: 1-Click Automated Script (Recommended)
Double-click [`build.bat`](./build.bat) or run from PowerShell:
```powershell
.\build.ps1
```
This script automatically validates your Go and GCC environment, installs the Wails CLI if missing, compiles the production binary, and places `NovelArchiver.exe` directly in the project root.

#### Method 2: Standard Wails CLI
Run directly from the root project directory:
```powershell
wails build
```

#### What Happens Under the Hood During Build:
1. **Binding Generation:** Wails scans [`app.go`](./app.go) and automatically generates typed TypeScript and JavaScript wrapper functions into `frontend/wailsjs/go/main/App.js`.
2. **Asset Embedding:** Go's compiler bundles the entire `frontend/` directory (`index.html`, `style.css`, `app.js`, icons) into the binary data section via `//go:embed all:frontend`.
3. **Resource Compilation:** Compiles Windows resource files, application icons, and High-DPI awareness manifests.
4. **CGo Linking:** GCC links the Go runtime with Windows GUI subsystems and the WebView2 loader.
5. **Output:** A single, self-contained Windows executable `build/bin/NovelArchiver.exe` (~11.6 MB).

---

## Storage Structure

All scraped prose, cleaned chapters, and metadata are saved locally in the `./novels/` folder:

```
novels/
├── <novel-slug>/
│   ├── metadata.json           # Title, author, cover URL, and chapter list
│   ├── raw/
│   │   ├── chapter_0001.html   # Raw cached chapter HTML
│   │   └── chapter_0002.html
│   └── clean/
│       ├── index.html          # Searchable master catalog & reading portal
│       ├── chapter_0001.html   # Editorial typeset reader chapter
│       └── chapter_0002.html
```

---

## License

This project is open-source software licensed under the [MIT License](LICENSE). Web serial content belongs to their respective authors and publishers. NovelArchiver is intended for personal offline reading and archival preservation.
