---
applyTo: "cmd/network-mapper/web/**"
---

# Web UI Instructions

- This is a vanilla HTML/CSS/JS frontend — no frameworks, no npm, no bundler.
- All third-party libraries are vendored in `web/lib/` and loaded via `<script>` tags.
- The topology graph is rendered with Cytoscape.js using a custom 3-tier hierarchical layout (BMC top, switches middle, hosts bottom).
- The web UI fetches data from `/api/topology` (JSON) served by the Go backend.
- Follow the existing dark theme: background `#1a1a2e`, graph area `#0d1117`, accent `#e94560`.
- Node types and their styles:
  - **switch**: blue `#2196F3`, round-rectangle
  - **host**: green `#4CAF50`, ellipse
  - **bmc**: orange `#FF9800`, diamond
- JS modules and their responsibilities:
  - `app.js` — entry point, fetches topology, transforms data, wires events
  - `graph.js` — Cytoscape initialization, layout, hover effects, compound grouping, export
  - `sidebar.js` — detail panel for selected nodes/edges
  - `toolbar.js` — layout switcher, search, filter, fit, group toggle, export button
  - `popup.js` — floating card shown on node/edge click
- When adding new UI features, extend the existing module that owns that concern rather than creating new files.
