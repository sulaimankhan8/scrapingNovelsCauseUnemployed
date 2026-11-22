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

## Verification log (2026-11-22)

| Site | Novel page | Chapter page | TOC | Notes |
|---|---|---|---|---|
| NovelBin | browser | browser | JSON API | The JSON CDN gives 403 to plain HTTP but works in-page. `.com`/`.me` are down (503). |
| NovelFull | ✅ http | ✅ http | 29 pages × 50 | Ad iframes inside the content. |
| ReadNovelFull | ✅ http | ✅ http | ✅ 1 ajax call, 1,432 chapters | |
| Royal Road | ✅ http | ✅ http | ✅ inline | Hidden decoy paragraphs, randomised classes. |
| WuxiaWorld | ✅ http | ✅ http | next-link via embedded JSON | Paid chapters are locked. |
| FreeWebNovel | ✅ http | ✅ http | sequential 1..N from og meta | |
| NovelFire | ✅ http | browser (→ novelphoenix.com) | ✅ paginated ×100 | Mirror domain-hopping. |
| Webnovel | ✅ http | to confirm | `/catalog` | Most chapters paid. |
| EmpireNovel | human-check | human-check | to capture | Selectors TODO. |
| ScribbleHub | human-check | human-check | ajax | Selectors from the known layout, need confirming. |

Test URLs for each site are in the `urls` list in `PLAN.md` (§ Site parsers).
