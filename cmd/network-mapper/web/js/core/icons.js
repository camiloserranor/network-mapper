// core/icons.js — SVG icon definitions (Azure portal style)
// All icons use fill="currentColor" to inherit text color.

'use strict';

NM.core.icons = {
    // Network switch — simplified switch icon (rectangle with ports)
    switch: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M2 3.5A1.5 1.5 0 013.5 2h9A1.5 1.5 0 0114 3.5v9a1.5 1.5 0 01-1.5 1.5h-9A1.5 1.5 0 012 12.5v-9zM3.5 3a.5.5 0 00-.5.5v9a.5.5 0 00.5.5h9a.5.5 0 00.5-.5v-9a.5.5 0 00-.5-.5h-9zM5 5.5a.5.5 0 01.5-.5h1a.5.5 0 010 1h-1a.5.5 0 01-.5-.5zm4 0a.5.5 0 01.5-.5h1a.5.5 0 010 1h-1a.5.5 0 01-.5-.5zM5 8a.5.5 0 01.5-.5h1a.5.5 0 010 1h-1A.5.5 0 015 8zm4 0a.5.5 0 01.5-.5h1a.5.5 0 010 1h-1A.5.5 0 019 8zm-4 2.5a.5.5 0 01.5-.5h1a.5.5 0 010 1h-1a.5.5 0 01-.5-.5zm4 0a.5.5 0 01.5-.5h1a.5.5 0 010 1h-1a.5.5 0 01-.5-.5z"/></svg>',

    // Host/Server — server rack icon
    host: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M3 2.5A1.5 1.5 0 014.5 1h7A1.5 1.5 0 0113 2.5v2A1.5 1.5 0 0111.5 6h-7A1.5 1.5 0 013 4.5v-2zm1.5-.5a.5.5 0 00-.5.5v2a.5.5 0 00.5.5h7a.5.5 0 00.5-.5v-2a.5.5 0 00-.5-.5h-7zM5 3.5a.5.5 0 11-1 0 .5.5 0 011 0zM3 7.5A1.5 1.5 0 014.5 6h7A1.5 1.5 0 0113 7.5v2A1.5 1.5 0 0111.5 11h-7A1.5 1.5 0 013 9.5v-2zm1.5-.5a.5.5 0 00-.5.5v2a.5.5 0 00.5.5h7a.5.5 0 00.5-.5v-2a.5.5 0 00-.5-.5h-7zM5 8.5a.5.5 0 11-1 0 .5.5 0 011 0zM4.5 11A1.5 1.5 0 003 12.5v1A1.5 1.5 0 004.5 15h7a1.5 1.5 0 001.5-1.5v-1a1.5 1.5 0 00-1.5-1.5h-7zm-.5 1.5a.5.5 0 01.5-.5h7a.5.5 0 01.5.5v1a.5.5 0 01-.5.5h-7a.5.5 0 01-.5-.5v-1zM5 13.5a.5.5 0 11-1 0 .5.5 0 011 0z"/></svg>',

    // Virtual Machine — monitor icon
    vm: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M2.5 2A1.5 1.5 0 001 3.5v7A1.5 1.5 0 002.5 12H6v1H4.5a.5.5 0 000 1h7a.5.5 0 000-1H10v-1h3.5a1.5 1.5 0 001.5-1.5v-7A1.5 1.5 0 0013.5 2h-11zM2 3.5a.5.5 0 01.5-.5h11a.5.5 0 01.5.5v7a.5.5 0 01-.5.5h-11a.5.5 0 01-.5-.5v-7zM7 12h2v1H7v-1z"/></svg>',

    // BMC — chip/processor icon
    bmc: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M5 1a.5.5 0 01.5.5v1.03A4.5 4.5 0 019.5 2.5V1.5a.5.5 0 011 0v1.03a4.5 4.5 0 011.97.97H13.5a.5.5 0 010 1h-1.03a4.5 4.5 0 010 5H13.5a.5.5 0 010 1h-1.03a4.5 4.5 0 01-.97.97V13.5a.5.5 0 01-1 0v-1.03a4.5 4.5 0 01-5 0v1.03a.5.5 0 01-1 0v-1.03A4.5 4.5 0 013.53 11.5H2.5a.5.5 0 010-1h1.03a4.5 4.5 0 010-5H2.5a.5.5 0 010-1h1.03A4.5 4.5 0 015 3.53V1.5A.5.5 0 015 1zm.5 3A3.5 3.5 0 002 7.5 3.5 3.5 0 005.5 11h5A3.5 3.5 0 0014 7.5 3.5 3.5 0 0010.5 4h-5z"/></svg>',

    // Network/VLAN — globe/network icon
    network: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1a7 7 0 100 14A7 7 0 008 1zM2 8a6 6 0 0111.94-.92H10.5a.5.5 0 010-1h3.44A6 6 0 002 8zm4.5-4.93V6.5a.5.5 0 01-1 0V3.07A6 6 0 002.06 7h3.44a.5.5 0 010 1H2.06a6 6 0 003.44 3.93V8.5a.5.5 0 011 0v3.43a6 6 0 003 0V8.5a.5.5 0 011 0v3.43A6 6 0 0013.94 9h-3.44a.5.5 0 010-1h3.44a6 6 0 00-3.44-4.93V6.5a.5.5 0 01-1 0V3.07a6 6 0 00-3 0z"/></svg>',

    // Warning — triangle with exclamation
    warning: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M7.134 1.496a1 1 0 011.732 0l6.267 10.847A1 1 0 0114.267 14H1.733a1 1 0 01-.866-1.5L7.134 1.496zM8 5a.5.5 0 00-.5.5v3a.5.5 0 001 0v-3A.5.5 0 008 5zm0 6.5a.75.75 0 100-1.5.75.75 0 000 1.5z"/></svg>',

    // Clock — for timeline
    clock: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 2a6 6 0 100 12A6 6 0 008 2zM1 8a7 7 0 1114 0A7 7 0 011 8zm7.5-3.5a.5.5 0 00-1 0V8a.5.5 0 00.146.354l2 2a.5.5 0 00.708-.708L8.5 7.793V4.5z"/></svg>',

    // History — clock with arrow
    history: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1a7 7 0 11-4.95 2.05.5.5 0 01.7.71A6 6 0 108 2V4.5a.5.5 0 01-.854.354l-2-2a.5.5 0 010-.708l2-2A.5.5 0 018 .5V1z"/><path d="M8.5 4.5a.5.5 0 00-1 0V8a.5.5 0 00.146.354l2 2a.5.5 0 00.708-.708L8.5 7.793V4.5z"/></svg>',

    // Inventory/List — clipboard icon
    list: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M2.5 3a.5.5 0 000 1h11a.5.5 0 000-1h-11zm0 3a.5.5 0 000 1h11a.5.5 0 000-1h-11zm0 3a.5.5 0 000 1h11a.5.5 0 000-1h-11zm0 3a.5.5 0 000 1h11a.5.5 0 000-1h-11z"/></svg>',

    // Logo — hexagonal network node
    logo: '<svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor"><path d="M10 1l7.66 4.42v8.84L10 18.68l-7.66-4.42V5.42L10 1zm0 1.15L3.34 5.96v8.08L10 17.53l6.66-3.49V5.96L10 2.15z"/><circle cx="10" cy="10" r="2.5"/><path d="M10 4v3.5M10 12.5V16M4.5 7L7.5 8.75M12.5 11.25L15.5 13M4.5 13L7.5 11.25M12.5 8.75L15.5 7" stroke="currentColor" stroke-width="1" fill="none"/></svg>',

    // Refresh — arrow sync
    refresh: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M11.534 7h3.932a.25.25 0 01.192.41l-1.966 2.36a.25.25 0 01-.384 0l-1.966-2.36A.25.25 0 0111.534 7zm-7.068 2H.534a.25.25 0 00-.192.41l1.966 2.36a.25.25 0 00.384 0l1.966-2.36A.25.25 0 004.466 9z"/><path d="M8 3a5 5 0 014.546 2.914.5.5 0 00.908-.418A6 6 0 002 8a6 6 0 00.442 2.27.5.5 0 10.936-.35A5 5 0 018 3zm0 10a5 5 0 01-4.546-2.914.5.5 0 00-.908.418A6 6 0 0014 8a6 6 0 00-.442-2.27.5.5 0 10-.936.35A5 5 0 018 13z"/></svg>'
};

// Helper to get icon HTML with optional size override.
NM.core.icon = function(name, size) {
    var svg = NM.core.icons[name] || '';
    if (size && svg) {
        svg = svg.replace(/width="\d+"/, 'width="' + size + '"')
                 .replace(/height="\d+"/, 'height="' + size + '"');
    }
    return svg;
};
