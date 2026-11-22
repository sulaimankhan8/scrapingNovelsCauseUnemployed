# NovelArchiver

Save web novels to your own device and read them offline, with no ads, pop-ups or redirects. Good for flights, commutes and patchy connections.

NovelArchiver runs locally on **Windows** and **Android**. Paste a novel page or any chapter link. It saves the chapters with their formatting (bold, italics, tables, images and in-story **[System]** boxes) into a library on your device, with a clean reader and EPUB export.

**Download:** see [Releases](../../releases) or the [download page](https://sulaimankhan8.github.io/novelarchiver/).

## Features

- **10 supported sites:** NovelBin, NovelFull, ReadNovelFull, Royal Road, WuxiaWorld, FreeWebNovel, NovelFire, Webnovel, ScribbleHub, EmpireNovel. Other sites fall back to a generic reader-mode parser.
- **Paste either link:** a novel's page or any chapter page. From a chapter link you can download everything, or from that chapter onward.
- **Formatting kept:** system screens like `[Would you like to quit the game?]` become styled boxes; ads, scripts and watermark lines are removed without touching story text.
- **Offline reader:** light, dark and sepia themes, font size, keyboard navigation, remembers where you stopped.
- **Resume and update:** interrupted downloads continue where they left off; **Update** fetches newly released chapters.
- **Storage control:** see each novel's size, delete a chapter range you've read, or delete images only.
- **EPUB export:** one file for Google Play Books, Apple Books, Moon+ Reader, KOReader or Send to Kindle.
- **Polite fetching:** at most 3 requests at a time per site, with retries and back-off.

### Sites that check for bots

Some sites only serve real browsers, or show Cloudflare's "Verify you are human" box. For those, NovelArchiver loads pages in a visible built-in browser window. If a check appears, **you** tick it, and it's remembered. The app never automates or bypasses these checks.

## Responsible use

NovelArchiver is meant for keeping a **personal offline copy** of stories you read. Don't redistribute downloaded chapters. Respect authors and the sites' terms, and support the original publishers and translators where you can.

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
wails build -clean        # -> build\bin\NovelArchiver.exe
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

- Windows: `%AppData%\NovelArchiver\library` (override with `NOVELARCHIVER_LIBRARY`)
- Android: the app's private storage (exports go to Downloads)

Each novel is a folder with `book.json`, `chapters/0001.html …`, `cover.*` and `img/`.
