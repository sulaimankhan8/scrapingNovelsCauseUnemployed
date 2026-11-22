# Design brief: NovelArchiver (Web Novel Archival & Reading Suite)

Tag every line: [Given] from the user, [Found: link] from research, [Assumed] otherwise.

## Product
- What it is: A private web novel archiver and distraction-free offline reading suite. [Given]
- Who uses it (primary group, expertise, context of use): Enthusiasts of serialized web novels (fantasy, xianxia, sci-fi, horror) who binge hundreds or thousands of chapters, often on tablets, laptops, and mobile screens in bed or during commutes. [Found: r/noveltranslations, RoyalRoad reader surveys]
- Main job-to-be-done: Paste a novel URL, reliably download and sanitize all chapters, and provide a reading experience as pleasant as a well-bound book. [Given]
- What it must never do or imply: It must never feel like a generic SaaS developer utility, crypto dashboard, or ad-stuffed pirate site. [Given]

## Evidence
| Item | Type | Source | Confidence |
|---|---|---|---|
| User preference for "Editorial Bibliophile" aesthetic | [Given] | Direct user selection in turn 2 | High |
| User preference for "Balanced Shelf" layout | [Given] | Direct user selection in turn 2 | High |
| Readers hate intrusive ads, anti-adblock modals, and broken pagination | [Found] | https://www.reddit.com/r/noveltranslations/ | High |
| Readers value typography, font scaling, warm paper sepia, and scroll tracking | [Found] | Readwise Reader & Apple Books design reviews | High |
| Scrapers frequently break on anti-bot/Cloudflare with zero feedback | [Found] | Calibre FanFicFare issues tracker | High |

Top user pains (ranked, with source):
1. **Unreadable web formatting & ad clutter:** Web novel sites cram animated banners, inline fake chapters, and trackers that drain battery and strain eyes. [Found: r/noveltranslations]
2. **Missing or corrupted chapters:** Serial scrapers silently save empty or Cloudflare challenge pages, ruining plot arcs. [Found: project codebase audit]
3. **Clinical, uninviting software:** Existing offline tools (Calibre, CLI scripts) feel like spreadsheet tools rather than personal libraries. [Assumed]

## Competitors & references
- Competitors (3):
  1. *Calibre + FanFicFare plugin*: Deeply capable archiver, but complex UI designed like a 2005 Windows utility. [Found: calibre-ebook.com]
  2. *Readwise Reader*: Gold standard for editorial type, keyboard navigation, and archival reading flow. [Found: readwise.io/read]
  3. *Apple Books*: Exquisite shelf presentation, realistic cover shadows, typography-first reading salon. [Found: apple.com/apple-books]
- References (4–6):
  1. *Readwise Reader*: Studied for typography hierarchy and keyboard navigation (`←`/`→`/`H`). Adapt: persistent reading position and calm typography.
  2. *Standard Ebooks*: Studied for cover typography and editorial standard. Adapt: clean chapter headings and typography ornaments.
  3. *Penguin Clothbound Classics*: Studied for world-derived tactile aesthetic (cloth textures, foil stamping, deckle edge paper).
  4. *Linear App*: Studied for keyboard focus states, subtle status badges, and zero clutter.
- Category conventions to keep: Top URL paste input, cover thumbnails, chapter count indicator, dark/light reading modes.
- Category clichés to break: No neon purple/indigo gradients, no generic floating stat cards with fake percentages, no emojis masquerading as professional interface icons, no clinical white cards over dark gray void.

## Screen(s)
For the main screen (Archive Dashboard & Library Shelf):
- One job: Archive any novel with one paste, and launch immediately into offline reading.
- Primary action: "Archive Novel" (single-click download & build).
- Tier 1 (Must see now): Active Library Shelf (book covers, chapter counts, "Read" & "Re-do" actions) + Link Archiver input bar.
- Tier 2 (Supporting): Discovered novel metadata card (synopsis, author, total chapters) + live archival progress bar.
- Tier 3 (On demand): Collapsible technical inspector / terminal drawer, chapter range filters, audit & repair dialog.
- States:
  - *Empty library:* Warm bibliophile welcome with sample novel link to try.
  - *Inspecting:* Subtle shimmer badge, disabled inspect button with spinner.
  - *Archiving active:* Live chapter counter and calm progress bar.
  - *Archival complete:* Toast alert and shelf refresh.
  - *Error:* Clear, human explanation with actionable next step.
- Domain vocabulary: "Archive", "Library Shelf", "Volume", "Chapter", "Sync with Source", "Typeset", "Format Reader".

## Story
- Hero / Problem / Stakes / Turn / Resolution:
  - *Hero:* A fiction enthusiast with 10 ongoing web serials.
  - *Problem:* Web novel sites are littered with ads, slow loading, and risk of random takedowns.
  - *Stakes:* Losing reading progress or straining eyes during late-night binge sessions.
  - *Turn:* NovelArchiver transforms raw, ad-ridden web pages into permanent, beautiful offline volumes in seconds.
  - *Resolution:* A private, permanent personal library ready to read anywhere with zero ads and flawless typography.
- Headline: "Your Private Web Novel Bibliotheca"

## Direction
- Thesis: It should feel like a dedicated bibliophile's personal archival reading salon, never a generic developer utility or SaaS dashboard.
- World objects (palette source): Bookcloth binding, vellum paper, printer's black ink, terracotta wax seals, foil stamp brass.
- Palette:
  - Vellum Cream (`#faf7f2`) & Deep Ink Slate (`#12161a`)
  - Accent Terracotta Wax (`#c2410c` / `#b4442a`)
  - Subtle Linen Line (`#2a323d` / `#e5dfd5`)
- Type:
  - Display & Headings: `Newsreader`, `Georgia`, serif
  - Interface & Controls: `Plus Jakarta Sans`, system-ui, sans-serif
  - Chapter & Metadata: `ui-monospace`, `JetBrains Mono`
- Signature element: **"Clothbound Shelf & Ribbon Progress"** — visual book spines with foil accents and elegant reading progress indicators.
- Layout from data shape: Book-shelf grid reflecting physical library volumes, topped with a calm folio archiver bar.

## Constraints
- Stack: HTML5, CSS3, Vanilla JS, Node.js HTTP server.
- Devices / breakpoints: Mobile (320px–480px), Tablet (768px), Desktop (1024px–1440px+).
- Accessibility target: WCAG 2.2 AA (minimum 4.5:1 text contrast, visible focus rings, aria labels on icon buttons).
