# Site parser profiles

`sites.json` describes how to read each supported site. The Go core loads it at startup, so fixing a site that changed its layout means editing JSON, with no rebuild. Users can also drop extra profiles into `<AppData>/NovelArchiver/sites/`.

## How a pasted link is handled

```
user pastes URL
   │
   ├─ matches a profile's  input.novelUrl   → novel page
   ├─ matches a profile's  input.chapterUrl → chapter page
   │       └─ find the novel page via input.novelFromChapter
   │          (a link on the chapter page, or a rewrite of the URL)
   │          └─ if that fails: crawl forward with chapter.next from this chapter
   └─ matches nothing → "generic" profile (JSON-LD / og: meta / readability)
```

The user can paste **either the novel description page or any chapter page**, including the first one. When a chapter page is given, the app offers two choices:
- "Whole novel": go to the TOC.
- "From this chapter onward": follow the next links.

## Fields

| Field | Meaning |
|---|---|
| `status` | `verified`: tested with plain HTTP. `partial`: some pages need the browser, or some chapters are paid. `browser-only`: works only inside a real browser. `human-check`: Cloudflare checkbox the user must tick once. |
| `fetchMode` | `http`: Go HTTP client. `browser`: load inside the app's hidden WebView, which shares the user's cookies. `http+browser`: per stage. |
| selector syntax | CSS selector. `@attr` reads an attribute instead of text. `a, b` means try in order. `x ~ regex` applies a regex to the value. |
| `toc.strategy` | `inline` (all on one page), `paginated`, `ajax`, `api` (JSON), `sequential` (`chapter-1..N`), `next-link` (follow "Next"), `generic`. |
| `chapter.remove` | Elements removed before sanitising. |
| `removeHiddenByPageCss` | Drop elements that the page's own CSS hides (Royal Road decoys). |

## Bot checks

The app never tries to bypass a bot check. Sites marked `browser` / `human-check` open in a visible in-app browser window. The user ticks the checkbox themselves. The app then reads pages through that same browser session at a polite rate.

## Verification log (2026-11-22, live downloads)

| Site | How it's fetched | Result |
|---|---|---|
| NovelFull | plain HTTP | ✅ list (1,432) + chapters |
| ReadNovelFull | plain HTTP | ✅ list (1,432, one ajax call) + chapters |
| FreeWebNovel | plain HTTP | ✅ list (776, sequential) + chapters |
| WuxiaWorld | plain HTTP | ✅ follows "next" links; paid chapters stop the crawl |
| NovelBin | site browser (JSON CDN via in-page fetch) | ✅ list (1,432) + chapters, system boxes kept |
| NovelFire | list over HTTP, chapters in site browser | ✅ |
| Royal Road | site browser | ✅ list + chapters, decoy lines removed |
| Webnovel | site browser | ✅ catalog (1,274; 1,239 paid skipped) + free chapters |
| EmpireNovel | site browser | ✅ numbered list (2,531) + chapters; Cloudflare check passed in the window |
| ScribbleHub | site browser | ✅ follows "Next" from chapter 1 (its TOC is 15 per page, newest first) |

To fix or add a site, save the page as the engine sees it and probe it:

```
go run ./cmd/na dump <url> > page.html
go run ./cmd/na inspect <url>
```
