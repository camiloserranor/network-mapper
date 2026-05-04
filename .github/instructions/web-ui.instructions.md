---
applyTo: "cmd/network-mapper/web/**"
---

# Web UI Instructions

- This is a vanilla HTML/CSS/JS frontend — no frameworks, no npm, no bundler.
- All third-party libraries are vendored in `web/lib/` and loaded via `<script>` tags.
- The topology graph is rendered with Cytoscape.js using a multi-view architecture with three scoped views.
- The web UI fetches data from `/api/topology` (JSON) served by the Go backend.
- Follow the existing dark theme: background `#1b1a19`, graph area `#0d1117`, accent `#0078d4` (Azure portal blue).
- Node types and their styles:
  - **switch**: blue `#0078d4`, round-rectangle, with SVG icon
  - **host**: green `#44b700`, round-rectangle, with SVG icon
  - **bmc**: orange `#f7630c`, round-rectangle, with SVG icon
  - **vm**: purple `#a36efd`, round-rectangle, with SVG icon
  - **unknown**: gray `#8a8886`, ellipse
- Multi-view architecture:
  - **Fabric View** (default): shows only switches with host-count badges, spine on top, leaf below
  - **Switch Detail**: shows a switch + its connected hosts with VM-count badges
  - **Host Detail**: shows a host + connected switches above + VMs below (capped at 100)
  - Navigation: double-click or breadcrumb bar to drill down/up
- JS modules and their responsibilities:
  - `app.js` — entry point, ViewManager, view renderers (fabric/switch/host), Inventory panel, data helpers
  - `graph.js` — Cytoscape initialization, styles, layout (dagre/cose), search, export
  - `sidebar.js` — detail panel for selected nodes/edges
  - `toolbar.js` — refresh button, force layout toggle, search, fit, export buttons
  - `popup.js` — floating card shown on node/edge click with drill-down button
- When adding new UI features, extend the existing module that owns that concern rather than creating new files.
