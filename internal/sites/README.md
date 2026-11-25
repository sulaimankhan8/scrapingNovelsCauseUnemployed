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
| `language` | What the site's novels are written in: `en` (default), `zh`, `ja`, `ko`. Novels in other languages are saved as written and translated to English (Google Translate) as they download; see `design/foreign-novels-plan.md`. |
| `toc.sortAttr` | Order the chapter list by this number on each item or an ancestor (69shuba's HTML lists newest first, with `data-num`). |
| `toc.strategy: kakuyomu` | Kakuyomu's chapter list, read from the work page's `__NEXT_DATA__` script. |
| `novel.altNames` | Selector for the page's "Alternative names". Searching Chinese / Japanese sites uses the original title found here. |
| `query` | Added to every request to the site. AO3 uses `view_adult=true` so Mature works show their text. |
| `novel.api` + `fields` | JSON novel details. Field paths can be dotted (`user.name`) and `a\|b` tries each in turn. |
| `toc.listPath`, `toc.urlField` | For `toc.strategy: api`: where the chapter array sits in the JSON (Wattpad: `parts`) and which field holds each chapter's link. |
| `chapter.idFromUrl`, `chapter.format` | For `chapter.strategy: api`: a regex whose group 1 (taken from the chapter link) becomes `{chapterId}`, and `html` when the API answers with the chapter's HTML instead of JSON. |

## Search by title

A profile with a `search` block can be searched from the add-novel field: type a title instead of a link. Every searchable site is searched at once, and the results are grouped so each novel shows once with a row per site (chapter count, latest chapter, Count / Preview / Get).

| Field | Meaning |
|---|---|
| `search.page` | The site's search URL; `{query}` is replaced by the URL-escaped title. |
| `search.fetchMode` | `http` or `browser`, like the other stages. Browser searches run one at a time, after the HTTP ones have shown their results. |
| `search.optIn` | `true` when the search page usually shows a "verify you are human" check. Such sites are only searched when the user taps "Search them too". |
| `search.item` | One element per result. |
| `search.title`, `search.link` | The result's title and novel link. Links that don't match `input.novelUrl` are dropped, so every result can be downloaded. |
| `search.cover`, `search.author`, `search.latest` | Optional extras. |
| `search.chapters` | Optional number shown as the chapter count: the count, or the latest chapter's number, from the results page. "Count" replaces it with the real count from the chapter list. |
| `search.json` | For search APIs that answer JSON (Wattpad, Syosetu): `path` to the result array and dotted paths for `title`, `link`, `author`, `cover`, `chapters`. A value with `{field}` is a template (`https://ncode.syosetu.com/{ncode\|lower}/`). |

Searchable today (verified 2026-11-25): NovelFull, ReadNovelFull, NovelPing, FreeWebNovel, NovelFire, Wattpad and Archive of Our Own over HTTP; WebNovel through the site browser; Royal Road and ScribbleHub on request. The first results show after at most 6 seconds; slower sites join when they answer. NovelBin's search runs inside its page from a CDN list, and WuxiaWorld's is client-side, so neither has a `search` block yet.

Chinese and Japanese sites (UUKanshu, Syosetu) are searched only when the **中文 / 日本語** chips are on, or when the title is typed in Chinese or Japanese. An English title is turned into the original one through the English sites' "Alternative names" (NovelFull, ReadNovelFull), with Google's translation as a last guess. A search page that answers an exact title with the novel itself (UUKanshu) counts as a single result. 69shuba's search shows a Cloudflare check every time, so it isn't searched; paste its links.

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

## Checking every site

```
go run ./cmd/na test                 # every site: discover the example, fetch its first free chapter, search for its title
go run ./cmd/na test -site wattpad   # one site
go run ./cmd/na test -optin          # also the searches that ask for a human check
```

Results on 2026-11-25: NovelBin, NovelFull, ReadNovelFull, NovelPing, WuxiaWorld, FreeWebNovel, WebNovel, Wattpad and Archive of Our Own passed every check. Royal Road, EmpireNovel and ScribbleHub (and NovelFire's chapters) stopped at a "verify you are human" check, which needs a person to tick it in the site window. AO3 rate-limits hard: a burst of requests gets HTTP 429 for a few minutes.
