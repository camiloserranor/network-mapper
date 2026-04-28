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
            // Deactivate VLAN group if TOR group is toggled on
            if (isGrouped) {
                document.getElementById('vlan-group-btn').classList.remove('active');
            }
        });

        // Group by VLAN
        document.getElementById('vlan-group-btn').addEventListener('click', () => {
            NetworkGraph.toggleGroupByVLAN(currentTopology);
            const btn = document.getElementById('vlan-group-btn');
            btn.classList.toggle('active', NetworkGraph.isVLANGrouped());
            // Deactivate TOR group if VLAN group is toggled on
            if (NetworkGraph.isVLANGrouped()) {
                document.getElementById('group-btn').classList.remove('active');
            }
        });

        // VLAN filter dropdown
        const vlanFilter = document.getElementById('vlan-filter');
        if (vlanFilter) {
            vlanFilter.addEventListener('change', (e) => {
                const vlanId = e.target.value;
                if (vlanId === '') {
                    // Show all
                    NetworkGraph.filterByType('all');
                } else {
                    filterByVLAN(parseInt(vlanId));
                }
            });
        }
    }

    function filterByVLAN(vlanId) {
        const cy = NetworkGraph.getInstance();
        if (!cy) return;

        cy.elements().forEach(ele => {
            const vlans = ele.data('vlans') || [];
            if (ele.isNode()) {
                if (vlans.includes(vlanId) || ele.data('type') === 'switch') {
                    ele.style('display', 'element');
                } else {
                    ele.style('display', 'none');
                }
            }
        });
        // Show edges where both endpoints are visible
        cy.edges().forEach(e => {
            const srcVisible = e.source().style('display') !== 'none';
            const tgtVisible = e.target().style('display') !== 'none';
            e.style('display', (srcVisible && tgtVisible) ? 'element' : 'none');
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
        const endpointCount = (topology.endpoints || []).length;
        const vlanCount = (topology.vlans || []).length;
        const failureCount = (topology.partial_failures || []).length;

        let text = `${deviceCount} devices · ${linkCount} links`;
        if (endpointCount > 0) text += ` · ${endpointCount} VMs`;
        if (vlanCount > 0) text += ` · ${vlanCount} VLANs`;

        if (topology.collected_at) {
            const collected = new Date(topology.collected_at);
            const ago = timeSince(collected);
            text += ` · ${ago}`;
        }

        if (failureCount > 0) {
            text += ` · ⚠ ${failureCount} failure${failureCount > 1 ? 's' : ''}`;
        }

        badge.textContent = text;

        // Populate VLAN filter dropdown
        populateVLANFilter(topology);
    }

    function populateVLANFilter(topology) {
        const vlanFilter = document.getElementById('vlan-filter');
        if (!vlanFilter) return;

        const currentVal = vlanFilter.value;
        vlanFilter.innerHTML = '<option value="">All VLANs</option>';

        const vlans = topology.vlans || [];
        vlans.sort((a, b) => a.id - b.id);
        for (const v of vlans) {
            const opt = document.createElement('option');
            opt.value = v.id;
            opt.textContent = v.name ? `VLAN ${v.id} — ${v.name}` : `VLAN ${v.id}`;
            vlanFilter.appendChild(opt);
        }

        // Restore previous selection if still valid
        if (currentVal) vlanFilter.value = currentVal;
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
