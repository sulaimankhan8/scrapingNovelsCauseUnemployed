# Plan: Chinese, Japanese and Korean novels with automatic translation

Decided 2026-11-25. Background and site checks: `reports/Chinese, Korean and Japanese novel sites.md`.

## Status (2026-11-25)

Built: steps 1–7 and the Chinese/Japanese part of step 3 (69shuba, UUKanshu, Syosetu, Kakuyomu); the name list works in the translator and is stored per book, but has no screen to edit it yet. Not yet: the second wave of sites, Korean sites, Kakuyomu search.

## What the user gets

- Search by English title still works. With the **中文 / 日本語 / 한국어** chips turned on, Chinese, Japanese and Korean sites are searched too, and their copies show as extra rows (tagged "translated") in the same card as the English sites, with their chapter counts.
- Downloading from those sites saves the original chapter and an English translation next to it. Translation is automatic, free (Google Translate, the same service as Chrome's "Translate page"), needs no account or key, and happens once per chapter.
- The reader opens in English, with an **EN / 原文** switch. EPUB export is in English. The novel's title, author and description are translated too; the original title is kept.
- An optional per-novel name list (原文 → English) fixes names that come out differently from chapter to chapter. Off by default.

## How languages are known (hybrid)

1. **Pre-vetted sites.** Each site profile in `sites.json` has a `language` field (`en`, `zh`, `ja`, `ko`; missing means `en`). Only tested sites are added. English sites are never translated.
2. **Detection, as a safety net.** For a pasted link from an unknown site, and for every chapter before translating: count the scripts in the text. Hangul means Korean; kana (with or without Chinese characters) means Japanese; Chinese characters without kana or Hangul mean Chinese; mostly Latin letters means English, which is never translated. The page's `<html lang>` and charset are hints.

## Steps

1. **Text encodings.** Decode GBK / GB18030 / Big5 / Shift_JIS / EUC-JP / EUC-KR pages to UTF-8, in the HTTP downloader (from the `Content-Type` header or the page's `<meta charset>`) and in the site browser's in-page fetch (decode with the page's own charset).
2. **Language labels and detection.** `language` on profiles, `Language` on the novel and the stored book, `internal/lang` with script detection.
3. **Sites.** 69shuba (Chinese, site window, GBK), Shousetsuka ni Narou / syosetu.com (Japanese, HTTP, official search API) and Kakuyomu (Japanese, HTTP). Each with a search block and a live test in `na test`. Later waves, each only after a live test: uukanshu, Piaotia, the free chapters of Zongheng / 17k / JJWXC, AlphaPolis, Hameln, and the Korean free sites (Naver Web Novel, Munpia, Joara).
4. **Translation.** `internal/translate`: Google's free web endpoint, sending paragraphs in batches of up to ~4,500 characters, paced (one request at a time, a short pause between), retrying with backoff on HTTP 429, and stopping for a while if Google keeps refusing. The chapter's HTML structure (paragraphs, bold, system boxes) is kept: each paragraph's text is translated and put back in place. The name list is applied before sending (Google leaves English words alone).
5. **Storage.** The original stays at `chapters/0001.html`; the English goes to `chapters/0001.en.html`. `book.json` gains `language`, `originalTitle`, `translate` (on/off), the name list, and per chapter `titleOriginal` and `translated`. A failed translation never fails the chapter: the original is saved, and the next update or "Translate missing" tries again.
6. **Reader and export.** The reader shows the English file when there is one, with an EN / 原文 switch that is remembered. The contents page and the library show English titles. EPUB export uses English (original as a fallback per chapter).
7. **Search.** Language chips in the search bar (English on by default; the others are remembered). For Chinese sites the app needs the Chinese title, which machine translation gets wrong, so it looks it up: (a) from the English sites' own results, which often carry the Chinese author name (NovelBin: "爱潜水的乌贼"), searching the Chinese site for that author; (b) Google's translation of the English title as a second guess; (c) typing Chinese works directly. Chinese results are shown with translated titles.
8. **Testing and release.** Unit tests for encodings, detection, the translator's HTML round trip and batching; `na test` for every new site; the app driven in the desktop dev build and the Android simulator; then the exe and the APK.

## Limits

- Google's free endpoint is not an official API for apps. It can slow down or refuse heavy use; the app paces itself and pauses when refused, and the original text is always saved.
- Paid chapters (Qidian VIP, Naver Series, KakaoPage) stay out of reach; the app never gets around paywalls or logins.
