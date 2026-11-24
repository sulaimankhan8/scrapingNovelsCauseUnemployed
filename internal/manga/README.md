# InkShelfs manga: site profiles

`sites.json` tells the manga engine how to read each site. It is built into the app, and users can drop extra or replacement profiles into `%AppData%\MangaArchiver\sites\*.json`. A profile with the same `id` replaces the built-in one, so a site that changes its layout can be fixed without rebuilding.

Manga is part of InkShelfs: `manga_app.go` binds it as `window.go.main.MangaApp`, the screens are in `frontend/manga.js` and `frontend/manga.css`, and its library is `%AppData%\MangaArchiver\library` (kept apart from the novels). It reuses `internal/fetch`, `internal/browser` (the WebView2 site window) and the selector mini-language from `internal/sites`.

## Flow

```
pasted link ──► seriesUrl / chapterUrl regex ──► series page ──► metadata + chapter list
                         (chapter link → seriesFromChapter)
chapter ──► chapter page (+ optional image-list request) ──► image links ──► saved pages
```

## Fields

| Field | Meaning |
|---|---|
| `fetchMode` | `http` (default) or `browser`. Browser sites load pages in the site window (real Edge), so Cloudflare checks pass or can be ticked by the user. HTTP sites also fall back to the window if they start showing a check. Images always try plain HTTP first, with the right `Referer`. |
| `status` | `tested`: works over plain HTTP. `human-check`: may show "Verify you are human". |
| `api` | `mangadex` uses built-in code for the official MangaDex API. |
| `headers` | Sent with every page request (MangaDex wants an honest User-Agent). |
| `imageReferer` | `chapter` (default) sends the chapter page as Referer for images. `origin` sends the site's home page. |
| `seriesUrl`, `chapterUrl` | Regexes with named groups (`{slug}`, `{id}`, ...) usable in templates. |
| `seriesFromChapter` | A `{var}` template, or a selector on the chapter page. |
| `series.*` | Selectors for `title`, `cover`, `author`, `status`, `description`. Missing ones fall back to `og:` meta tags. |
| `vars` | Extra values read from the series page: a selector, or `re:<regex>` on the raw HTML. |
| `chapters.url` | Where the list lives (default: the series page). `method: POST` for Madara's `ajax/chapters/`. |
| `chapters.items` / `title` | Selector for chapter links, and an optional sub-selector for the title. |
| `chapters.json` | For JSON lists: `script` (read from a `<script>` such as `#__NEXT_DATA__`), dotted `path`, `title` and `url` (field names or `{field}` templates; unknown names come from the series values), `base`. |
| `chapters.order` | `newest-first` lists are reversed so chapter 1 comes first. |
| `pages.url` / `vars` | An optional second request for the image list (`{chapterId}` read from the chapter page). |
| `pages.jsonField` | The image list is HTML inside this JSON field. |
| `pages.images` | Selector for page images; `@a\|b` picks the first attribute that holds a real link (placeholders and `data:` are skipped). |
| `pages.sortAttr` | Order images by this numeric attribute (on the image or an ancestor). |
| `pages.scriptArray` | Take the longest JavaScript array of links (MangaKatana). |
| `pages.regex` | Every match in the raw page, in order, de-duplicated (Flame Comics). |
| `pages.exclude` | Drop links matching this regex. |

Every downloaded file is checked to be a real JPEG/PNG/WebP/GIF/AVIF, so an error or bot-check page is never saved as a page. A chapter is written to `<key>.part` and renamed only when every page has arrived.

## Debugging a site

```
go run ./cmd/ma inspect <series-or-chapter-url>   # metadata + chapter count
go run ./cmd/ma pages <series-url> 1              # image links of chapter 1
go run ./cmd/ma dump <any-url-on-the-site>        # the page exactly as the engine sees it
go run ./cmd/ma test -lib <tmp-dir>               # every site: discover + download chapter 1
```

## Verification log (2026-11-24, live downloads, site window enabled)

Each site's example series: discover, then download chapter 1. Every page was checked to be a real image.

| Site | Kind | How it's fetched | Result |
|---|---|---|---|
| MangaDex | Manga / manhwa / manhua | official JSON API | ✅ Berserk: 404 chapters, ch. 1 = 94 pages |
| Mangapill | Manga | HTTP | ✅ Berserk: 405 chapters (prototype + main run), ch. 1 = 92 pages |
| Weeb Central | Manga / manhwa | HTTP (htmx list + image endpoints) | ✅ 284 chapters, ch. 1 = 86 pages |
| MangaKatana | Manga / manhwa | HTTP (page list in a script) | ✅ 129 chapters, ch. 1 = 105 pages |
| MangaRead | All three | HTTP (Madara POST list) | ✅ 133 chapters, ch. 1 = 41 pages |
| Toonily | Manhwa | HTTP | ✅ 145 chapters, ch. 1 = 10 pages |
| ManhuaPlus | Manhua | HTTP (image list via ajax, ordered by `data-index`) | ✅ 941 chapters, ch. 1 = 214 pages |
| Flame Comics | Manhwa | HTTP (`__NEXT_DATA__`) | ✅ 226 chapters, ch. 1 = 34 pages |
| WEBTOON | Manhwa (official) | HTTP (mobile episode API) | ✅ Tower of God: 652 episodes, ep. 1 = 8 pages |
| MangaGeko | All three | HTTP | ✅ 15 chapters, ch. 1 = 199 pages |
| MangaNato / MangaKakalot | All three | site window (Cloudflare passed by itself) + chapter API; images over HTTP | ✅ 62 chapters, ch. 1 = 97 pages |
| Asura Scans | Manhwa | site window (Cloudflare sometimes) | ✅ 102 chapters, ch. 1 = 14 pages |

Also checked in the app window: adding from a chapter link, single-chapter download, the strip and paged readers, resume position, update checks that download only new chapters, and CBZ export.

Not supported (checked 2026-11-24): Bato.to (closed), Comick (no longer hosts images), Mangago (encrypted page lists), FanFox/MangaHere (packed scripts). ManhwaTop, KunManga, ManhuaUS, ManhuaFast and ManhwaClan block the Madara chapter-list request behind Cloudflare.
