// toolbar.js — Toolbar controls: layout switching, search, export

'use strict';

const Toolbar = (() => {
    let currentTopology = null;

    function init() {
        // Layout buttons
        document.querySelectorAll('#layout-buttons .btn').forEach((btn) => {
            btn.addEventListener('click', () => {
                const layout = btn.dataset.layout;
                if (!layout) return;

                // Update active state
                document.querySelectorAll('#layout-buttons .btn').forEach((b) => b.classList.remove('active'));
                btn.classList.add('active');

                NetworkGraph.runLayout(layout);
            });
        });

        // Search
        const searchInput = document.getElementById('search-input');
        let searchTimeout = null;
        searchInput.addEventListener('input', () => {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                NetworkGraph.searchNodes(searchInput.value);
            }, 200);
        });

        // Fit
        document.getElementById('fit-btn').addEventListener('click', () => {
            NetworkGraph.fitToScreen();
        });

        // Export PNG
        document.getElementById('export-png-btn').addEventListener('click', () => {
            NetworkGraph.exportPNG();
        });

        // Export JSON
        document.getElementById('export-json-btn').addEventListener('click', () => {
            NetworkGraph.exportJSON();
        });

        // Collapse all
        document.getElementById('collapse-all-btn').addEventListener('click', () => {
            NetworkGraph.collapseAll();
        });

        // Group by VLAN
        document.getElementById('vlan-group-btn').addEventListener('click', () => {
            NetworkGraph.toggleGroupByVLAN(currentTopology);
            const btn = document.getElementById('vlan-group-btn');
            btn.classList.toggle('active', NetworkGraph.isVLANGrouped());
        });
    }

    function updateBadge(topology) {
        // Store reference for VLAN grouping
        currentTopology = topology;
    }

    return { init, updateBadge };
})();
