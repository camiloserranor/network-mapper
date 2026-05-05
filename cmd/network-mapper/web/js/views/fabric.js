// views/fabric.js — Fabric overview: Cytoscape compound nodes with ports
// Responsibility: build graph elements, position them, wire interactions

'use strict';

NM.views.renderFabric = function() {
    const topology = NM.state.topology;
    const roles = NM.data.classifySwitches(topology);
    const hostCounts = NM.data.countHostsPerSwitch(topology);
    const elements = [];
    const switches = (topology.devices || []).filter(d => d.type === 'switch');

    // Build port connection map for all switches
    const portMaps = {};
    for (const sw of switches) {
        portMaps[sw.id] = NM.data.buildPortMap(topology, sw.id);
    }

    // Create compound switch nodes with port children
    for (const sw of switches) {
        const ifaces = sw.interfaces || [];
        const role = roles[sw.id] || 'leaf';
        const hCount = hostCounts[sw.id] || 0;
        const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;

        const healthPct = Math.round((ifacesUp / Math.max(ifaces.length, 1)) * 100);
        let labelParts = [sw.system_name || sw.id];
        if (hCount > 0) labelParts.push(hCount + ' hosts');
        labelParts.push(healthPct + '% healthy');

        elements.push({
            data: {
                id: sw.id,
                label: labelParts.join('\n'),
                type: 'switch-parent',
                role: role,
                deviceType: 'switch',
                system_name: sw.system_name || '',
                interfaces_up: ifacesUp,
                interfaces_total: ifaces.length,
                hostCount: hCount,
                mgmtAddress: sw.management_address || '',
            },
        });

        // Port child nodes — from interfaces or link data
        const createdPorts = new Set();
        const connectedPorts = ifaces.filter(iface => portMaps[sw.id][iface.name || ''] !== undefined);

        for (const iface of connectedPorts) {
            const portName = iface.name || '';
            const conn = portMaps[sw.id][portName];
            const isUp = iface.oper_status === 'UP';
            createdPorts.add(portName);
            elements.push(buildPortElement(sw.id, portName, conn, isUp));
        }

        // Create from link data when interface info is missing
        for (const [portName, conn] of Object.entries(portMaps[sw.id])) {
            if (createdPorts.has(portName)) continue;
            createdPorts.add(portName);
            elements.push(buildPortElement(sw.id, portName, conn, true));
        }
    }

    // Edges between ports (switch-to-switch only, deduplicated)
    const addedEdges = new Set();
    for (const link of (topology.links || [])) {
        const localDev = (topology.devices || []).find(d => d.id === link.local_device);
        const remoteDev = (topology.devices || []).find(d => d.id === link.remote_device);
        if (!localDev || !remoteDev) continue;
        if (localDev.type !== 'switch' || remoteDev.type !== 'switch') continue;

        const sourcePort = link.local_device + '::port::' + link.local_port;
        const targetPort = link.remote_device + '::port::' + link.remote_port;
        const edgeKey = [sourcePort, targetPort].sort().join('|');
        if (addedEdges.has(edgeKey)) continue;
        addedEdges.add(edgeKey);

        elements.push({
            data: {
                id: 'edge::' + sourcePort + '::' + targetPort,
                source: sourcePort,
                target: targetPort,
                type: 'switch-link',
                localDevice: link.local_device,
                remoteDevice: link.remote_device,
                localPort: link.local_port || '',
                remotePort: link.remote_port || '',
                speed: link.speed || '',
                operStatus: link.oper_status || 'UP',
                mtu: link.mtu || '',
                sourceType: link.source_type || '',
                discoveredAt: link.discovered_at || '',
            },
        });
    }

    // Classify for layout: spine (few non-switch links) vs leaf (many)
    const layout = computeLayout(switches, portMaps, topology, elements);

    // Position port nodes at calculated positions
    const portSize = 12;
    const portGap = 4;
    for (const el of elements) {
        if (el.data && el.data.type === 'port') {
            const center = layout.centers[el.data.parent];
            if (center) el.position = { x: center.x, y: center.y };
        }
    }

    NM.graph.render(elements, 'preset');
    const cy = NM.graph.getInstance();

    // Arrange ports in 2 columns inside each switch
    cy.nodes('[type="switch-parent"]').forEach((parent) => {
        const center = layout.centers[parent.data('id')];
        if (!center) return;

        const children = parent.children().sort((a, b) => {
            return (a.data('portName') || '').localeCompare(b.data('portName') || '', undefined, { numeric: true });
        });
        if (children.length === 0) return;

        const cols = 2;
        const rows = Math.ceil(children.length / cols);
        const totalH = rows * (portSize + portGap) - portGap;
        const totalW = cols * (portSize + portGap) - portGap;

        children.forEach((child, i) => {
            const col = i % cols;
            const row = Math.floor(i / cols);
            child.position({
                x: center.x - totalW / 2 + col * (portSize + portGap) + portSize / 2,
                y: center.y - totalH / 2 + row * (portSize + portGap) + portSize / 2,
            });
        });
    });

    cy.fit(cy.elements(), 30);

    // Apply health-based pie background on switch nodes
    applyHealthStyles(cy);

    wireInteractions(cy);
};

// --- Private helpers ---

function buildPortElement(switchId, portName, conn, isUp) {
    const connType = conn ? (conn.remoteType || 'unknown') : (isUp ? 'none' : 'down');
    return {
        data: {
            id: switchId + '::port::' + portName,
            parent: switchId,
            label: '',
            type: 'port',
            portName: portName,
            connType: connType,
            remoteId: conn ? conn.remoteId : '',
            remoteName: conn ? conn.remoteName : '',
            remoteType: conn ? conn.remoteType : '',
            remotePort: conn ? conn.remotePort : '',
            speed: conn ? conn.speed : '',
            operStatus: isUp ? 'UP' : 'DOWN',
            switchId: switchId,
        },
    };
}

function computeLayout(switches, portMaps, topology, elements) {
    const switchLinkCounts = {};
    const switchToSwitchCounts = {};
    for (const sw of switches) {
        switchLinkCounts[sw.id] = Object.keys(portMaps[sw.id]).length;
        switchToSwitchCounts[sw.id] = 0;
    }
    for (const link of (topology.links || [])) {
        const localDev = (topology.devices || []).find(d => d.id === link.local_device);
        const remoteDev = (topology.devices || []).find(d => d.id === link.remote_device);
        if (!localDev || !remoteDev) continue;
        if (localDev.type === 'switch' && remoteDev.type === 'switch') {
            switchToSwitchCounts[localDev.id] = (switchToSwitchCounts[localDev.id] || 0) + 1;
            switchToSwitchCounts[remoteDev.id] = (switchToSwitchCounts[remoteDev.id] || 0) + 1;
        }
    }

    const spineIds = [];
    const leafIds = [];
    for (const sw of switches) {
        const total = switchLinkCounts[sw.id] || 0;
        const toSwitch = switchToSwitchCounts[sw.id] || 0;
        if (total <= toSwitch + 1) spineIds.push(sw.id);
        else leafIds.push(sw.id);
    }

    // Fallback: split by link count
    if (spineIds.length === 0 || leafIds.length === 0) {
        const sorted = [...switches].sort((a, b) =>
            (switchLinkCounts[a.id] || 0) - (switchLinkCounts[b.id] || 0)
        );
        spineIds.length = 0;
        leafIds.length = 0;
        const splitAt = Math.max(1, Math.floor(sorted.length / 2));
        sorted.forEach((sw, i) => {
            if (i < splitAt) spineIds.push(sw.id);
            else leafIds.push(sw.id);
        });
    }

    // Update element roles
    for (const el of elements) {
        if (el.data && el.data.type === 'switch-parent') {
            el.data.role = spineIds.includes(el.data.id) ? 'spine' : 'leaf';
        }
    }

    // Position switches: spines top row, leaves bottom row
    const hGap = 220;
    const vGap = 250;

    function rowCenters(ids, yPos) {
        const totalWidth = (ids.length - 1) * hGap;
        const startX = -totalWidth / 2;
        const positions = {};
        ids.forEach((id, i) => { positions[id] = { x: startX + i * hGap, y: yPos }; });
        return positions;
    }

    return {
        centers: { ...rowCenters(spineIds, 0), ...rowCenters(leafIds, vGap) },
        spineIds,
        leafIds,
    };
}

function applyHealthStyles(cy) {
    cy.nodes('[type="switch-parent"]').forEach((node) => {
        const up = node.data('interfaces_up') || 0;
        const total = node.data('interfaces_total') || 1;
        const pct = Math.round((up / Math.max(total, 1)) * 100);
        const healthColor = pct > 80 ? '#4CAF50' : pct > 50 ? '#FF9800' : '#e94560';

        // Use pie-chart background as a health indicator ring
        node.style({
            'pie-size': '100%',
            'pie-1-background-color': healthColor,
            'pie-1-background-size': pct,
            'pie-2-background-color': '#323130',
            'pie-2-background-size': 100 - pct,
            'pie-1-background-opacity': 0.25,
            'pie-2-background-opacity': 0.15,
        });
    });
}

function wireInteractions(cy) {
    cy.off('mouseover', 'node[type="port"]');
    cy.off('mouseout', 'node[type="port"]');
    cy.off('tap', 'node[type="port"]');
    cy.off('tap', 'node[type="switch-parent"]');
    cy.off('tap', 'edge[type="switch-link"]');
    cy.off('mouseover', 'node[type="switch-parent"]');
    cy.off('mouseout', 'node[type="switch-parent"]');

    // Switch hover animation
    cy.on('mouseover', 'node[type="switch-parent"]', (evt) => {
        evt.target.addClass('hover');
        document.getElementById('cy').style.cursor = 'pointer';
    });
    cy.on('mouseout', 'node[type="switch-parent"]', (evt) => {
        evt.target.removeClass('hover');
        document.getElementById('cy').style.cursor = 'default';
    });

    cy.on('mouseover', 'node[type="port"]', (evt) => {
        const node = evt.target;
        node.addClass('highlight');
        node.connectedEdges().addClass('highlight');
        showFabricTooltip(node);
    });
    cy.on('mouseout', 'node[type="port"]', (evt) => {
        const node = evt.target;
        node.removeClass('highlight');
        node.connectedEdges().removeClass('highlight');
        hideFabricTooltip();
    });
    cy.on('tap', 'node[type="port"]', (evt) => {
        const node = evt.target;
        const remoteId = node.data('remoteId');
        const remoteType = node.data('remoteType');
        if (!remoteId) return;
        if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
        else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
    });
    cy.on('tap', 'node[type="switch-parent"]', (evt) => {
        NM.state.ViewManager.navigateTo('switch', evt.target.data('id'));
    });
    cy.on('tap', 'edge[type="switch-link"]', (evt) => {
        showLinkDetail(evt.target);
    });
}

function showFabricTooltip(node) {
    let tooltip = document.getElementById('port-hover-tooltip');
    if (!tooltip) {
        tooltip = document.createElement('div');
        tooltip.id = 'port-hover-tooltip';
        tooltip.className = 'port-hover-tooltip';
        document.body.appendChild(tooltip);
    }

    const data = node.data();
    const esc = NM.core.escapeHtml;
    let html = '<div class="port-tooltip-row"><span class="port-tooltip-label">Port:</span><span class="port-tooltip-value">' + esc(data.portName) + '</span></div>';
    html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + data.operStatus + '</span></div>';
    if (data.remoteId) {
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Connected to:</span><span class="port-tooltip-value">' + esc(data.remoteName) + '</span></div>';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Remote port:</span><span class="port-tooltip-value">' + esc(data.remotePort) + '</span></div>';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Type:</span><span class="port-tooltip-value">' + esc(data.remoteType) + '</span></div>';
        if (data.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + esc(data.speed) + '</span></div>';
    }

    tooltip.innerHTML = html;
    tooltip.style.display = 'block';

    const pos = node.renderedPosition();
    const cyContainer = document.getElementById('cy').getBoundingClientRect();
    tooltip.style.left = (cyContainer.left + pos.x + 12) + 'px';
    tooltip.style.top = (cyContainer.top + pos.y - 10) + 'px';
}

function hideFabricTooltip() {
    const tooltip = document.getElementById('port-hover-tooltip');
    if (tooltip) tooltip.style.display = 'none';
}

function showLinkDetail(edge) {
    const data = edge.data();
    const esc = NM.core.escapeHtml;
    const topology = NM.state.topology;

    // Find device names
    const localDev = (topology.devices || []).find(d => d.id === data.localDevice);
    const remoteDev = (topology.devices || []).find(d => d.id === data.remoteDevice);
    const localName = localDev ? (localDev.system_name || localDev.id) : data.localDevice;
    const remoteName = remoteDev ? (remoteDev.system_name || remoteDev.id) : data.remoteDevice;

    let tooltip = document.getElementById('port-hover-tooltip');
    if (!tooltip) {
        tooltip = document.createElement('div');
        tooltip.id = 'port-hover-tooltip';
        tooltip.className = 'port-hover-tooltip';
        document.body.appendChild(tooltip);
    }

    let html = '<div class="port-tooltip-row"><span class="port-tooltip-label">Link</span><span class="port-tooltip-value" style="color:#0078d4">\u2501</span></div>';
    html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Source:</span><span class="port-tooltip-value">' + esc(localName) + ' : ' + esc(data.localPort) + '</span></div>';
    html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Target:</span><span class="port-tooltip-value">' + esc(remoteName) + ' : ' + esc(data.remotePort) + '</span></div>';
    if (data.operStatus) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + esc(data.operStatus) + '</span></div>';
    if (data.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + esc(data.speed) + '</span></div>';
    if (data.mtu) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">MTU:</span><span class="port-tooltip-value">' + esc(data.mtu) + '</span></div>';
    if (data.sourceType) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Discovery:</span><span class="port-tooltip-value">' + esc(data.sourceType) + '</span></div>';
    if (data.discoveredAt) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Discovered:</span><span class="port-tooltip-value">' + esc(data.discoveredAt) + '</span></div>';

    tooltip.innerHTML = html;
    tooltip.style.display = 'block';

    const midpoint = edge.renderedMidpoint();
    const cyContainer = document.getElementById('cy');
    const rect = cyContainer.getBoundingClientRect();
    tooltip.style.left = (rect.left + midpoint.x + 12) + 'px';
    tooltip.style.top = (rect.top + midpoint.y - 10) + 'px';

    // Dismiss on next tap/click anywhere
    setTimeout(() => {
        const dismiss = () => { hideFabricTooltip(); document.removeEventListener('click', dismiss); };
        document.addEventListener('click', dismiss, { once: true });
    }, 100);
}
