# InkShelfs (formerly NovelArchiver)

Save web novels, manga, manhwa and manhua to your own device and read them offline, with no ads, pop-ups or redirects. Good for flights, commutes and patchy connections.

InkShelfs runs locally on **Windows** and **Android**, for novels and manga. Paste a novel page or any chapter link. It saves the chapters with their formatting (bold, italics, tables, images and in-story **[System]** boxes) into a library on your device, with a clean reader and EPUB export.

**Download:** see [Releases](../../releases) or the [InkShelfs download page](https://inkshelfs.vercel.app/).

## Features

- **10 supported sites:** NovelBin, NovelFull, ReadNovelFull, Royal Road, WuxiaWorld, FreeWebNovel, NovelFire, Webnovel, ScribbleHub, EmpireNovel. Other sites fall back to a generic reader-mode parser.
- **Paste either link:** a novel's page or any chapter page. From a chapter link you can download everything, or from that chapter onward.
- **Formatting kept:** system screens like `[Would you like to quit the game?]` become styled boxes; ads, scripts and watermark lines are removed without touching story text.
- **Offline reader:** light, dark and sepia themes, font size, keyboard navigation, remembers where you stopped (shown as **Continue** in the library).
- **Download queue:** add novels while one is downloading; they run one at a time. Pause and resume any time.
- **Full update:** **Update** (or **Update all**) refreshes the description, status and poster, downloads new and missing chapters, re-checks saved chapters for revisions on the site, and keeps chapters the site removed. You get a short summary of what changed.
- **Your poster, or a generated one:** the site's poster is shown; novels without one get a clean generated cover until a poster appears.
- **Storage control:** see each novel's size, delete a chapter range you've read, or delete images only.
- **Back up and restore:** one file with every novel, reading place and setting; restoring merges, never deletes.
- **Notifications:** when a download finishes or a site needs you to tick a human check. On Android, downloads can wait for Wi-Fi.
- **EPUB export:** one file for Google Play Books, Apple Books, Moon+ Reader, KOReader or Send to Kindle.
- **Polite fetching:** at most 3 requests at a time per site, with retries and back-off.

### Sites that check for bots

Some sites only serve real browsers, or show Cloudflare's "Verify you are human" box. For those, InkShelfs loads pages in a visible built-in browser window. If a check appears, **you** tick it, and it's remembered. The app never automates or bypasses these checks.

## Responsible use

InkShelfs is meant for keeping a **personal offline copy** of stories you read. Don't redistribute downloaded chapters. Respect authors and the sites' terms, and support the original publishers and translators where you can.

## How it works

```
frontend/            shared UI (desktop WebView2 and Android WebView)
internal/sites/      sites.json: one parser profile per site (edit to fix a site, no rebuild needed)
internal/source/     discovery, chapter lists, chapter extraction
internal/fetch/      polite HTTP client
internal/browser/    real-browser fetching (WebView2 on Windows, Android WebView)
internal/clean/      allow-list sanitizer that keeps formatting
internal/library/    on-disk library (book.json + one HTML fragment per chapter)
internal/archiver/   download jobs: resume, update, crawl
internal/reader/     offline reader pages
internal/export/     EPUB writer
internal/service/    app layer shared by desktop and Android
mobile/              gomobile bindings for the Android app
android/             Android shell (Java)
cmd/na/              command-line tool: inspect / get / dump / list
```

The desktop app is Go + [Wails v2](https://wails.io). The Android app runs the same Go code through gomobile, so both platforms behave identically.

## Building

**Desktop (Windows):** needs Go 1.22+, the Wails CLI and GCC (MSYS2).

```powershell
wails build -clean        # -> build\bin\InkShelfs.exe  (or: powershell -File build.ps1)
```

**Android:** needs the Android SDK, the NDK (`sdkmanager ndk/29.0.14206865`) and Go.

```powershell
.\build_apk.ps1                      # debug build for phones + emulators
.\build_apk.ps1 -Release             # signed, shrunk build (needs your key, see below)
.\build_apk.ps1 -Release -DebugSign  # same, signed with the debug key, for testing
```

To sign releases, create your own key once. Keep the key and its passwords safe: Android only installs updates signed with the same key.

```powershell
keytool -genkeypair -v -keystore "$HOME\novelarchiver-release.jks" -alias novelarchiver -keyalg RSA -keysize 4096 -validity 10000
```

Then copy `android\keystore.properties.example` to `android\keystore.properties` and fill it in. Both your key file and `keystore.properties` are git-ignored.

**Command line:**

```powershell
go run ./cmd/na inspect <novel-or-chapter-url>
go run ./cmd/na get <url> -from 1 -to 50
go run ./cmd/na list
```

**Tests:** `go test ./...`. Parser tests use saved copies of site pages in `test-fixtures/pages/`. Those copies aren't distributed with the source; tests that need them are skipped when they're absent. To create them, use `go run ./cmd/na dump <url> > test-fixtures/pages/<name>.html`.

## Where your library lives

- Windows: `%AppData%\NovelArchiver\library` (override with `NOVELARCHIVER_LIBRARY`). The folder keeps its old name so existing libraries carry over.
- Android: the app's private storage (exports go to Downloads)

Each novel is a folder with `book.json`, `chapters/0001.html …`, `cover.*` and `img/`.

## Manga, manhwa and manhua

Built into the same app on Windows (`InkShelfs.exe`) and Android (`InkShelfs.apk`, since 3.1). The library shelf has All / Novels / Manga tabs, and the Add box works out by itself whether a link is a novel or a manga. On Android, swipe to turn pages and double-tap to zoom; manga downloads keep going in the background and follow "Only on Wi-Fi".

- **12 manga sites:** MangaDex, Mangapill, Weeb Central, MangaKatana, MangaRead, Toonily, ManhuaPlus, Flame Comics, WEBTOON, MangaGeko, MangaNato/MangaKakalot and Asura Scans. The **Sites** page (globe button) lists every novel and manga site with a "Try an example" link. Profiles and the debugging workflow are in [internal/manga/README.md](internal/manga/README.md).
- **Choose chapters:** all, the first N, a range, or from a pasted chapter on. Or tick chapters with **Select** on the series page. Every downloaded file is checked to be a real image.
- **Reader:** long strip for webtoons; pages right-to-left or left-to-right; two pages side by side. Pages keep their real height before they load, so the strip never jumps. It remembers your page, and at the end of a chapter it opens (or downloads) the next one.
- **Updates:** "Get new chapters" checks novels and manga together; only new chapters are downloaded.
- **Export:** one `.cbz` per series with `ComicInfo.xml`, readable in Komga, Kavita, Mihon/Tachiyomi, CDisplayEx and other comic readers.
- **One site window:** a "Verify you are human" check passed once counts for both novels and manga.

```powershell
go run ./cmd/ma test -lib $env:TEMP\ma-test   # live check of every manga site
go run ./cmd/androidsim -root $env:TEMP\sim    # the Android UI in a browser, on the real phone core (no emulator)
```

Manga library: `%AppData%\MangaArchiver\library` (override with `MANGAARCHIVER_LIBRARY`), kept apart from the novels. Each series is a folder with `series.json`, `cover.*` and `ch/<key>/001.jpg …`.
