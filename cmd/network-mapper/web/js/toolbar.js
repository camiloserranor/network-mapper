// toolbar.js — Toolbar controls: layout switching, search, filter, export

'use strict';

const Toolbar = (() => {
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

        // Filter
        document.getElementById('filter-select').addEventListener('change', (e) => {
            NetworkGraph.filterByType(e.target.value);
        });

        // Fit
        document.getElementById('fit-btn').addEventListener('click', () => {
            NetworkGraph.fitToScreen();
        });

        // Export
        document.getElementById('export-btn').addEventListener('click', () => {
            NetworkGraph.exportPNG();
        });

        // Group by TOR
        document.getElementById('group-btn').addEventListener('click', () => {
            const isGrouped = NetworkGraph.toggleGroupByTOR();
            const btn = document.getElementById('group-btn');
            btn.classList.toggle('active', isGrouped);
        });
    }

    function updateBadge(topology) {
        const badge = document.getElementById('info-badge');
        if (!topology) {
            badge.textContent = 'No data';
            return;
        }

        const deviceCount = (topology.devices || []).length;
        const linkCount = (topology.links || []).length;
        const failureCount = (topology.partial_failures || []).length;

        let text = `${deviceCount} devices · ${linkCount} links`;

        if (topology.collected_at) {
            const collected = new Date(topology.collected_at);
            const ago = timeSince(collected);
            text += ` · ${ago}`;
        }

        if (failureCount > 0) {
            text += ` · ⚠ ${failureCount} failure${failureCount > 1 ? 's' : ''}`;
        }

        badge.textContent = text;
    }

    function timeSince(date) {
        const seconds = Math.floor((new Date() - date) / 1000);
        if (seconds < 60) return 'just now';
        const minutes = Math.floor(seconds / 60);
        if (minutes < 60) return `${minutes}m ago`;
        const hours = Math.floor(minutes / 60);
        if (hours < 24) return `${hours}h ago`;
        const days = Math.floor(hours / 24);
        return `${days}d ago`;
    }

    return { init, updateBadge };
})();
