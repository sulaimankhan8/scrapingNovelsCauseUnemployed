# Design Decisions Log

## 2026-11-01 - Direction Selected: Editorial Bibliophile
- **Why**: The user explicitly selected the *Editorial Bibliophile* direction (warm paper, deep ink tones, literary typography, and balanced shelf layout) over modern developer or terminal aesthetics.
- **Impact**: All colors, fonts, card styles, and copy transition from generic dark-mode SaaS components to an intentional literary archive.

## 2026-11-01 - Semantic Token Architecture & Contrast Normalization
- **Why**: Mechanical audit flagged raw hex colors, inconsistent border-radii (8 distinct values), and insufficient contrast (< 4.5:1) on primary buttons, card links, and headers.
- **Impact**: Defined a single source of truth in CSS custom properties (`:root`):
  - `--bg-canvas`: deep leather slate in dark mode, cream linen in light mode.
  - `--bg-surface`: raised book card cloth.
  - `--text-main`: high-contrast warm vellum / printer's ink (contrast > 12:1).
  - `--text-muted`: folio annotation gray (contrast > 4.8:1).
  - `--accent`: terracotta wax seal (`#c2410c` / `#b4442a`, contrast > 4.6:1 against dark and light surfaces).
  - Radius unified to a clean 3-tier scale: `--radius-sm` (4px), `--radius-md` (8px), `--radius-lg` (14px), and `--radius-pill` (9999px).

## 2026-11-01 - Icon Standardization (Replaced Emojis with Semantic SVGs)
- **Why**: Raw emojis render inconsistently across Windows, macOS, Android, and Linux, look playful rather than archival, and fail accessibility screen-reader checks.
- **Impact**: Replaced all raw emojis in `index.html` and `app.js` with crisp, inline monochrome SVG icons with proper `aria-hidden="true"` and descriptive button labels or `aria-label` attributes.

## 2026-11-01 - Motion & Accessibility Guards
- **Why**: Audit flagged pulse animations and lack of reduced-motion checks.
- **Impact**: Wrapped all transitions and spin animations in `@media (prefers-reduced-motion: no-preference)`. Ensured keyboard focus `:focus-visible` is prominent and distinctive with an offset terracotta focus ring.
