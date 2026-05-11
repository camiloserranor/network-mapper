// app.js — Entry point: fetch topology, init modules, start first view

'use strict';

(async function () {
    try {
        const topology = await fetchTopology();
        NM.state.topology = topology;

        NM.ui.Toolbar.init();
        NM.ui.Timeline.init();
        NM.core.showWarnings(topology.partial_failures);

        // Initialize Cytoscape (provides export and fabric view canvas)
        NM.graph.init('cy', [], topology);

        NM.ui.Sidebar.init(topology);
        NM.ui.Popup.init(topology);
        NM.ui.Inventory.init();

        // Render initial view
        NM.state.ViewManager.navigateToFabric();

        // Connect WebSocket for live updates
        connectWebSocket();

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

function connectWebSocket() {
    var protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    var url = protocol + '//' + location.host + '/api/ws';
    var ws;

    function connect() {
        ws = new WebSocket(url);

        ws.onmessage = function(evt) {
            try {
                var msg = JSON.parse(evt.data);

                if (msg.type === 'topology_update' && msg.topology) {
                    // Only apply if in live mode
                    if (NM.ui.Timeline.isLive()) {
                        NM.state.topology = msg.topology;
                        NM.core.showWarnings(msg.topology.partial_failures);
                        NM.ui.Sidebar.setTopology(msg.topology);
                        NM.ui.Popup.setTopology(msg.topology);
                        NM.ui.Inventory.update();
                        NM.graph.setTopology(msg.topology);
                        NM.state.ViewManager.renderCurrentView();
                    }
                } else if (msg.type === 'snapshot_list' && msg.snapshots) {
                    NM.ui.Timeline.onSnapshotListUpdate(msg.snapshots);
                }
            } catch (e) {
                console.error('WebSocket message parse error:', e);
            }
        };

        ws.onclose = function() {
            // Reconnect after 3 seconds
            setTimeout(connect, 3000);
        };

        ws.onerror = function() {
            ws.close();
        };
    }

    connect();
}
