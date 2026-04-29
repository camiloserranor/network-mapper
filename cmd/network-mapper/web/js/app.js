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

        const elements = topologyToCytoscape(topology);
        const cy = NetworkGraph.init('cy', elements);

        Sidebar.init(topology);
        Popup.init(topology);

        // Click on node → show popup card near node
        cy.on('tap', 'node', (evt) => {
            const node = evt.target;
            Popup.showForNode(node.data(), node.renderedPosition());
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

function topologyToCytoscape(topology) {
    const elements = [];

    // Detect spine switches: switches that are only connected to other switches (not hosts/VMs/BMCs)
    const switchIds = new Set((topology.devices || []).filter(d => d.type === 'switch').map(d => d.id));
    const switchConnectsNonSwitch = new Set();
    for (const link of (topology.links || [])) {
        const localIsSwitch = switchIds.has(link.local_device);
        const remoteIsSwitch = switchIds.has(link.remote_device);
        if (localIsSwitch && !remoteIsSwitch) switchConnectsNonSwitch.add(link.local_device);
        if (remoteIsSwitch && !localIsSwitch) switchConnectsNonSwitch.add(link.remote_device);
    }

    // Devices → nodes
    for (const device of (topology.devices || [])) {
        // Count interface health
        const ifaces = device.interfaces || [];
        const ifacesUp = ifaces.filter((i) => i.oper_status === 'UP').length;

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
            },
        });
    }

    // Endpoints → VM nodes
    for (const ep of (topology.endpoints || [])) {
        const epId = 'vm-' + ep.mac.replace(/:/g, '');
        const label = (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac;

        elements.push({
            data: {
                id: epId,
                label: label,
                type: 'vm',
                mac: ep.mac,
                ips: ep.ips || [],
                vlans: ep.vlans || [],
                host_device: ep.host_device || '',
                host_port: ep.host_port || '',
                switch_id: ep.switch_id || '',
            },
        });

        // Link VM to its parent host (if known)
        if (ep.host_device) {
            elements.push({
                data: {
                    id: `${ep.host_device}::vm::${epId}`,
                    source: ep.host_device,
                    target: epId,
                    source_type: 'mac-table',
                    edgeLabel: ep.host_port || '',
                    oper_status: 'UP',
                },
            });
        }
    }

    // Links → edges
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
