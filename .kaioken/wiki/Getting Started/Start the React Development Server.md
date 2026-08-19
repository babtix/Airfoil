# Start the React Development Server

This chapter explains how to launch the Vite development server for the Airfoil site, which pages are available at runtime, and how the site consumes the JSON indexes and Markdown stories produced by the Go agent.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Launching the Dev Server](#launching-the-dev-server)
- [Available Pages & Routes](#available-pages--routes)
- [Data Consumption: JSON Index & Markdown Stories](#data-consumption-json-index--markdown-stories)
- [Build vs. Dev Behavior](#build-vs-dev-behavior)
- [Referenced Files](#referenced-files)

---

## Prerequisites

Before starting the dev server, ensure the Go agent has run at least once so that the data artifacts exist:

| Artifact | Path | Produced By |
|----------|------|-------------|
| Story index | `data/index.json` | `./airfoil run` |
| Story markdown files | `site/src/content/stories/*.md` | `./airfoil run` |
| Deduplication state | `data/state.json` | `./airfoil run` |
| Raw items | `data/items/*.json` | `./airfoil run` |

The site reads **only** `data/index.json` and `site/src/content/stories/*.md` at build time. The dev server serves the same files through Vite's module graph.

---

## Launching the Dev Server

From the repository root:

```bash
cd site
npm install        # first time only
npm run dev
```

The `dev` script is defined in `package.json` as a direct call to `vite`:

`site/package.json:7-9`

```json
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "oxlint",
    "preview": "vite preview"
  }
```

Vite starts on `http://localhost:5173` by default. The configuration is minimal:

`site/vite.config.ts:1-10`

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
})
```

No custom aliases, proxy rules, or build-time plugins are configured. The React plugin handles JSX/TSX transpilation and Fast Refresh.

---

## Available Pages & Routes

The React Router setup (defined in `site/src/App.tsx`, not in the provided source files but described in the architecture brief) exposes the following routes:

| Route | Page Component | Purpose |
|-------|----------------|---------|
| `/` | `HomePage` | Landing page with featured stories |
| `/feed` | `FeedPage` | Dense list of all stories (~25 per desktop screen) |
| `/story/:slug` | `StoryPage` | Full story view with summary, sources, tags |
| `/search` | `SearchPage` | Client-side search (only page with client JS) |
| `/top` | `TopPage` | Highest-scored stories across all time |
| `/ship` | `ShipPage` | Stories tagged with shipping/releases |
| `/digest` | `DigestPage` | Periodic digest view |

All pages except `/search` render with **zero client-side JavaScript** — they are fully server-rendered at build time and served as static HTML. The `/search` page hydrates a small search index for client-side filtering.

### Component Hierarchy (per architecture brief)

```
App (Router)
├── Header
├── Layout
│   ├── LeftRail (navigation, filters)
│   ├── Main
│   │   ├── ActiveFilterBar
│   │   └── [Page Content]
│   │       ├── StoryCard[] (Feed, Home, Top, Ship, Digest)
│   │       └── StoryDetail (StoryPage)
│   └── RightRail (metadata, related)
└── Footer
```

Shared components: `TagPill`, `ScoreBadge`, `AirfoilSVG`, `MobileOverlay`.

Custom hooks: `useAirfoilSignals` (filter state machine: date window, tag, source, query, sort), `useTheme` (dark/light).

---

## Data Consumption: JSON Index & Markdown Stories

### At Build Time (`npm run build`)

1. `tsc -b` type-checks the project.
2. `vite build` bundles the application.
3. During the build, Vite imports:
   - `data/index.json` → typed as `IndexEntry[]` (mirrors Go `model.IndexEntry`)
   - `site/src/content/stories/*.md` → each file's frontmatter + body become a `Story` object (mirrors Go `model.Story`)

The TypeScript types in `site/src/types/story.ts` are kept in sync with the Go frontmatter:

```typescript
// site/src/types/story.ts (per architecture brief)
interface Story {
  slug: string;
  title: string;
  date: string;           // ISO 8601
  tier: 'major' | 'notable' | 'minor';
  tags: string[];
  sources: Source[];      // { name, type, url }
  clusterId: string;
  summary: string;        // LLM-generated markdown
  // ...other frontmatter fields
}
```

### At Dev Time (`npm run dev`)

Vite serves the same files through its dev server. Hot Module Replacement (HMR) updates components on edit, but **the data files are not watched for changes**. If you re-run the Go agent (`./airfoil run`) while the dev server is running, you must refresh the browser to see new stories.

> **Note**: The dev server does not re-run the Go agent. The agent and the site are completely separate processes. The typical workflow:
> 1. Run `./airfoil run` to generate fresh content.
> 2. Start `npm run dev` to view the site.
> 3. Edit React/TypeScript/CSS → HMR updates instantly.
> 4. To pick up new agent output, stop dev server, re-run agent, restart dev server (or just refresh if only `data/index.json` and markdown changed).

---

## Build vs. Dev Behavior

| Aspect | `npm run dev` | `npm run build` |
|--------|---------------|-----------------|
| TypeScript checking | No (relies on IDE/editor) | Yes (`tsc -b`) |
| Minification | No | Yes |
| Source maps | Yes | Yes (configurable) |
| Data source | `data/index.json` + `site/src/content/stories/*.md` (served by Vite) | Same files, inlined into bundle |
| Client JS on `/search` | Hydrated | Hydrated |
| Client JS on other pages | None (static HTML) | None (static HTML) |
| Output | Served from memory | `site/dist/` (static assets) |

The `preview` script (`vite preview`) serves the production build locally for verification before deployment.

---

## Referenced Files

- `site/package.json` — scripts, dependencies, devDependencies
- `site/vite.config.ts` — Vite configuration (React plugin only)
- Architecture brief (authoritative context) — page/component inventory, data flow, type definitions, routing structure

<!-- kaioken:files site/package.json,site/vite.config.ts -->
