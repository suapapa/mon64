---
name: mon64
description: Ops dashboard framing pixel status badges for Prometheus-scraped nodes
colors:
  bg: "#12121c"
  surface: "#1a1a2e"
  surface-hover: "#22223a"
  badge-frame: "#000000"
  badge-frame-hover: "#050508"
  ink: "#e8e8f0"
  muted: "#9a9aac"
  border: "#2e2e48"
  accent: "#3388ff"
  accent-hover: "#66aaff"
  load-low: "#3388ff"
  load-mid: "#44cc66"
  load-high: "#ffaa33"
  load-crit: "#ff4444"
  error: "#ff4444"
  focus-ring: "#66aaff"
typography:
  title:
    fontFamily: "ui-monospace, Cascadia Code, SF Mono, Menlo, Consolas, monospace"
    fontSize: "1.25rem"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "0.06em"
  body:
    fontFamily: "ui-monospace, Cascadia Code, SF Mono, Menlo, Consolas, monospace"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: "normal"
  label:
    fontFamily: "ui-monospace, Cascadia Code, SF Mono, Menlo, Consolas, monospace"
    fontSize: "0.7rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "0.08em"
  meta:
    fontFamily: "ui-monospace, Cascadia Code, SF Mono, Menlo, Consolas, monospace"
    fontSize: "0.8rem"
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: "normal"
rounded:
  sm: "3px"
  md: "6px"
spacing:
  1: "0.25rem"
  2: "0.5rem"
  3: "0.75rem"
  4: "1rem"
  5: "1.5rem"
components:
  node-row:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0.25rem 0.75rem 0.25rem 0.25rem"
  node-row-hover:
    backgroundColor: "{colors.surface-hover}"
  badge-stack:
    backgroundColor: "{colors.badge-frame}"
    rounded: "{rounded.md}"
    padding: "0.5rem"
  named-badge:
    backgroundColor: "{colors.badge-frame}"
    rounded: "{rounded.md}"
    padding: "0.5rem"
  link:
    textColor: "{colors.accent}"
  link-hover:
    textColor: "{colors.accent-hover}"
  metric-low:
    textColor: "{colors.load-low}"
  metric-mid:
    textColor: "{colors.load-mid}"
  metric-high:
    textColor: "{colors.load-high}"
  metric-crit:
    textColor: "{colors.load-crit}"
---

# Design System: mon64

## Overview

**Creative North Star: "The Rack Panel"**

mon64 is an ops status wall, not a marketing site. The UI frames the real artifact — Tom Thumb pixel PNG badges from `internal/exporter` — the way a dim rack room frames instrument lights. Chrome stays quiet; the badges and load numbers carry the signal.

The palette is cool indigo locked to badge PNG colors (`#1a1a2e` surface, blue→green→orange→red load stops). Motion only reports connection and refresh state. Density is intentional: one masthead, configured named badges, one fleet list.

**Key Characteristics:**
- Dark-only (`color-scheme: dark`); matches badge PNGs
- Single monospace stack for brand, data, and chrome
- Pixel badges are the signature; UI never competes with them
- Load color buckets mirror badge meter stops (≤33 / ≤66 / <90 / ≥90)
- Restrained accent use: links, focus, and live state — not decoration

## Colors

Cool indigo neutrals with a blue accent shared with badge `loadBlue`. Semantic load colors are the same stops as `internal/exporter/png.go`.

### Primary
- **Signal Blue** (`#3388ff`): Links, focus ring sibling, low-load metric values; badge load stop at 0%

### Neutral
- **Rack Void** (`#12121c`): Page background
- **Badge Indigo** (`#1a1a2e`): Row surface; matches PNG badge background
- **Panel Lift** (`#22223a`): Row hover
- **Ink** (`#e8e8f0`): Primary text
- **Muted Rail** (`#9a9aac`): Meta, labels, secondary chrome (AA on surface)
- **Seam** (`#2e2e48`): Borders and dividers
- **Badge Frame** (`#000000` / hover `#050508`): Stack tile behind stacked PNG

### Named load / status
- **Load Low** `#3388ff` · **Load Mid** `#44cc66` · **Load High** `#ffaa33` · **Load Crit / Error** `#ff4444`

**The Badge Match Rule.** Dashboard surfaces and load colors must stay aligned with `internal/exporter` badge PNG colors. Do not invent a second “dashboard-only” accent green.

**The One Accent Rule.** Interactive accent (`#3388ff`) is for links, selection, and focus — not large fills or hero washes.

## Typography

**Body / Title / Label Font:** `ui-monospace, "Cascadia Code", "SF Mono", Menlo, Consolas, monospace`

**Character:** One monospace family keeps the UI continuous with Tom Thumb pixel badges. No display serif; no Inter/system-ui split.

### Hierarchy
- **Title** (700, 1.25rem, tracking 0.06em, lowercase): `mon64` wordmark only
- **Body** (400, 0.9375rem): Default copy and metric values (values at 600 / 1rem)
- **Label** (600, 0.7rem, tracking 0.08em): Metric keys (CPU / GPU / MEM / SWAP)
- **Meta** (400, 0.8rem): Updated timestamp, footer, live status text (~0.75rem)

## Elevation

Flat tonal layering. No drop shadows on rows or cards. Depth comes from `--bg` → `--surface` → `--surface-hover` and 1px `--border` seams. The badge stack uses a true black frame so pixel art reads crisply.

Focus uses a 2px `--focus-ring` outline with 2px offset — never shadow-as-focus.

## Components

### Masthead
Brand + status line. Live indicator states: `connecting` | `live` | `reconnecting` | `error`. Announced via `role="status"` + `aria-live="polite"`.

### Badge stack
Centered, bordered tile; links to stacked PNG. Signature element — keep it dominant and free of overlay chips.

### Node row
Flex row: 128px (2×) badge link + metric chips. Unreachable rows tint with error mix and expose `role="status"` on the error line.

### Metric chip
Label (muted) + value (load-colored via `data-level`: `low` | `mid` | `high` | `crit` | `na`). Numbers remain the primary signal; color is secondary.

### Empty state
Dashed surface panel with concrete next step (add `nodes` in config YAML).

### Links
Accent text, underline on hover; footer API links use ≥44px min-height touch targets.

## Do's and Don'ts

### Do
- Keep the pixel badge as the primary visual and match its indigo/load palette
- Use CSS variables from `:root` in `web/static/style.css` for every color
- Prefer state-driven motion (live pulse) and honor `prefers-reduced-motion`
- Treat missing metrics as `n/a`, never fake `0`

### Don't
- Don't use acid-green-on-black terminal clichés unrelated to badge stops
- Don't add glassmorphism, gradient text, decorative grids, or soft wide card shadows
- Don't put display fonts or fluid clamp heroes on this product surface
- Don't scrape remotes from the request path — the UI only reads store/API snapshots
