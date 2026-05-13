---
applyTo: "cmd/network-mapper/web/**"
---

# Web UI Instructions

## Stack & Constraints

- Vanilla HTML/CSS/JS — no frameworks, no npm, no bundler, no ES modules.
- All third-party libraries are vendored in `web/lib/` and loaded via `<script>` tags.
- The web UI fetches data from `/api/topology` (JSON) served by the Go backend.
- Script load order matters: namespace → core → data → graph → UI → views → app.

## Architecture: Namespace-Based Module Pattern

All modules attach to the global `window.NM` namespace object (defined in `core/namespace.js`).

```
NM
├── core      — escapeHtml, showError, showWarnings
├── state     — topology (shared data), ViewManager (navigation)
├── data      — pure data functions (classifySwitches, buildPortMap, etc.)
├── graph     — Cytoscape wrapper (init, render, layout, export)
├── ui        — Sidebar, Popup, Toolbar, Inventory
└── views     — renderFabric, renderSwitch, renderHost, renderVM
```

### File Structure

```
web/js/
├── core/
│   ├── namespace.js    — bootstraps window.NM structure
│   ├── utils.js        — escapeHtml, showError, showWarnings
│   └── state.js        — ViewManager (navigation, breadcrumb)
├── data/
│   └── topology.js     — pure data helpers (no DOM, no state)
├── ui/
│   └── inventory.js    — inventory panel with search + navigation
├── views/
│   ├── fabric.js       — fabric overview (Cytoscape compound nodes)
│   ├── switch.js       — switch detail (port diagram)
│   ├── host.js         — host detail (NIC diagram + VM cloud)
│   └── vm.js           — VM info card
├── graph.js            — Cytoscape init, styles, render, export
├── sidebar.js          — detail panel for nodes/edges
├── popup.js            — floating card on click
├── toolbar.js          — search, refresh, export buttons
└── app.js              — slim entry point (fetch, init, go)
```

## Design Principles

1. **SOLID** — each module has a single responsibility.
2. **Dependency direction** — views depend on data + state; data depends on nothing.
3. **No global variables** — everything goes through `NM.*` namespace.
4. **Pure data layer** — functions in `data/topology.js` take topology as input, return results, never touch DOM.

## Multi-View Architecture

- **Fabric View** (default): Cytoscape compound nodes — switches with port children, spine on top, leaf below.
- **Switch Detail**: HTML port diagram with hover tooltips, click-to-navigate.
- **Host Detail**: HTML NIC cards + VM cloud with chips.
- **VM Detail**: HTML info card with location links.
- Navigation: click ports/nodes, breadcrumb bar, or inventory panel.

## Visual Theme

- Azure portal-inspired: toolbar uses deep blue (`#0c2340`) in dark mode, with a 2px blue accent line at the top.
- Light theme: neutral backgrounds with azure blue (`#0078d4`) accent.
- Device colors:
  - **switch**: `#0078d4` (blue)
  - **host**: `#44b700` (green)
  - **bmc**: `#f7630c` (orange)
  - **vm**: `#a36efd` (purple)
  - **unknown**: `#8a8886` (gray)

## Adding New Features

- New views: create a file in `views/`, attach render function to `NM.views.*`, register in ViewManager.
- New data helpers: add to `data/topology.js` on `NM.data.*`.
- New UI components: create in `ui/` or extend existing sidebar/popup/toolbar.
- Always reference other modules through `NM.*` — never use bare function names.
