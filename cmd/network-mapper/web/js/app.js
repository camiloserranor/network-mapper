// app.js — Entry point: fetch topology, init modules, start first view

'use strict';

(async function () {
    try {
        const topology = await fetchTopology();
        NM.state.topology = topology;

        NM.ui.Toolbar.init();
        NM.core.showWarnings(topology.partial_failures);

        // Initialize Cytoscape (provides export and fabric view canvas)
        NM.graph.init('cy', [], topology);

        NM.ui.Sidebar.init(topology);
        NM.ui.Popup.init(topology);
        NM.ui.Inventory.init();

        // Render initial view
        NM.state.ViewManager.navigateToFabric();

    } catch (err) {
        NM.core.showError(err.message);
    }
})();

async function fetchTopology() {
    const resp = await fetch('/api/topology');
    if (!resp.ok) {
        let msg = 'HTTP ' + resp.status;
        try { const body = await resp.json(); if (body.error) msg = body.error; } catch (_) {}
        throw new Error(msg);
    }
    return resp.json();
}
