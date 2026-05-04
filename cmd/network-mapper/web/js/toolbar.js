// toolbar.js — Toolbar controls: search, export, refresh, force layout toggle

'use strict';

const Toolbar = (() => {
    function init() {
        // Search
        const searchInput = document.getElementById('search-input');
        searchInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                const q = searchInput.value.trim().toLowerCase();
                if (!q) return;
                // Search devices and navigate to first match
                const devices = (currentTopology && currentTopology.devices) || [];
                const match = devices.find(d =>
                    (d.system_name || '').toLowerCase().includes(q) ||
                    d.id.toLowerCase().includes(q) ||
                    (d.chassis_id || '').toLowerCase().includes(q)
                );
                if (match) {
                    if (match.type === 'switch') ViewManager.navigateTo('switch', match.id);
                    else if (match.type === 'host') ViewManager.navigateTo('host', match.id);
                }
            }
        });

        // Fit → scroll to top
        const fitBtn = document.getElementById('fit-btn');
        if (fitBtn) fitBtn.addEventListener('click', () => {
            document.getElementById('detail-view').scrollTop = 0;
        });

        // Refresh
        document.getElementById('refresh-btn').addEventListener('click', async () => {
            const btn = document.getElementById('refresh-btn');
            btn.disabled = true;
            btn.textContent = '\u21BB Refreshing...';
            try {
                const resp = await fetch('/api/topology');
                if (resp.ok) {
                    const topology = await resp.json();
                    currentTopology = topology;
                    showWarnings(topology.partial_failures);
                    Sidebar.setTopology(topology);
                    Popup.setTopology(topology);
                    Inventory.update(topology);
                    NetworkGraph.setTopology(topology);
                    ViewManager.renderCurrentView();
                }
            } catch (err) {
                console.error('Refresh failed:', err);
            } finally {
                btn.disabled = false;
                btn.textContent = '\u27F3 Refresh';
            }
        });

        // Export JSON
        document.getElementById('export-json-btn').addEventListener('click', () => {
            NetworkGraph.exportJSON();
        });
    }

    return { init };
})();
