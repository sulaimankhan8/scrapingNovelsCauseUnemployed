# NovelArchiver: Analysis & Improvement Plan

## What the app is

A local-only novel archiver for reading offline with no ads, e.g. on flights. You paste a novel URL, choose a chapter range, it scrapes the chapters, cleans them, and saves them to a local library with a built-in reader. It runs on desktop and Android.

Current pieces:

| Part | Tech | Files |
|---|---|---|
| Desktop app | Go + Wails v2 (WebView2) | `main.go`, `app.go`, `discovery.go`, `scraper.go`, `generator.go`, `storage.go`, `types.go` |
| Android app | Java + WebView, **a second, separate scraper** | `android/app/src/main/java/.../ScraperEngine.java`, `NovelBridge.java`, `MainActivity.java` |
| UI | Vanilla HTML/CSS/JS, shared (copied into Android assets) | `frontend/` |
| Legacy | Old Node scripts (`scrape.js`, `clean.js`, ...) | deleted in the working tree, but the deletion isn't committed |

## Problems found

### 1. Two scrapers that have already drifted apart
The Go and Java engines each have their own discovery, fetching, cleaning and HTML generation. They already disagree:
- Go builds chapter URLs from the CDN `index`. Java uses `i + 1`, which is off by one.
- The Go cleaner, the Java cleaner and the old `clean.js` all produce different output.

Every fix has to be made twice, and the two versions keep drifting.

### 2. Only one site actually works
Discovery is hard-coded to `boxnovela.b-cdn.net` / `novelbin`. The "web fallback" parses HTML with regex. If it finds no chapter list, it **invents 100 chapters** (`discovery.go`). Pasting a link from any other site will fail quietly or produce junk.

### 3. Formatting is lost: system screens, bold, italics, images
`CleanChapterContent` runs every paragraph through `cleanTagText`, which **strips all tags**:
- `<strong>[Would you like to quit the game?]</strong>` becomes plain text.
- `<i>`, `<table>`, `<blockquote>`, `<hr>` and `<img>` are all dropped.

The current library still looks right only because it was cleaned by the old `clean.js`.

### 4. The ad filter deletes real story text
Any line under 120 characters that contains `"pirate"`, `"read at"`, `"watermark"`, etc. is removed. *"The pirate ship drew closer."* gets deleted.

### 5. Not actually offline
- Every chapter page loads Google Fonts from the internet.
- Covers are hot-linked to the CDN.
- Chapter images are never downloaded.

On a plane, covers and fonts break.

### 6. Wasteful storage
3 novels took **151 MB**. Each chapter is stored twice: the raw copy, plus a "clean" copy with ~500 lines of CSS embedded in every file. A shared reader template plus the content stored once would take roughly a third of the space. Deletion only works on a whole novel. You can't free space by deleting read chapters.

### 7. Reliability
- No retry or backoff.
- Failed chapters aren't recorded, so you have to "Audit" to find gaps.
- `CancelTask` clears `isBusy` before the goroutine has stopped, which is a race.
- A range that matches no chapters silently scrapes *everything*.
- Raw files contain the `<h3>` title twice.
- `NovelsDir = "novels"` is relative to the current working directory, so the library moves depending on how the exe is launched.

### 8. Repo hygiene
- `novelarchiver.exe`, `NovelArchiver.apk` and `android/app/build/` sit untracked in the repo root.
- About 5,900 chapter files are in git history.
- `.gitignore` only covers Node.

## Proposed architecture

```
                 ┌──────────── core/ (Go, one codebase) ────────────┐
                 │  sources/   site adapters (novelbin, generic, …)  │
                 │  fetch/     HTTP client, retry, rate-limit        │
                 │  clean/     HTML sanitizer (allow-list, keeps     │
                 │             formatting + images)                  │
                 │  library/   on-disk store, delete, usage stats    │
                 │  jobs/      download queue, resume, cancel        │
                 │  export/    EPUB                                  │
                 └───────▲───────────────────────────▲───────────────┘
                         │                           │
          Desktop: Wails v2 bindings     Android: gomobile bind → .aar
                         │               (thin Kotlin/Java WebView shell)
                         └──────── frontend/ (one shared UI) ─────────┘
```

**Key decision: keep Go and share it with Android through `gomobile bind`.** Delete `ScraperEngine.java`. Android keeps only a ~100-line WebView shell that calls the same Go core. This fixes problem 1 for good.

### Site adapters (fixes #2)
```go
type Source interface {
    Match(url string) bool
    Discover(ctx, url) (*Novel, error)          // title, author, cover, chapter list
    FetchChapter(ctx, ch Chapter) (html string, err error)
}
```
- **`novelbin` adapter**: the existing CDN JSON path, cleaned up.
- **`generic` adapter**: uses goquery (real HTML parsing) with readability-style content detection. If the site has no table of contents, it follows "next chapter" links.
- **User-defined rules**: a small JSON file per site (`{ "content": "#chr-content", "next": "a#next_chap", "toc": ".list-chapter a" }`). New sites can be added without a rebuild.

### Site parsers: 10 sites (added 2026-11-22)
Profiles are in [`internal/sites/sites.json`](internal/sites/sites.json). The field guide and verification log are in [`sites/README.md`](internal/sites/README.md).

| # | Site | Works with | TOC strategy |
|---|---|---|---|
| 1 | NovelBin | in-app browser (CDN blocks plain HTTP) | JSON API |
| 2 | NovelFull | plain HTTP ✅ | paginated |
| 3 | ReadNovelFull / AllNovelFull | plain HTTP ✅ | one ajax call |
| 4 | Royal Road | plain HTTP ✅ | inline |
| 5 | WuxiaWorld | plain HTTP ✅ (free chapters) | follow next |
| 6 | FreeWebNovel | plain HTTP ✅ | sequential 1..N |
| 7 | NovelFire | TOC http, chapters in browser | paginated |
| 8 | Webnovel | plain HTTP (free chapters only) | catalog page |
| 9 | EmpireNovel | user ticks the Cloudflare check once | TBD |
| 10 | ScribbleHub | user ticks the Cloudflare check once | ajax |
| — | Any other site | generic (JSON-LD / og / readability / next links) | auto |

**Input:** the user can paste the **novel page or any chapter page**. A chapter URL is mapped back to its novel. Otherwise the app crawls forward from that chapter using "next" links. All 16 test URLs below map to exactly one site and the right page type:

```
https://novelbin.xyz/novel/lord-of-the-mysteries
https://novelbin.xyz/read/lord-of-the-mysteries/chapter-1
https://novelfull.com/lord-of-the-mysteries.html
https://novelfull.com/lord-of-the-mysteries/chapter-1-crimson.html
https://readnovelfull.com/lord-of-the-mysteries-v1.html
https://readnovelfull.com/lord-of-the-mysteries/chapter-1-crimson-v1.html
https://www.royalroad.com/fiction/21220/mother-of-learning
https://www.royalroad.com/fiction/21220/mother-of-learning/chapter/301778/1-good-morning-brother
https://www.wuxiaworld.com/novel/martial-world
https://www.wuxiaworld.com/novel/martial-world/mw-chapter-1
https://freewebnovel.com/novel/last-gun-alchemist
https://freewebnovel.com/novel/last-gun-alchemist/chapter-1
https://novelfire.net/book/lord-of-the-mysteries
https://novelphoenix.com/novel/lord-of-the-mysteries/chapter-1
https://www.webnovel.com/book/32824130400116105
https://www.scribblehub.com/series/2/
```

**Two fetch engines** behind one `Fetcher` interface:
- `HTTPFetcher`: the Go client, used for 6 of the 10 sites.
- `BrowserFetcher`: a hidden WebView2 window (desktop) or Android WebView that loads the page as a normal browser and returns the rendered DOM / JSON.

For Cloudflare sites the window becomes visible so the **user** can tick "Verify you are human". The app never automates or bypasses that check.

**Important:** the current app's main path is already broken. `boxnovela.b-cdn.net` now returns 403 to the Go HTTP client, so novelbin downloads fail today. Moving novelbin to `BrowserFetcher` fixes this.

### Cleaning (fixes #3, #4)
- Use an allow-list sanitizer (`bluemonday`). Keep `p, br, hr, strong, b, em, i, u, s, sup, sub, blockquote, table/tr/td/th, ul/ol/li, h1-h4, img, span/div with safe classes`. Drop scripts, iframes, event handlers and inline ad containers.
- Ad removal is driven by the adapter: remove known ad *elements*, and match watermark lines exactly by pattern (e.g. `^Source: novelbin`). No substring matching on words.
- Detect "system screen" blocks (lines in `[ … ]` or `【 … 】`, or bordered divs) and wrap them in `<div class="sys">` so the reader can style them as boxes.
- Golden tests: `test-fixtures/sample-chapters/*` in, expected HTML out.

### Storage (fixes #5, #6)
```
<AppData>/NovelArchiver/library/<slug>/
    book.json            metadata + chapter list + per-chapter status
    cover.jpg            downloaded
    chapters/0001.html   sanitized fragment only (no CSS)
    images/<hash>.jpg    downloaded chapter images, links rewritten
```
- One reader template in the app renders fragments. Fonts are bundled locally.
- The library location is an absolute app-data path, and the user can change it.
- Delete options: a whole novel, a chapter range ("delete chapters 1–500, I've read them"), or images only. Show the size of each novel.
- Optional: export to **EPUB**, so the book works in any reader app.

### Download jobs (fixes #7)
- Per-host concurrency limit (default 4) and polite delay. Exponential backoff on 429/5xx.
- Each chapter's status (`pending/ok/failed`) is saved in `book.json`, so a "Resume" or "Retry failed" works after a crash or app close.
- "Update" fetches only new chapters for ongoing novels.
- Validate ranges: an empty range is an error, not "scrape everything".

## Phased roadmap

| Phase | Scope | Result |
|---|---|---|
| **0. Cleanup** | Commit the removal of the Node scripts. Add a proper `.gitignore` (exe, apk, `android/**/build`, `novels/`, `build/bin`). Move the Go code into `core/` + `cmd/desktop/`. | A clean repo |
| **1. Core rewrite** | `fetch`, `clean` (sanitizer + golden tests), `library` (new layout, absolute path), `sources/novelbin`. | Desktop scrapes novelbin with formatting kept |
| **2. Offline completeness** | Download covers and images, bundle fonts, one reader template, per-chapter status, resume/retry. | Fully works in airplane mode |
| **3. Storage UX** | Per-novel size, range delete, image-only delete, update-new-chapters. | Space management |
| **4. Android on the shared core** | `gomobile bind` the core, replace `ScraperEngine.java`, storage under app files dir. | Identical behavior on phone |
| **5. More sites** | Profile engine for `sites.json`, BrowserFetcher, the 10 profiles, generic adapter, next-link crawling, chapter-URL input. Before phase 1 ends, novelbin must move to BrowserFetcher, because it is broken today. | Paste a novel or chapter link from any of the 10 sites |
| **6. Extras** | EPUB export, reading progress/bookmarks, search. | Polish |

## Test data
`test-fixtures/novels.json` holds the 4 novels that were in the library before it was wiped:
- Endless Evolution: Last Star
- Horror Game Developer
- Lord of the Mysteries
- Super Gene (from git history)

`test-fixtures/sample-chapters/` keeps two raw chapters to use as cleaner golden-test inputs. Horror Game Dev ch. 1 has the bold `[System]` prompts.

## Progress (2026-11-22)

**Done, phases 0, 1, 2, 3 and most of 5:**
- New Go core in `internal/`:
  - `sites`: profile engine plus the 10 profiles.
  - `fetch`: polite HTTP with per-host limits, retry, and Retry-After support.
  - `source`: discovery, all TOC strategies, generic fallback, and links that point at either a chapter or a novel page.
  - `clean`: keeps formatting, system boxes and images.
  - `library`: AppData library, range delete, image delete, sizes.
  - `archiver`: resume, update, crawl mode.
  - `reader`: offline reader with no web fonts.
- The desktop app (`app.go`) uses the new core. The old `discovery/scraper/generator/storage/types.go` files were deleted.
- Headless CLI: `go run ./cmd/na inspect|get|list`.
- Tests: `go test ./...` runs offline against `test-fixtures/pages`.

**Live results:**
- Download works over plain HTTP: NovelFull, ReadNovelFull, FreeWebNovel, WuxiaWorld.
- NovelFire: the chapter list works, but chapters need the browser.
- Cloudflare blocks Go's HTTP client, so these need the in-app browser: Royal Road, Webnovel, NovelBin (CDN).
- Need the user to tick the "Verify you are human" checkbox: EmpireNovel, ScribbleHub.

**Site browser (done 2026-11-22):** `internal/browser` opens a WebView2 window (minimized in the taskbar) for sites that refuse plain HTTP.
- Pages load as in Edge. Once the window is on a site, the engine reads pages with that page's own `fetch()`, using its cookies.
- If a site shows "Verify you are human", the window comes to the front and waits up to 3 minutes for the user. The app never clicks it.
- Cookies persist in `%AppData%/NovelArchiver/SiteBrowser`.

All 10 sites now download (verified live 2026-11-22). EmpireNovel shows a Cloudflare check, which the user ticks once in the site browser window.

**Android on the shared core (done 2026-11-22):**
- `internal/service` holds the app logic. Desktop `app.go` and Android `mobile/` are thin wrappers around it.
- `gomobile bind ./mobile` produces `android/app/libs/core.aar`.
- The Android shell (about 400 lines of Java) does three things:
  - Serves the shared `frontend/` and Go-rendered `/library/...` pages.
  - Bridges page calls through `android-shim.js` to `Core.call()`.
  - Implements the browser primitives on a WebView overlay (`SiteBrowser`); the strategy lives in Go (`internal/browser/driven.go`).
- The old Java scraper is deleted.
- Verified on an Android emulator (16 KB pages):
  - FreeWebNovel download from a chapter link.
  - NovelBin through the WebView site browser: 274 chapters, 0 failures.
  - Reader with system boxes.
  - Airplane-mode reading.
- Royal Road showed the emulator a Cloudflare check, which waits for the user, as designed.

Build: `.uild_apk.ps1` (needs the Android NDK; `sdkmanager ndk/29.0.14206865`).

**EPUB export (done 2026-11-22):** `internal/export` writes EPUB 3 (plus toc.ncx for older readers).
- Contents: cover, title page, the downloaded chapters (system boxes, tables, images), and a contents page.
- Desktop: **Export EPUB** in a novel's details opens a save dialog (defaults to Downloads).
- Android: the same button saves straight to the phone's Downloads folder.
- Verified: a 274-chapter export on the emulator, with 279 XML documents and 0 malformed.

**Next:**
1. Test on a real phone (Cloudflare usually lets real devices through).
2. Release-signed APK.
3. Optional: check exports with the official `epubcheck` validator.
