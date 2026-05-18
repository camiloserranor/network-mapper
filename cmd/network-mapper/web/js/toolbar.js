// toolbar.js — Toolbar controls: search, export, refresh

'use strict';

NM.ui.Toolbar = (() => {
    function init() {
        // Search
        const searchInput = document.getElementById('search-input');
        searchInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                const q = searchInput.value.trim().toLowerCase();
                if (!q) return;
                const devices = (NM.state.topology && NM.state.topology.devices) || [];
                const match = devices.find(d =>
                    (d.system_name || '').toLowerCase().includes(q) ||
                    d.id.toLowerCase().includes(q) ||
                    (d.chassis_id || '').toLowerCase().includes(q)
                );
                if (match) {
                    if (match.type === 'switch') NM.state.ViewManager.navigateTo('switch', match.id);
                    else if (match.type === 'host') NM.state.ViewManager.navigateTo('host', match.id);
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
            btn.innerHTML = NM.core.icons.refresh + ' Refreshing...';
            try {
                const resp = await fetch('/api/topology');
                if (resp.ok) {
                    var rawTopology = await resp.json();
                    var topology = NM.data.adaptV2(rawTopology);
                    NM.state.topology = topology;
                    NM.state.rawTopology = rawTopology;
                    NM.core.showWarnings(topology.partial_failures);
                    NM.ui.Sidebar.setTopology(topology);
                    NM.ui.Popup.setTopology(topology);
                    NM.ui.Inventory.update();
                    NM.graph.setTopology(topology);
                    NM.state.ViewManager.renderCurrentView();
                }
            } catch (err) {
                console.error('Refresh failed:', err);
            } finally {
                btn.disabled = false;
                btn.innerHTML = NM.core.icons.refresh + ' Refresh';
            }
        });

        // Export JSON
        document.getElementById('export-json-btn').addEventListener('click', () => {
            NM.graph.exportJSON();
        });

        // Theme toggle
        const themeBtn = document.getElementById('theme-toggle-btn');
        if (themeBtn) {
            const saved = localStorage.getItem('nm-theme');
            if (saved === 'dark') {
                document.documentElement.setAttribute('data-theme', 'dark');
            }

            themeBtn.addEventListener('click', function() {
                const current = document.documentElement.getAttribute('data-theme');
                if (current === 'dark') {
                    document.documentElement.removeAttribute('data-theme');
                    localStorage.setItem('nm-theme', 'light');
                } else {
                    document.documentElement.setAttribute('data-theme', 'dark');
                    localStorage.setItem('nm-theme', 'dark');
                }
            });
        }
    }

    return { init };
})();
