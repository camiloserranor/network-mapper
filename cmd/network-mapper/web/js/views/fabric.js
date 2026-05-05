// views/fabric.js — Fabric overview: SVG front-panel switches with port hotspots
// Responsibility: build graph elements, position them, wire interactions

'use strict';

// SVG geometry constants — shared between SVG generation and port positioning
const FABRIC_PORT = {
    W: 14,       // port width (px in SVG viewBox)
    H: 10,       // port height
    GAP_X: 2,    // horizontal gap between ports
    GAP_Y: 3,    // vertical gap between rows
    PAD_X: 40,   // left padding for port area
    PAD_Y: 26,   // top padding for port area (below nameplate)
    ROWS: 2,     // number of port rows
};

// Compute SVG chassis dimensions for a given port count
function computeChassisDims(portCount) {
    const cols = Math.ceil(portCount / FABRIC_PORT.ROWS);
    const portsW = cols * (FABRIC_PORT.W + FABRIC_PORT.GAP_X) - FABRIC_PORT.GAP_X;
    const portsH = FABRIC_PORT.ROWS * (FABRIC_PORT.H + FABRIC_PORT.GAP_Y) - FABRIC_PORT.GAP_Y;
    const panelW = FABRIC_PORT.PAD_X * 2 + portsW;
    const panelH = FABRIC_PORT.PAD_Y + portsH + 22; // 22px for health bar + bottom padding
    return { panelW, panelH, cols, portsW, portsH };
}

// Get the position of a port relative to chassis center (0,0)
function getPortCenter(index, portCount) {
    const dims = computeChassisDims(portCount);
    const col = index % dims.cols;
    const row = Math.floor(index / dims.cols);
    // Port top-left in SVG coordinates
    const px = FABRIC_PORT.PAD_X + col * (FABRIC_PORT.W + FABRIC_PORT.GAP_X);
    const py = FABRIC_PORT.PAD_Y + row * (FABRIC_PORT.H + FABRIC_PORT.GAP_Y);
    // Center of port in SVG coords
    const cx = px + FABRIC_PORT.W / 2;
    const cy = py + FABRIC_PORT.H / 2;
    // Convert to offset from chassis center
    return {
        x: cx - dims.panelW / 2,
        y: cy - dims.panelH / 2,
    };
}

// Generate simplified SVG for fabric view (no interactive elements, just visuals)
function buildFabricChassisImage(sw, ifaces, portMap, role, ifacesUp, hostCount) {
    const portCount = Math.max(ifaces.length, 1);
    const dims = computeChassisDims(portCount);
    const colors = { switch: '#0078d4', host: '#44b700', bmc: '#f7630c', unknown: '#8a8886' };

    const chassisColor = role === 'spine' ? '#1e2a3a' : '#252423';
    const borderColor = role === 'spine' ? '#2899f5' : '#0078d4';
    const pct = Math.round((ifacesUp / Math.max(ifaces.length, 1)) * 100);
    const healthColor = pct > 80 ? '#4CAF50' : pct > 50 ? '#FF9800' : '#e94560';

    let svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + dims.panelW + ' ' + dims.panelH + '">';

    // Chassis body
    svg += '<rect x="1" y="1" width="' + (dims.panelW - 2) + '" height="' + (dims.panelH - 2) + '" rx="6" ry="6" ';
    svg += 'fill="' + chassisColor + '" stroke="' + borderColor + '" stroke-width="1.5"/>';

    // Ventilation grille (left decorative)
    for (let i = 0; i < 2; i++) {
        const gx = 10 + i * 10;
        svg += '<rect x="' + gx + '" y="' + (dims.panelH / 2 - 8) + '" width="2" height="16" rx="1" fill="#3a3a3a" opacity="0.5"/>';
    }

    // LED indicator (top-right)
    const ledColor = ifacesUp > 0 ? '#4CAF50' : '#e94560';
    svg += '<circle cx="' + (dims.panelW - 14) + '" cy="12" r="3" fill="' + ledColor + '"/>';

    // Nameplate text
    const name = (sw.system_name || sw.id).substring(0, 20);
    svg += '<text x="' + FABRIC_PORT.PAD_X + '" y="14" font-size="8" font-weight="600" fill="#ffffff" font-family="Segoe UI,sans-serif">';
    svg += escSvg(name) + '</text>';

    // Summary line
    let summary = role.toUpperCase() + ' · ' + ifacesUp + '/' + ifaces.length + ' UP';
    if (hostCount > 0) summary += ' · ' + hostCount + ' hosts';
    svg += '<text x="' + FABRIC_PORT.PAD_X + '" y="22" font-size="5.5" fill="#8a8886" font-family="Segoe UI,sans-serif">';
    svg += escSvg(summary) + '</text>';

    // Ports
    for (let i = 0; i < ifaces.length; i++) {
        const iface = ifaces[i];
        const portName = iface.name || '';
        const conn = portMap[portName];
        const isUp = iface.oper_status === 'UP';

        const col = i % dims.cols;
        const row = Math.floor(i / dims.cols);
        const px = FABRIC_PORT.PAD_X + col * (FABRIC_PORT.W + FABRIC_PORT.GAP_X);
        const py = FABRIC_PORT.PAD_Y + row * (FABRIC_PORT.H + FABRIC_PORT.GAP_Y);

        let fillColor = '#484644';
        if (!isUp) fillColor = '#2a2a2a';
        else if (conn) fillColor = colors[conn.remoteType] || colors.unknown;

        const borderCol = conn ? (colors[conn.remoteType] || '#605e5c') : (isUp ? '#605e5c' : '#3a3a3a');
        const opacity = isUp ? '1' : '0.4';

        svg += '<rect x="' + px + '" y="' + py + '" width="' + FABRIC_PORT.W + '" height="' + FABRIC_PORT.H + '" rx="2" ';
        svg += 'fill="' + fillColor + '" stroke="' + borderCol + '" stroke-width="0.8" opacity="' + opacity + '"/>';
    }

    // Health bar (bottom)
    const hbX = FABRIC_PORT.PAD_X;
    const hbY = dims.panelH - 10;
    const hbW = dims.panelW - FABRIC_PORT.PAD_X * 2;
    svg += '<rect x="' + hbX + '" y="' + hbY + '" width="' + hbW + '" height="3" rx="1.5" fill="#3a3a3a"/>';
    svg += '<rect x="' + hbX + '" y="' + hbY + '" width="' + Math.round(hbW * pct / 100) + '" height="3" rx="1.5" fill="' + healthColor + '"/>';

    svg += '</svg>';
    return svg;
}

function escSvg(str) {
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

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

    // SVG images per switch (for background-image)
    const switchSvgs = {};

    // Create compound switch nodes with port children
    for (const sw of switches) {
        const ifaces = sw.interfaces || [];
        const role = roles[sw.id] || 'leaf';
        const hCount = hostCounts[sw.id] || 0;
        const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;

        // Derive health
        let healthPct;
        if (ifaces.length > 0) {
            healthPct = Math.round((ifacesUp / ifaces.length) * 100);
        } else {
            const linkCount = (topology.links || []).filter(
                l => l.local_device === sw.id || l.remote_device === sw.id
            ).length;
            healthPct = linkCount > 0 ? 100 : 0;
        }

        // Generate SVG for this switch
        const svgStr = buildFabricChassisImage(sw, ifaces, portMaps[sw.id], role, ifacesUp, hCount);
        const svgDataUri = 'data:image/svg+xml;utf8,' + encodeURIComponent(svgStr);
        switchSvgs[sw.id] = svgDataUri;

        // Compute chassis size for Cytoscape node dimensions
        const dims = computeChassisDims(Math.max(ifaces.length, 1));
        // Scale factor: map SVG units to Cytoscape model units
        const scale = 1.8;
        const nodeW = dims.panelW * scale;
        const nodeH = dims.panelH * scale;

        elements.push({
            data: {
                id: sw.id,
                label: '',
                type: 'switch-parent',
                role: role,
                deviceType: 'switch',
                system_name: sw.system_name || sw.id,
                interfaces_up: ifacesUp,
                interfaces_total: ifaces.length,
                hostCount: hCount,
                healthPct: healthPct,
                mgmtAddress: sw.management_address || '',
                nodeW: nodeW,
                nodeH: nodeH,
                portCount: ifaces.length,
            },
        });

        // Port child nodes — invisible hitboxes positioned to match SVG ports
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

    // Position port nodes at initial center positions
    for (const el of elements) {
        if (el.data && el.data.type === 'port') {
            const center = layout.centers[el.data.parent];
            if (center) el.position = { x: center.x, y: center.y };
        }
    }

    NM.graph.render(elements, 'preset');
    const cy = NM.graph.getInstance();

    // Apply SVG background images and size switch nodes
    cy.nodes('[type="switch-parent"]').forEach((parent) => {
        const id = parent.data('id');
        const svgUri = switchSvgs[id];
        if (!svgUri) return;

        parent.style({
            'background-image': svgUri,
            'background-fit': 'cover',
            'background-clip': 'node',
            'background-opacity': 1,
            'width': parent.data('nodeW'),
            'height': parent.data('nodeH'),
            'padding': '0px',
            'border-width': 0,
            'background-color': 'transparent',
            'shape': 'round-rectangle',
        });
    });

    // Position port child nodes to align with SVG port rectangles
    const scale = 1.8;
    cy.nodes('[type="switch-parent"]').forEach((parent) => {
        const center = layout.centers[parent.data('id')];
        if (!center) return;

        const children = parent.children().sort((a, b) => {
            return (a.data('portName') || '').localeCompare(b.data('portName') || '', undefined, { numeric: true });
        });
        if (children.length === 0) return;

        const portCount = parent.data('portCount') || children.length;

        children.forEach((child, i) => {
            const offset = getPortCenter(i, portCount);
            child.position({
                x: center.x + offset.x * scale,
                y: center.y + offset.y * scale,
            });
        });
    });

    cy.fit(cy.elements(), 30);

    // Add legend panel
    addLegendPanel();

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

    // Compute max node width for spacing
    let maxNodeW = 300;
    for (const el of elements) {
        if (el.data && el.data.nodeW && el.data.nodeW > maxNodeW) {
            maxNodeW = el.data.nodeW;
        }
    }

    // Position switches: spines top row, leaves bottom row
    const hGap = maxNodeW + 60;
    const vGap = 200;

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

// Health styling is now part of the SVG background image — no separate style application needed

function addLegendPanel() {
    // Remove previous
    const existing = document.getElementById('fabric-legend');
    if (existing) existing.remove();

    const container = document.getElementById('cy');
    if (!container) return;

    const legend = document.createElement('div');
    legend.id = 'fabric-legend';
    legend.className = 'fabric-legend';
    legend.innerHTML =
        '<div class="legend-title">Legend</div>' +
        '<div class="legend-section">' +
            '<div class="legend-subtitle">Ports</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#0078d4"></span>Switch uplink</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#44b700"></span>Host connection</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#f7630c"></span>BMC</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#8a8886"></span>Unknown</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#323130;border:1px solid #484644"></span>Down</div>' +
        '</div>' +
        '<div class="legend-section">' +
            '<div class="legend-subtitle">Edges</div>' +
            '<div class="legend-item"><span class="legend-line" style="background:#4CAF50"></span>Link UP</div>' +
            '<div class="legend-item"><span class="legend-line dashed" style="background:#e94560"></span>Link DOWN</div>' +
        '</div>' +
        '<div class="legend-section">' +
            '<div class="legend-subtitle">Switches</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#1e2a3a;border:2px solid #2899f5"></span>Spine</div>' +
            '<div class="legend-item"><span class="legend-swatch" style="background:#252423;border:2px solid #0078d4"></span>Leaf</div>' +
        '</div>';

    container.appendChild(legend);
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
