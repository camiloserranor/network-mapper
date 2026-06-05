// ui/inventory.js — Inventory panel: grouped device list with search and navigation

'use strict';

NM.ui.Inventory = (function() {
    let filterText = '';

    const groupConfig = [
        { type: 'switch',  label: 'Switches',  color: '#0078d4' },
        { type: 'host',    label: 'Hosts',      color: '#44b700' },
        { type: 'bmc',     label: 'BMCs',       color: '#f7630c' },
        { type: 'unknown', label: 'Unknown',    color: '#8a8886' },
        { type: 'vm',      label: 'VMs',        color: '#a36efd' },
    ];

    const collapsedGroups = new Set(['vm']);

    function init() {
        wireEvents();
        render();
    }

    function update() {
        render();
    }

    function wireEvents() {
        const toggleBtn = document.getElementById('inventory-toggle');
        const expandBtn = document.getElementById('inventory-expand-btn');
        const panel = document.getElementById('inventory-panel');

        if (toggleBtn) {
            toggleBtn.addEventListener('click', () => {
                panel.classList.add('collapsed');
                expandBtn.classList.remove('hidden');
            });
        }
        if (expandBtn) {
            expandBtn.addEventListener('click', () => {
                panel.classList.remove('collapsed');
                expandBtn.classList.add('hidden');
            });
        }

        const filterInput = document.getElementById('inventory-search');
        if (filterInput) {
            filterInput.addEventListener('input', () => {
                filterText = filterInput.value.toLowerCase();
                render();
            });
        }
    }

    function render() {
        const container = document.getElementById('inventory-groups');
        const topology = NM.state.topology;
        if (!container || !topology) return;

        const devices = topology.devices || [];
        const endpoints = topology.endpoints || [];
        const esc = NM.core.escapeHtml;

        let html = '';
        for (const group of groupConfig) {
            let items;
            if (group.type === 'vm') {
                items = endpoints.map(ep => ({
                    id: 'vm-' + ep.mac.replace(/:/g, ''),
                    name: (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac,
                    type: 'vm',
                }));
            } else {
                items = devices.filter(d => d.type === group.type).map(d => ({
                    id: d.id,
                    name: d.system_name || d.id,
                    type: d.type,
                }));
            }

            if (filterText) {
                items = items.filter(i => i.name.toLowerCase().includes(filterText) || i.id.toLowerCase().includes(filterText));
            }
            if (items.length === 0 && !filterText) continue;

            const isCollapsed = collapsedGroups.has(group.type);
            html += '<div class="inv-group' + (isCollapsed ? ' collapsed' : '') + '">';
            html += '<div class="inv-group-header" data-type="' + group.type + '">';
            html += '<span class="inv-group-chevron">\u25BE</span>';
            html += '<span class="inv-group-dot" style="background:' + group.color + '"></span>';
            html += '<span>' + group.label + '</span>';
            html += '<span class="inv-group-count">' + items.length + '</span>';
            html += '</div>';

            html += '<div class="inv-group-items">';
            for (const item of items) {
                html += '<div class="inv-item" data-id="' + esc(item.id) + '" data-type="' + item.type + '">';
                html += '<span class="inv-item-label">' + esc(item.name) + '</span>';
                html += '</div>';
            }
            html += '</div></div>';
        }

        container.innerHTML = html;

        // Wire group toggle
        container.querySelectorAll('.inv-group-header').forEach(header => {
            header.addEventListener('click', () => {
                const type = header.dataset.type;
                if (collapsedGroups.has(type)) collapsedGroups.delete(type);
                else collapsedGroups.add(type);
                render();
            });
        });

        // Wire item click → navigate
        container.querySelectorAll('.inv-item').forEach(item => {
            item.addEventListener('click', () => {
                const id = item.dataset.id;
                const type = item.dataset.type;
                if (type === 'switch') NM.state.ViewManager.navigateTo('switch', id);
                else if (type === 'host') NM.state.ViewManager.navigateTo('host', id);
                else if (type === 'bmc') NM.state.ViewManager.navigateTo('bmc', id);
                else if (type === 'vm') NM.state.ViewManager.navigateTo('vm', id);
            });
        });
    }

    return { init, update };
})();
