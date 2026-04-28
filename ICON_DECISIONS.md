# Icon Design Decisions

## Overview

The network-mapper web UI uses **Microsoft Fluent UI System Icons** to represent
device types in the topology graph. This document explains the icon strategy and
the rationale behind key decisions.

## Icon Source

All icons come from the official
[microsoft/fluentui-system-icons](https://github.com/microsoft/fluentui-system-icons)
repository, licensed under MIT. We use 20px variants for clarity at small sizes.

| Device Type | Icon Name                            | Color   |
|-------------|--------------------------------------|---------|
| Switch      | `ic_fluent_router_20_regular`        | #0078d4 |
| Host        | `ic_fluent_server_20_filled`         | #44b700 |
| VM          | `ic_fluent_desktop_20_regular`       | #a36efd |
| BMC         | `ic_fluent_developer_board_20_regular` | #f7630c |

## Why Standalone SVG Files (not inline data URIs or NPM)

We evaluated three approaches for referencing these icons:

### Option 1: NPM package (`@fluentui/svg-icon-web`)
- **Pro:** Automatic updates, large icon library available.
- **Con:** Adds a ~4000-icon dependency for just 4 icons. Our frontend is vanilla
  JS with no bundler (Webpack, Vite, etc.), so consuming NPM packages would
  require adding a build pipeline. Too much overhead.

### Option 2: Inline data URIs in JavaScript
- **Pro:** Zero HTTP requests, everything in one file.
- **Con:** Long SVG path strings clutter the JS code, making it hard to read and
  maintain. Mixing icon art with application logic violates separation of concerns.

### Option 3: Standalone SVG files served from `/img/` ✅ (chosen)
- **Pro:** Clean separation — icons are image assets, JS is application logic.
  Each SVG is self-documenting (includes source comment and license reference).
  Easy to update or swap an icon without touching JS. Cytoscape loads them via
  `background-image` URL, which is the most natural approach.
- **Pro:** The Go binary embeds the entire `web/` directory via `go:embed`, so
  `/img/*.svg` is served same-origin. No CORS issues, no CDN dependency, works
  in air-gapped Azure Local environments.
- **Con:** 4 extra HTTP requests on first load (~5KB total, trivial).

## How Icons Are Used

1. **SVG files** live in `cmd/network-mapper/web/img/`:
   - `icon-switch.svg` — Router icon, filled with Azure blue (#0078d4)
   - `icon-host.svg` — Server icon, filled with green (#44b700)
   - `icon-vm.svg` — Desktop icon, filled with purple (#a36efd)
   - `icon-bmc.svg` — Developer board icon, filled with orange (#f7630c)

2. **graph.js** references them by URL path:
   ```js
   const typeIcons = {
       switch: '/img/icon-switch.svg',
       host:   '/img/icon-host.svg',
       vm:     '/img/icon-vm.svg',
       bmc:    '/img/icon-bmc.svg',
   };
   ```

3. **Cytoscape** renders them as node background images with these key properties:
   ```js
   'background-image': typeIcons.switch,
   'background-fit': 'contain',
   'background-clip': 'node',
   'background-width': '70%',
   'background-height': '70%',
   ```
   The `70%` sizing leaves padding between the icon and the node border.

## Updating Icons

To change an icon:
1. Find the desired icon in the
   [Fluent UI icon catalog](https://github.com/microsoft/fluentui-system-icons/tree/main/assets)
2. Download the SVG (prefer 20px regular or filled variants)
3. Change the `fill` attribute to the device type color
4. Add the source comment header (icon name, license, color rationale)
5. Replace the corresponding file in `web/img/`

No JavaScript changes are needed unless you add a new device type.
