// core/namespace.js — Global namespace for Network Mapper UI
// All modules attach their public API here to avoid polluting window.

'use strict';

window.NM = {
    state: {},    // shared application state
    data: {},     // topology data helpers
    graph: {},    // Cytoscape wrapper (filled by graph.js)
    ui: {},       // UI panels (toolbar, sidebar, popup, inventory)
    views: {},    // view renderers (fabric, switch, host, vm)
    core: {},     // utilities (escapeHtml, errors, warnings)
};
