// app.js — Main entry point: fetch topology, transform to Cytoscape elements, wire everything up

'use strict';

let currentTopology = null;

(async function () {
    try {
        const topology = await fetchTopology();
        currentTopology = topology;

        Toolbar.init();
        Toolbar.updateBadge(topology);
        showWarnings(topology.partial_failures);

        // Dismiss tree hint
        const treeHintDismiss = document.getElementById('tree-hint-dismiss');
        if (treeHintDismiss) {
            treeHintDismiss.addEventListener('click', () => {
                document.getElementById('tree-hint').classList.add('hidden');
            });
        }

        const elements = topologyToCytoscape(topology);
        const cy = NetworkGraph.init('cy', elements, topology);

        Sidebar.init(topology);
        Popup.init(topology);
        Inventory.init(topology);

        // Single click → show popup; double-click → expand/collapse
        cy.on('tap', 'node', (evt) => {
            const node = evt.target;
            Popup.showForNode(node.data(), node.renderedPosition());
        });

        cy.on('dbltap', 'node', (evt) => {
            const node = evt.target;
            Popup.hide();
            NetworkGraph.toggleExpand(node.id());
        });

        // Click on edge → show popup card near midpoint
        cy.on('tap', 'edge', (evt) => {
            const edge = evt.target;
            Popup.showForEdge(edge.data(), edge.renderedMidpoint());
        });

        // Click on background → hide popup and sidebar
        cy.on('tap', (evt) => {
            if (evt.target === cy) {
                Popup.hide();
                Sidebar.hide();
            }
        });

        // Hide popup on drag/zoom
        cy.on('viewport', () => {
            if (Popup.isVisible()) Popup.hide();
        });

        // Start WebSocket connection for live updates
        LiveConnection.init((newTopology) => {
            currentTopology = newTopology;
            Toolbar.updateBadge(newTopology);
            showWarnings(newTopology.partial_failures);
            Sidebar.setTopology(newTopology);
            Popup.setTopology(newTopology);
            Inventory.update(newTopology);
            NetworkGraph.setTopology(newTopology);
            NetworkGraph.updateElements(topologyToCytoscape(newTopology));
        });

    } catch (err) {
        showError(err.message);
    }
})();

async function fetchTopology() {
    const resp = await fetch('/api/topology');

    if (!resp.ok) {
        let msg = `HTTP ${resp.status}`;
        try {
            const body = await resp.json();
            if (body.error) msg = body.error;
        } catch (_) {}
        throw new Error(msg);
    }

    return resp.json();
}

// Classify switches as 'spine' or 'leaf'.
// Spine: connected only to other switches. Leaf: connected to at least one non-switch.
function classifySwitches(topology) {
    const deviceTypes = {};
    for (const d of (topology.devices || [])) {
        deviceTypes[d.id] = d.type || 'unknown';
    }

    const switchNeighborTypes = {}; // switchId → Set of neighbor types
    for (const link of (topology.links || [])) {
        const lt = deviceTypes[link.local_device] || 'unknown';
        const rt = deviceTypes[link.remote_device] || 'unknown';

        if (lt === 'switch') {
            if (!switchNeighborTypes[link.local_device]) switchNeighborTypes[link.local_device] = new Set();
            switchNeighborTypes[link.local_device].add(rt);
        }
        if (rt === 'switch') {
            if (!switchNeighborTypes[link.remote_device]) switchNeighborTypes[link.remote_device] = new Set();
            switchNeighborTypes[link.remote_device].add(lt);
        }
    }

    const roles = {}; // deviceId → 'spine' | 'leaf'
    for (const d of (topology.devices || [])) {
        if (d.type !== 'switch') continue;
        const neighbors = switchNeighborTypes[d.id] || new Set();
        // Spine if only connected to switches (or nothing)
        const hasNonSwitch = [...neighbors].some(t => t !== 'switch');
        roles[d.id] = hasNonSwitch ? 'leaf' : 'spine';
    }
    return roles;
}

// Build the parent→children adjacency from topology links.
// Returns { parentId: Set<childId> } where parent is always the higher-tier device.
function buildChildMap(topology) {
    const typeRank = { bmc: 0, spine: 0, leaf: 1, switch: 1, host: 2, unknown: 3, vm: 4 };
    const switchRoles = classifySwitches(topology);

    const deviceTypes = {};
    for (const d of (topology.devices || [])) {
        if (d.type === 'switch') {
            deviceTypes[d.id] = switchRoles[d.id] || 'leaf';
        } else {
            deviceTypes[d.id] = d.type || 'unknown';
        }
    }

    const children = {}; // parentId → Set<childId>
    for (const link of (topology.links || [])) {
        const lt = deviceTypes[link.local_device] || 'unknown';
        const rt = deviceTypes[link.remote_device] || 'unknown';
        const lr = typeRank[lt] ?? 3;
        const rr = typeRank[rt] ?? 3;

        // Lower rank = higher tier (parent). Equal rank = no parent-child.
        if (lr < rr) {
            if (!children[link.local_device]) children[link.local_device] = new Set();
            children[link.local_device].add(link.remote_device);
        } else if (rr < lr) {
            if (!children[link.remote_device]) children[link.remote_device] = new Set();
            children[link.remote_device].add(link.local_device);
        }
    }

    // VMs are children of their host_device
    for (const ep of (topology.endpoints || [])) {
        if (ep.host_device) {
            const epId = 'vm-' + ep.mac.replace(/:/g, '');
            if (!children[ep.host_device]) children[ep.host_device] = new Set();
            children[ep.host_device].add(epId);
        }
    }

    return children;
}

// Count VMs per host from endpoints array
function countVMsPerHost(topology) {
    const vmCounts = {};
    for (const ep of (topology.endpoints || [])) {
        if (ep.host_device) {
            vmCounts[ep.host_device] = (vmCounts[ep.host_device] || 0) + 1;
        }
    }
    return vmCounts;
}

function topologyToCytoscape(topology) {
    const elements = [];

    // Compute child map and VM counts for expand labels
    const childMap = buildChildMap(topology);
    const vmCounts = countVMsPerHost(topology);

    // Detect spine switches: switches that are only connected to other switches (not hosts/VMs/BMCs)
    const switchIds = new Set((topology.devices || []).filter(d => d.type === 'switch').map(d => d.id));
    const switchConnectsNonSwitch = new Set();
    for (const link of (topology.links || [])) {
        const localIsSwitch = switchIds.has(link.local_device);
        const remoteIsSwitch = switchIds.has(link.remote_device);
        if (localIsSwitch && !remoteIsSwitch) switchConnectsNonSwitch.add(link.local_device);
        if (remoteIsSwitch && !localIsSwitch) switchConnectsNonSwitch.add(link.remote_device);
    }

    // Devices → nodes (no VMs — they are created on-demand when expanding a host)
    for (const device of (topology.devices || [])) {
        const ifaces = device.interfaces || [];
        const ifacesUp = ifaces.filter((i) => i.oper_status === 'UP').length;
        const childCount = (childMap[device.id] ? childMap[device.id].size : 0);

        // Determine role: spine switches only connect to other switches
        let role = '';
        if (device.type === 'switch') {
            role = switchConnectsNonSwitch.has(device.id) ? 'leaf' : 'spine';
        }

        elements.push({
            data: {
                id: device.id,
                label: device.system_name || device.id,
                type: device.type || 'unknown',
                role: role,
                chassis_id: device.chassis_id || '',
                system_name: device.system_name || '',
                system_description: device.system_description || '',
                mgmt_addr: device.management_address || '',
                software_version: device.software_version || '',
                uptime: device.uptime || '',
                interfaces_up: ifacesUp,
                interfaces_total: ifaces.length,
                vlans: device.vlans || [],
                annotations: device.annotations || {},
                vmCount: vmCounts[device.id] || 0,
                childCount: childCount,
            },
        });
    }

    // Links → edges (only device-to-device, no VM edges)
    for (const link of (topology.links || [])) {
        const edgeLabel = `${link.local_port || '?'} ↔ ${link.remote_port || '?'}`;
        elements.push({
            data: {
                id: `${link.local_device}::${link.local_port}::${link.remote_device}::${link.remote_port}`,
                source: link.local_device,
                target: link.remote_device,
                local_port: link.local_port || '',
                remote_port: link.remote_port || '',
                remote_chassis_id: link.remote_chassis_id || '',
                source_type: link.source || 'lldp',
                discovered_at: link.discovered_at || '',
                edgeLabel: edgeLabel,
                oper_status: link.oper_status || '',
                speed: link.speed || '',
                mtu: link.mtu || '',
            },
        });
    }

    return elements;
}

function showError(message) {
    const overlay = document.getElementById('error-overlay');
    const msgEl = document.getElementById('error-message');
    overlay.classList.remove('hidden');
    msgEl.textContent = message;
}

// ---- Partial failure warnings ----

let dismissedSignature = null;

function showWarnings(failures) {
    const banner = document.getElementById('warning-banner');
    const summary = document.getElementById('warning-summary');
    const details = document.getElementById('warning-details');
    const toggle = document.getElementById('warning-toggle');

    if (!failures || failures.length === 0) {
        banner.classList.add('hidden');
        return;
    }

    // Build a stable signature so dismissal persists until failures change
    const sig = failures.map(f => `${f.switch}|${f.phase}|${f.message}`).sort().join('\n');
    if (sig === dismissedSignature) {
        // User dismissed this exact set — keep hidden
        return;
    }

    // Group failures by switch
    const groups = {};
    for (const f of failures) {
        const key = f.switch || 'unknown';
        if (!groups[key]) groups[key] = [];
        groups[key].push(f);
    }

    const switchNames = Object.keys(groups);
    const unreachable = switchNames.filter(s => groups[s].some(f => f.phase === 'connect'));
    const degraded = switchNames.filter(s => !groups[s].some(f => f.phase === 'connect'));

    // Summary text
    const parts = [];
    if (unreachable.length > 0) {
        parts.push(`${unreachable.length} switch${unreachable.length > 1 ? 'es' : ''} unreachable`);
    }
    if (degraded.length > 0) {
        parts.push(`${degraded.length} switch${degraded.length > 1 ? 'es' : ''} with partial data`);
    }
    summary.textContent = parts.join(', ') + ' — topology may be incomplete';

    // Build detail HTML, unreachable switches first
    const sortedNames = [...unreachable, ...degraded];
    let html = '';
    for (const sw of sortedNames) {
        const isUnreachable = unreachable.includes(sw);
        const sevClass = isUnreachable ? 'unreachable' : 'degraded';
        const sevLabel = isUnreachable ? 'Unreachable' : 'Partial data';

        html += `<div class="warning-switch-group">`;
        html += `<div class="warning-switch-name">`;
        html += `<span class="severity-dot ${sevClass}" title="${sevLabel}"></span>`;
        html += `${escapeHtml(sw)}`;
        html += `</div>`;

        for (const f of groups[sw]) {
            html += `<div class="warning-phase">`;
            html += `<span class="phase-label">${escapeHtml(f.phase)}:</span> ${escapeHtml(f.message)}`;
            html += `</div>`;
        }
        html += `</div>`;
    }
    details.innerHTML = html;

    // Wire toggle
    toggle.onclick = () => {
        details.classList.toggle('hidden');
        toggle.textContent = details.classList.contains('hidden') ? '▾' : '▴';
    };

    // Wire dismiss
    document.getElementById('warning-dismiss').onclick = () => {
        dismissedSignature = sig;
        banner.classList.add('hidden');
        details.classList.add('hidden');
        toggle.textContent = '▾';
    };

    banner.classList.remove('hidden');
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// ---- Inventory panel ----

const Inventory = (() => {
    let topology = null;
    let filterText = '';

    // Device type display config — order matters (determines group order)
    const groupConfig = [
        { type: 'switch',  label: 'Switches',  color: '#0078d4' },
        { type: 'host',    label: 'Hosts',      color: '#44b700' },
        { type: 'bmc',     label: 'BMCs',       color: '#f7630c' },
        { type: 'unknown', label: 'Unknown',    color: '#8a8886' },
        { type: 'vm',      label: 'VMs',        color: '#a36efd' },
    ];

    // Track which groups are collapsed (VMs collapsed by default)
    const collapsedGroups = new Set(['vm']);

    function init(topologyData) {
        topology = topologyData;
        render();
        wireEvents();
    }

    function update(topologyData) {
        topology = topologyData;
        render();
    }

    function wireEvents() {
        // Panel collapse/expand toggle
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

        // Filter input
        const searchInput = document.getElementById('inventory-search');
        if (searchInput) {
            let timeout = null;
            searchInput.addEventListener('input', () => {
                clearTimeout(timeout);
                timeout = setTimeout(() => {
                    filterText = searchInput.value.toLowerCase().trim();
                    render();
                }, 150);
            });
        }
    }

    function render() {
        const container = document.getElementById('inventory-groups');
        if (!container || !topology) return;

        // Build device lists by type
        const byType = {};
        for (const g of groupConfig) {
            byType[g.type] = [];
        }

        for (const device of (topology.devices || [])) {
            const type = device.type || 'unknown';
            if (!byType[type]) byType[type] = [];
            const label = device.system_name || device.id;
            byType[type].push({ id: device.id, label: label, type: type });
        }

        // Add VMs from endpoints
        for (const ep of (topology.endpoints || [])) {
            const epId = 'vm-' + ep.mac.replace(/:/g, '');
            const label = (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac;
            byType['vm'].push({ id: epId, label: label, type: 'vm', hostDevice: ep.host_device || '' });
        }

        let html = '';
        for (const g of groupConfig) {
            let items = byType[g.type] || [];

            // Apply filter
            if (filterText) {
                items = items.filter(item =>
                    item.label.toLowerCase().includes(filterText) ||
                    item.id.toLowerCase().includes(filterText)
                );
            }

            // Sort alphabetically
            items.sort((a, b) => a.label.localeCompare(b.label));

            const isCollapsed = collapsedGroups.has(g.type);
            const totalCount = (byType[g.type] || []).length;
            const shownCount = items.length;
            const countLabel = filterText && shownCount !== totalCount
                ? `${shownCount}/${totalCount}`
                : `${totalCount}`;

            html += `<div class="inv-group${isCollapsed ? ' collapsed' : ''}" data-type="${g.type}">`;
            html += `<div class="inv-group-header" data-type="${g.type}">`;
            html += `<span class="inv-group-chevron">▾</span>`;
            html += `<span class="inv-group-dot" style="background:${g.color}"></span>`;
            html += `<span>${g.label}</span>`;
            html += `<span class="inv-group-count">${countLabel}</span>`;
            html += `</div>`;
            html += `<div class="inv-group-items">`;

            // Limit items shown for large groups (VMs can be thousands)
            const maxShow = g.type === 'vm' ? 100 : 500;
            const displayItems = items.slice(0, maxShow);

            for (const item of displayItems) {
                html += `<div class="inv-item" data-id="${esc(item.id)}" data-type="${item.type}" title="${esc(item.id)}">`;
                html += `<span class="inv-item-label">${esc(item.label)}</span>`;
                html += `<span class="inv-item-buttons">`;
                html += `<button class="inv-btn inv-btn-details" data-id="${esc(item.id)}" data-type="${item.type}" title="Show details">ℹ</button>`;
                html += `<button class="inv-btn inv-btn-expand" data-id="${esc(item.id)}" data-type="${item.type}" title="Find in graph">⊞</button>`;
                html += `</span>`;
                html += `</div>`;
            }

            if (items.length > maxShow) {
                html += `<div class="inv-item" style="color:var(--text-muted);font-style:italic;cursor:default;">`;
                html += `… and ${items.length - maxShow} more`;
                html += `</div>`;
            }

            html += `</div></div>`;
        }

        container.innerHTML = html;

        // Wire click handlers on items and group headers
        container.querySelectorAll('.inv-group-header').forEach(header => {
            header.addEventListener('click', () => {
                const type = header.dataset.type;
                const group = header.closest('.inv-group');
                if (group.classList.contains('collapsed')) {
                    group.classList.remove('collapsed');
                    collapsedGroups.delete(type);
                } else {
                    group.classList.add('collapsed');
                    collapsedGroups.add(type);
                }
            });
        });

        // Wire details button handlers
        container.querySelectorAll('.inv-btn-details').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                const deviceId = btn.dataset.id;
                const deviceType = btn.dataset.type;
                handleDetailsClick(deviceId, deviceType);
            });
        });

        // Wire expand/find button handlers
        container.querySelectorAll('.inv-btn-expand').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                const deviceId = btn.dataset.id;
                const deviceType = btn.dataset.type;
                handleItemClick(deviceId, deviceType);
            });
        });
    }

    // Show device details in the sidebar
    function handleDetailsClick(deviceId, deviceType) {
        // For VMs, build a data object from endpoints
        if (deviceType === 'vm') {
            const ep = (topology.endpoints || []).find(
                e => ('vm-' + e.mac.replace(/:/g, '')) === deviceId
            );
            if (ep) {
                Sidebar.showNode({
                    id: deviceId,
                    label: (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac,
                    type: 'vm',
                    chassis_id: ep.mac,
                    mgmt_addr: (ep.ips || []).join(', '),
                    system_name: (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac,
                    annotations: { host_device: ep.host_device || '' },
                });
            }
            return;
        }

        // For regular devices, find in topology and show sidebar
        const device = (topology.devices || []).find(d => d.id === deviceId);
        if (device) {
            const ifaces = device.interfaces || [];
            const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;
            Sidebar.showNode({
                id: device.id,
                label: device.system_name || device.id,
                type: device.type || 'unknown',
                chassis_id: device.chassis_id || '',
                system_name: device.system_name || '',
                system_description: device.system_description || '',
                mgmt_addr: device.management_address || '',
                software_version: device.software_version || '',
                uptime: device.uptime || '',
                interfaces_up: ifacesUp,
                interfaces_total: ifaces.length,
                vlans: device.vlans || [],
                annotations: device.annotations || {},
            });
        }

        // Highlight the item in the inventory
        document.querySelectorAll('.inv-item').forEach(el => el.classList.remove('active'));
        const itemEl = document.querySelector(`.inv-item[data-id="${CSS.escape(deviceId)}"]`);
        if (itemEl) itemEl.classList.add('active');
    }

    function handleItemClick(deviceId, deviceType) {
        const cy = NetworkGraph.getInstance();
        if (!cy) return;

        if (deviceType === 'vm') {
            // For VMs: find and expand parent host first, then locate the VM
            const ep = (topology.endpoints || []).find(
                e => ('vm-' + e.mac.replace(/:/g, '')) === deviceId
            );
            if (ep && ep.host_device) {
                // Ensure the host's parent switch is expanded
                expandAncestors(ep.host_device);
                // Expand the host to show VMs
                NetworkGraph.expandNode(ep.host_device);
            }
        } else {
            // For devices: expand ancestors so the node becomes visible
            expandAncestors(deviceId);
        }

        // Wait for layout to settle, then select and pan to the node
        setTimeout(() => {
            const node = cy.getElementById(deviceId);
            if (node.length > 0) {
                cy.nodes().unselect();
                node.select();
                cy.animate({ center: { eles: node }, zoom: Math.max(cy.zoom(), 1.0) }, { duration: 400 });
                // Show popup
                Popup.showForNode(node.data(), node.renderedPosition());
            }
        }, 700);

        // Highlight the item in the inventory
        document.querySelectorAll('.inv-item').forEach(el => el.classList.remove('active'));
        const itemEl = document.querySelector(`.inv-item[data-id="${CSS.escape(deviceId)}"]`);
        if (itemEl) itemEl.classList.add('active');
    }

    // Walk up the topology tree and expand each ancestor so the target becomes visible
    function expandAncestors(deviceId) {
        if (!topology) return;

        const device = (topology.devices || []).find(d => d.id === deviceId);
        if (!device) return;

        const switchRoles = classifySwitches(topology);
        const deviceType = device.type || 'unknown';

        // Effective rank: spine=0, leaf/switch=1, bmc=2, host=2, unknown=3, vm=4
        function effectiveRank(dev) {
            const t = dev.type || 'unknown';
            if (t === 'switch') return switchRoles[dev.id] === 'spine' ? 0 : 1;
            const ranks = { bmc: 2, host: 2, unknown: 3, vm: 4 };
            return ranks[t] ?? 3;
        }

        const myRank = effectiveRank(device);

        // Find parent nodes (devices with lower effective rank connected via links)
        for (const link of (topology.links || [])) {
            let parentId = null;
            if (link.local_device === deviceId) parentId = link.remote_device;
            else if (link.remote_device === deviceId) parentId = link.local_device;
            if (!parentId) continue;

            const parent = (topology.devices || []).find(d => d.id === parentId);
            if (!parent) continue;

            const parentRank = effectiveRank(parent);
            if (parentRank < myRank) {
                // Recursively ensure the parent is visible
                expandAncestors(parentId);
                // Then expand the parent so this device appears
                NetworkGraph.expandNode(parentId);
                return; // Only need one parent path
            }
        }
    }

    function esc(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    return { init, update };
})();
