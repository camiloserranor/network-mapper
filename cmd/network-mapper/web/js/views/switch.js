// views/switch.js — Switch detail: SVG front-panel + full device info

'use strict';

NM.views.renderSwitch = function(switchId) {
    const topology = NM.state.topology;
    const swDev = (topology.devices || []).find(d => d.id === switchId);
    if (!swDev) return;

    const container = document.getElementById('detail-view');
    const roles = NM.data.classifySwitches(topology);
    const role = roles[switchId] || 'leaf';
    const ifaces = swDev.interfaces || [];
    const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;
    const portMap = NM.data.buildPortMap(topology, switchId);
    const hostCount = NM.data.countHostsPerSwitch(topology)[switchId] || 0;
    const esc = NM.core.escapeHtml;

    let html = '';

    // --- SVG Front Panel ---
    html += buildFrontPanelSVG(swDev, ifaces, portMap, role, ifacesUp, hostCount, esc);

    // --- Full device information ---
    html += '<div class="switch-info-panels">';

    // Device identity
    html += '<div class="info-panel">';
    html += '<div class="info-panel-title">Device Identity</div>';
    html += infoRow('ID', swDev.id);
    if (swDev.system_name) html += infoRow('System Name', swDev.system_name);
    if (swDev.chassis_id) html += infoRow('Chassis ID', swDev.chassis_id);
    html += infoRow('Role', role);
    html += infoRow('Type', swDev.type || 'switch');
    if (swDev.management_address) html += infoRow('Management IP', swDev.management_address);
    if (swDev.system_description) html += infoRow('Description', swDev.system_description);
    if (swDev.software_version) html += infoRow('Software Version', swDev.software_version);
    if (swDev.uptime) html += infoRow('Uptime', swDev.uptime);
    html += '</div>';

    // Resource utilization
    if (swDev.cpu_utilization || swDev.memory_total) {
        html += '<div class="info-panel">';
        html += '<div class="info-panel-title">Resources</div>';
        if (swDev.cpu_utilization) {
            html += infoRow('CPU', swDev.cpu_utilization.toFixed(1) + '%');
        }
        if (swDev.memory_total) {
            const memPct = ((swDev.memory_used || 0) / swDev.memory_total * 100).toFixed(1);
            const memUsedGB = ((swDev.memory_used || 0) / (1024*1024*1024)).toFixed(1);
            const memTotalGB = (swDev.memory_total / (1024*1024*1024)).toFixed(1);
            html += infoRow('Memory', memUsedGB + ' / ' + memTotalGB + ' GB (' + memPct + '%)');
        }
        html += '</div>';
    }

    // Interface summary
    html += '<div class="info-panel">';
    html += '<div class="info-panel-title">Interface Summary</div>';
    html += infoRow('Total Interfaces', ifaces.length);
    html += infoRow('UP', ifacesUp);
    html += infoRow('DOWN', ifaces.length - ifacesUp);
    html += infoRow('Health', Math.round((ifacesUp / Math.max(ifaces.length, 1)) * 100) + '%');
    html += infoRow('Connected Hosts', hostCount);
    const switchLinks = Object.values(portMap).filter(c => c.remoteType === 'switch').length;
    html += infoRow('Switch Uplinks', switchLinks);
    html += '</div>';

    // VLAN info
    const vlans = swDev.vlans || [];
    if (vlans.length > 0) {
        html += '<div class="info-panel">';
        html += '<div class="info-panel-title">VLANs (' + vlans.length + ')</div>';
        html += '<div class="info-panel-content" style="font-size:12px;color:var(--text-secondary)">' + esc(vlans.join(', ')) + '</div>';
        html += '</div>';
    }

    // BGP sessions
    const bgpSessions = swDev.bgp_sessions || [];
    if (bgpSessions.length > 0) {
        const established = bgpSessions.filter(s => s.session_state === 'ESTABLISHED').length;
        html += '<div class="info-panel wide">';
        html += '<div class="info-panel-title">BGP Sessions (' + established + '/' + bgpSessions.length + ' Established)</div>';
        html += '<table class="conn-table"><thead><tr><th>Neighbor</th><th>AS</th><th>State</th><th>VRF</th><th>Pfx Rcvd</th><th>Pfx Sent</th><th>Description</th></tr></thead><tbody>';
        for (const sess of bgpSessions) {
            const stateClass = sess.session_state === 'ESTABLISHED' ? 'bgp-up' : 'bgp-down';
            html += '<tr>';
            html += '<td>' + esc(sess.neighbor_address) + '</td>';
            html += '<td>' + (sess.peer_as || '\u2014') + '</td>';
            html += '<td><span class="bgp-state ' + stateClass + '">' + esc(sess.session_state || 'UNKNOWN') + '</span></td>';
            html += '<td>' + esc(sess.vrf_name || 'default') + '</td>';
            html += '<td>' + (sess.prefixes_received || 0) + '</td>';
            html += '<td>' + (sess.prefixes_sent || 0) + '</td>';
            html += '<td>' + esc(sess.description || '\u2014') + '</td>';
            html += '</tr>';
        }
        html += '</tbody></table>';
        html += '</div>';
    }

    // Annotations
    const annotations = swDev.annotations || {};
    const annotationKeys = Object.keys(annotations);
    if (annotationKeys.length > 0) {
        html += '<div class="info-panel">';
        html += '<div class="info-panel-title">Annotations</div>';
        for (const key of annotationKeys) {
            html += infoRow(key, String(annotations[key]));
        }
        html += '</div>';
    }

    // Connections table
    const links = (topology.links || []).filter(l => l.local_device === switchId || l.remote_device === switchId);
    if (links.length > 0) {
        html += '<div class="info-panel wide">';
        html += '<div class="info-panel-title">Connections (' + links.length + ')</div>';
        html += '<table class="conn-table"><thead><tr><th>Local Port</th><th>Remote Device</th><th>Remote Port</th><th>Type</th><th>Status</th><th>Speed</th></tr></thead><tbody>';
        for (const link of links) {
            const isLocal = link.local_device === switchId;
            const localPort = isLocal ? link.local_port : link.remote_port;
            const remoteDevId = isLocal ? link.remote_device : link.local_device;
            const remotePort = isLocal ? link.remote_port : link.local_port;
            const remoteDev = (topology.devices || []).find(d => d.id === remoteDevId);
            const remoteName = remoteDev ? (remoteDev.system_name || remoteDev.id) : remoteDevId;
            const remoteType = remoteDev ? (remoteDev.type || 'unknown') : 'unknown';
            const status = link.oper_status || '\u2014';
            const speed = link.speed || '\u2014';

            html += '<tr class="conn-row" data-remote-id="' + esc(remoteDevId) + '" data-remote-type="' + esc(remoteType) + '">';
            html += '<td>' + esc(localPort) + '</td>';
            html += '<td>' + esc(remoteName) + '</td>';
            html += '<td>' + esc(remotePort) + '</td>';
            html += '<td><span class="type-badge ' + esc(remoteType) + '">' + esc(remoteType) + '</span></td>';
            html += '<td>' + esc(status) + '</td>';
            html += '<td>' + esc(speed) + '</td>';
            html += '</tr>';
        }
        html += '</tbody></table>';
        html += '</div>';
    }

    html += '</div>'; // switch-info-panels

    container.innerHTML = html;

    // Wire SVG port clicks
    container.querySelectorAll('.svg-port[data-remote-id]').forEach(port => {
        port.addEventListener('click', () => {
            const remoteId = port.dataset.remoteId;
            const remoteType = port.dataset.remoteType;
            if (!remoteId) return;
            if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
        });
    });

    // Wire connection table rows
    container.querySelectorAll('.conn-row').forEach(row => {
        row.addEventListener('click', () => {
            const remoteId = row.dataset.remoteId;
            const remoteType = row.dataset.remoteType;
            if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
        });
    });

    // Wire SVG port hover tooltips
    container.querySelectorAll('.svg-port').forEach(port => {
        port.addEventListener('mouseenter', (e) => showSvgTooltip(e, port));
        port.addEventListener('mouseleave', hideSvgTooltip);
    });
};

function buildFrontPanelSVG(swDev, ifaces, portMap, role, ifacesUp, hostCount, esc) {
    const portCount = ifaces.length;
    if (portCount === 0) {
        return '<div class="switch-svg-container"><p style="color:var(--text-secondary);padding:16px">No interface data available</p></div>';
    }

    const rows = portCount <= 48 ? 2 : Math.ceil(portCount / 24);
    const actualCols = Math.ceil(portCount / rows);

    const portW = 28;
    const portH = 18;
    const portGapX = 4;
    const portGapY = 6;
    const paddingX = 80;
    const paddingY = 50;
    const panelW = paddingX * 2 + actualCols * (portW + portGapX) - portGapX;
    const panelH = paddingY + rows * (portH + portGapY) - portGapY + 40;

    const colors = { switch: '#0078d4', host: '#44b700', bmc: '#f7630c', unknown: '#8a8886' };

    let svg = '<div class="switch-svg-container">';
    svg += '<svg class="switch-front-panel" viewBox="0 0 ' + panelW + ' ' + panelH + '" preserveAspectRatio="xMidYMid meet">';

    // Chassis body
    const chassisColor = role === 'spine' ? '#1e2a3a' : '#252423';
    const borderColor = role === 'spine' ? '#2899f5' : '#0078d4';
    svg += '<rect x="2" y="2" width="' + (panelW - 4) + '" height="' + (panelH - 4) + '" rx="10" ry="10" ';
    svg += 'fill="' + chassisColor + '" stroke="' + borderColor + '" stroke-width="2.5"/>';

    // Ventilation grille (decorative)
    for (let i = 0; i < 3; i++) {
        const gx = 16 + i * 14;
        svg += '<rect x="' + gx + '" y="' + (panelH / 2 - 12) + '" width="3" height="24" rx="1" fill="#3a3a3a" opacity="0.5"/>';
    }

    // Nameplate (top-left inside chassis)
    svg += '<text x="' + paddingX + '" y="28" font-size="13" font-weight="600" fill="#ffffff" font-family="Segoe UI, sans-serif">';
    svg += esc(swDev.system_name || swDev.id);
    svg += '</text>';

    // Summary line
    let summary = role.toUpperCase() + ' \u00B7 ' + ifacesUp + '/' + ifaces.length + ' UP';
    if (hostCount > 0) summary += ' \u00B7 ' + hostCount + ' hosts';
    svg += '<text x="' + paddingX + '" y="42" font-size="9" fill="#8a8886" font-family="Segoe UI, sans-serif">';
    svg += esc(summary);
    svg += '</text>';

    // Health bar
    const healthBarX = paddingX;
    const healthBarY = panelH - 14;
    const healthBarW = panelW - paddingX * 2;
    const pct = Math.round((ifacesUp / Math.max(ifaces.length, 1)) * 100);
    const healthColor = pct > 80 ? '#4CAF50' : pct > 50 ? '#FF9800' : '#e94560';
    svg += '<rect x="' + healthBarX + '" y="' + healthBarY + '" width="' + healthBarW + '" height="4" rx="2" fill="#3a3a3a"/>';
    svg += '<rect x="' + healthBarX + '" y="' + healthBarY + '" width="' + Math.round(healthBarW * pct / 100) + '" height="4" rx="2" fill="' + healthColor + '"/>';

    // LED indicator (top-right)
    const ledColor = ifacesUp > 0 ? '#4CAF50' : '#e94560';
    svg += '<circle cx="' + (panelW - 24) + '" cy="20" r="5" fill="' + ledColor + '"/>';
    svg += '<circle cx="' + (panelW - 24) + '" cy="20" r="3" fill="' + ledColor + '" opacity="0.5"/>';

    // Ports in rows
    const portStartY = paddingY;
    for (let i = 0; i < portCount; i++) {
        const iface = ifaces[i];
        const portName = iface.name || '';
        const conn = portMap[portName];
        const isUp = iface.oper_status === 'UP';

        const col = i % actualCols;
        const row = Math.floor(i / actualCols);
        const px = paddingX + col * (portW + portGapX);
        const py = portStartY + row * (portH + portGapY);

        let fillColor = '#484644';
        if (!isUp) fillColor = '#2a2a2a';
        else if (conn) fillColor = colors[conn.remoteType] || colors.unknown;

        const borderCol = conn ? (colors[conn.remoteType] || '#605e5c') : (isUp ? '#605e5c' : '#3a3a3a');
        const opacity = isUp ? '1' : '0.4';

        svg += '<g class="svg-port" data-port="' + esc(portName) + '"';
        if (conn) {
            svg += ' data-remote-id="' + esc(conn.remoteId) + '"';
            svg += ' data-remote-type="' + esc(conn.remoteType) + '"';
            svg += ' data-remote-name="' + esc(conn.remoteName) + '"';
            svg += ' data-remote-port="' + esc(conn.remotePort) + '"';
            svg += ' data-speed="' + esc(conn.speed || '') + '"';
        }
        svg += ' data-status="' + (isUp ? 'UP' : 'DOWN') + '"';
        svg += ' style="cursor:' + (conn ? 'pointer' : 'default') + ';opacity:' + opacity + '">';

        svg += '<rect x="' + px + '" y="' + py + '" width="' + portW + '" height="' + portH + '" rx="3" ';
        svg += 'fill="' + fillColor + '" stroke="' + borderCol + '" stroke-width="1.2"/>';

        // Port label (short)
        const shortName = portName.replace(/.*\//, '').replace(/Ethernet/, 'E').substring(0, 4);
        svg += '<text x="' + (px + portW / 2) + '" y="' + (py + portH / 2 + 3.5) + '" ';
        svg += 'font-size="7" fill="#d2d0ce" text-anchor="middle" font-family="Cascadia Code, Consolas, monospace">';
        svg += esc(shortName);
        svg += '</text>';

        svg += '</g>';
    }

    svg += '</svg>';

    // Legend below SVG
    svg += '<div class="port-legend">';
    svg += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:#0078d4;background:rgba(0,120,212,0.2)"></div>Switch</div>';
    svg += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:#44b700;background:rgba(68,183,0,0.2)"></div>Host</div>';
    svg += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:#f7630c;background:rgba(247,99,12,0.2)"></div>BMC</div>';
    svg += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:#8a8886;background:rgba(138,136,134,0.2)"></div>Unknown</div>';
    svg += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:#3a3a3a;background:#2a2a2a"></div>Down</div>';
    svg += '</div>';
    svg += '</div>';

    return svg;
}

function showSvgTooltip(e, port) {
    let tooltip = document.getElementById('svg-port-tooltip');
    if (!tooltip) {
        tooltip = document.createElement('div');
        tooltip.id = 'svg-port-tooltip';
        tooltip.className = 'port-hover-tooltip';
        document.body.appendChild(tooltip);
    }

    const esc = NM.core.escapeHtml;
    const portName = port.dataset.port || '';
    const status = port.dataset.status || '';
    const remoteId = port.dataset.remoteId || '';
    const remoteName = port.dataset.remoteName || '';
    const remotePort = port.dataset.remotePort || '';
    const remoteType = port.dataset.remoteType || '';
    const speed = port.dataset.speed || '';

    let html = '<div class="port-tooltip-row"><span class="port-tooltip-label">Port:</span><span class="port-tooltip-value">' + esc(portName) + '</span></div>';
    html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + esc(status) + '</span></div>';
    if (remoteId) {
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Connected to:</span><span class="port-tooltip-value">' + esc(remoteName) + '</span></div>';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Remote port:</span><span class="port-tooltip-value">' + esc(remotePort) + '</span></div>';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Type:</span><span class="port-tooltip-value">' + esc(remoteType) + '</span></div>';
        if (speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + esc(speed) + '</span></div>';
    }

    tooltip.innerHTML = html;
    tooltip.style.display = 'block';

    const rect = port.getBoundingClientRect();
    tooltip.style.left = (rect.right + 8) + 'px';
    tooltip.style.top = (rect.top - 4) + 'px';
}

function hideSvgTooltip() {
    const tooltip = document.getElementById('svg-port-tooltip');
    if (tooltip) tooltip.style.display = 'none';
}

function infoRow(label, value) {
    const esc = NM.core.escapeHtml;
    return '<div class="info-row"><span class="info-row-label">' + esc(String(label)) + '</span><span class="info-row-value">' + esc(String(value)) + '</span></div>';
}
