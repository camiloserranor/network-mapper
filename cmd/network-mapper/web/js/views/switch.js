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
    html += '<div class="info-panel collapsible">';
    html += '<div class="info-panel-title">Device Identity <span class="panel-chevron">▾</span></div>';
    html += '<div class="panel-body">';
    html += infoRow('ID', swDev.id);
    if (swDev.system_name) html += infoRow('System Name', swDev.system_name);
    if (swDev.chassis_id) html += infoRow('Chassis ID', swDev.chassis_id);
    html += infoRow('Role', role);
    html += infoRow('Type', swDev.type || 'switch');
    if (swDev.management_address) html += infoRow('Management IP', swDev.management_address);
    if (swDev.system_description) html += infoRow('Description', swDev.system_description);
    if (swDev.software_version) html += infoRow('Software Version', swDev.software_version);
    if (swDev.uptime) html += infoRow('Uptime', swDev.uptime);
    html += '</div></div>';

    // Resource utilization
    if (swDev.cpu_utilization || swDev.memory_total) {
        html += '<div class="info-panel collapsible">';
        html += '<div class="info-panel-title">Resources <span class="panel-chevron">▾</span></div>';
        html += '<div class="panel-body">';
        if (swDev.cpu_utilization) {
            html += infoRow('CPU', swDev.cpu_utilization.toFixed(1) + '%');
        }
        if (swDev.memory_total) {
            const memPct = ((swDev.memory_used || 0) / swDev.memory_total * 100).toFixed(1);
            const memUsedGB = ((swDev.memory_used || 0) / (1024*1024*1024)).toFixed(1);
            const memTotalGB = (swDev.memory_total / (1024*1024*1024)).toFixed(1);
            html += infoRow('Memory', memUsedGB + ' / ' + memTotalGB + ' GB (' + memPct + '%)');
        }
        html += '</div></div>';
    }

    // Interface summary
    html += '<div class="info-panel collapsible">';
    html += '<div class="info-panel-title">Interface Summary <span class="panel-chevron">▾</span></div>';
    html += '<div class="panel-body">';
    html += infoRow('Total Interfaces', ifaces.length);
    html += infoRow('UP', ifacesUp);
    html += infoRow('DOWN', ifaces.length - ifacesUp);
    html += infoRow('Health', Math.round((ifacesUp / Math.max(ifaces.length, 1)) * 100) + '%');
    html += infoRow('Connected Hosts', hostCount);
    const switchLinks = Object.values(portMap).filter(c => c.remoteType === 'switch').length;
    html += infoRow('Switch Uplinks', switchLinks);
    html += '</div></div>';

    // VLAN membership visualization (Venn-like cards)
    html += buildVLANVisualization(swDev, portMap, esc);

    // VLAN info — show topology-level VLAN details and per-interface VLAN assignments
    const vlans = swDev.vlans || [];
    var ifaceVlanData = buildInterfaceVLANSummary(swDev);
    if (vlans.length > 0 || ifaceVlanData.length > 0) {
        html += '<div class="info-panel wide collapsible">';
        html += '<div class="info-panel-title">VLAN Assignments <span class="panel-chevron">▾</span></div>';
        html += '<div class="panel-body">';

        if (ifaceVlanData.length > 0) {
            html += '<table class="conn-table"><thead><tr><th>Port</th><th>Mode</th><th>Access</th><th>Native</th><th>Active VLANs</th></tr></thead><tbody>';
            for (var vi = 0; vi < ifaceVlanData.length; vi++) {
                var vd = ifaceVlanData[vi];
                html += '<tr>';
                html += '<td>' + esc(vd.name) + '</td>';
                html += '<td>' + esc(vd.mode || '\u2014') + '</td>';
                html += '<td>' + (vd.accessVlan || '\u2014') + '</td>';
                html += '<td>' + (vd.nativeVlan || '\u2014') + '</td>';
                html += '<td class="vlan-list">' + esc(vd.observedVlans || '\u2014') + '</td>';
                html += '</tr>';
            }
            html += '</tbody></table>';
        } else if (vlans.length > 0) {
            html += '<div class="info-panel-content" style="font-size:12px;color:var(--text-secondary)">' + esc(vlans.join(', ')) + '</div>';
        }

        html += '</div></div>';
    }

    // BGP sessions
    const bgpSessions = swDev.bgp_sessions || [];
    if (bgpSessions.length > 0) {
        const established = bgpSessions.filter(s => s.session_state === 'ESTABLISHED').length;
        html += '<div class="info-panel wide collapsible">';
        html += '<div class="info-panel-title">BGP Sessions (' + established + '/' + bgpSessions.length + ' Established) <span class="panel-chevron">▾</span></div>';
        html += '<div class="panel-body">';
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
        html += '</div></div>';
    }

    // Annotations
    const annotations = swDev.annotations || {};
    const annotationKeys = Object.keys(annotations);
    if (annotationKeys.length > 0) {
        html += '<div class="info-panel collapsible">';
        html += '<div class="info-panel-title">Annotations <span class="panel-chevron">▾</span></div>';
        html += '<div class="panel-body">';
        for (const key of annotationKeys) {
            html += infoRow(key, String(annotations[key]));
        }
        html += '</div></div>';
    }

    // Connections table
    const links = (topology.links || []).filter(l => l.local_device === switchId || l.remote_device === switchId);
    links.sort(function(a, b) {
        const aPort = (a.local_device === switchId ? a.local_port : a.remote_port) || '';
        const bPort = (b.local_device === switchId ? b.local_port : b.remote_port) || '';
        return aPort.localeCompare(bPort, undefined, { numeric: true });
    });
    if (links.length > 0) {
        html += '<div class="info-panel wide collapsible">';
        html += '<div class="info-panel-title">Connections (' + links.length + ') <span class="panel-chevron">▾</span></div>';
        html += '<div class="panel-body">';
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
        html += '</div></div>';
    }

    html += '</div>'; // switch-info-panels

    container.innerHTML = html;

    // Wire collapsible panel toggles
    container.querySelectorAll('.info-panel.collapsible .info-panel-title').forEach(function(title) {
        title.addEventListener('click', function() {
            title.parentElement.classList.toggle('collapsed');
        });
    });

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

    // Wire VLAN member chip clicks
    container.querySelectorAll('.vlan-member-chip[data-device-id]').forEach(function(chip) {
        chip.addEventListener('click', function() {
            var devId = chip.dataset.deviceId;
            var devType = chip.dataset.deviceType;
            if (devType === 'host') NM.state.ViewManager.navigateTo('host', devId);
            else if (devType === 'switch') NM.state.ViewManager.navigateTo('switch', devId);
        });
    });
};

function buildFrontPanelSVG(swDev, ifaces, portMap, role, ifacesUp, hostCount, esc) {
    if (ifaces.length === 0) {
        return '<div class="switch-svg-container"><p style="color:var(--text-secondary);padding:16px">No interface data available</p></div>';
    }

    // Classify interfaces: physical vs logical
    const physical = [];
    const logical = [];
    for (const iface of ifaces) {
        const name = (iface.name || '').toLowerCase();
        if (name.match(/^(ethernet|eth|mgmt|management|e\d)/i) || name.match(/^\d+\/\d+/)) {
            physical.push(iface);
        } else {
            logical.push(iface);
        }
    }

    // Sort physical ports numerically
    physical.sort((a, b) => {
        const na = (a.name || '').replace(/[^\d/]/g, '');
        const nb = (b.name || '').replace(/[^\d/]/g, '');
        return na.localeCompare(nb, undefined, { numeric: true });
    });

    // Sort logical: loopback first, then vlan, then port-channel, then others
    const logicalOrder = (name) => {
        const n = name.toLowerCase();
        if (n.startsWith('loopback') || n.startsWith('lo')) return 0;
        if (n.startsWith('vlan')) return 1;
        if (n.startsWith('port-channel') || n.startsWith('po')) return 2;
        return 3;
    };
    logical.sort((a, b) => {
        const oa = logicalOrder(a.name || '');
        const ob = logicalOrder(b.name || '');
        if (oa !== ob) return oa - ob;
        return (a.name || '').localeCompare(b.name || '', undefined, { numeric: true });
    });

    const portCount = physical.length || 1;
    const rows = portCount <= 48 ? 2 : Math.ceil(portCount / 24);
    const actualCols = Math.ceil(portCount / rows);

    const portW = 28;
    const portH = 18;
    const portGapX = 4;
    const portGapY = 6;
    const paddingX = 80;
    const paddingY = 50;
    const panelW = paddingX * 2 + actualCols * (portW + portGapX) - portGapX;
    const panelH = paddingY + rows * (portH + portGapY) - portGapY + 20;

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
    let summary = role.toUpperCase();
    if (hostCount > 0) summary += ' \u00B7 ' + hostCount + ' hosts';
    svg += '<text x="' + paddingX + '" y="42" font-size="9" fill="#8a8886" font-family="Segoe UI, sans-serif">';
    svg += esc(summary);
    svg += '</text>';

    // LED indicator (top-right)
    const ledColor = ifacesUp > 0 ? '#4CAF50' : '#e94560';
    svg += '<circle cx="' + (panelW - 24) + '" cy="20" r="5" fill="' + ledColor + '"/>';
    svg += '<circle cx="' + (panelW - 24) + '" cy="20" r="3" fill="' + ledColor + '" opacity="0.5"/>';

    // Physical ports in rows
    const portStartY = paddingY;
    for (let i = 0; i < physical.length; i++) {
        const iface = physical[i];
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

        // Port label (short) — extract just the port number for display
        var shortName = portName.replace(/.*\//, '');  // Strip slot prefix (e.g., Eth1/48 → 48)
        if (shortName === portName) {
            // No slash — strip known prefixes to get numeric part
            shortName = shortName.replace(/^(Ethernet|Eth|eth|Et)/, '');
        }
        if (shortName.length > 4) shortName = shortName.substring(0, 4);
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

    // Logical interfaces section (VLAN, Loopback, Port-Channel)
    if (logical.length > 0) {
        svg += '<div class="logical-interfaces">';
        svg += '<div class="logical-interfaces-title">Logical Interfaces</div>';
        svg += '<div class="logical-interfaces-grid">';
        for (const iface of logical) {
            const name = iface.name || '';
            const isUp = iface.oper_status === 'UP';
            const typeClass = name.toLowerCase().startsWith('vlan') ? 'vlan' :
                              name.toLowerCase().startsWith('loopback') || name.toLowerCase().startsWith('lo') ? 'loopback' :
                              name.toLowerCase().startsWith('port-channel') || name.toLowerCase().startsWith('po') ? 'port-channel' : 'other';
            svg += '<div class="logical-iface ' + typeClass + ' ' + (isUp ? 'up' : 'down') + '">';
            svg += '<span class="logical-iface-status"></span>';
            svg += '<span class="logical-iface-name">' + esc(name) + '</span>';
            svg += '</div>';
        }
        svg += '</div></div>';
    }

    svg += '</div>';

    return svg;
}

// buildVLANVisualization creates colored VLAN membership cards showing hosts/ports per VLAN.
function buildVLANVisualization(swDev, portMap, esc) {
    var vlanMembers = {}; // vlanId → [{port, host, hostId, hostType}]
    var ifaces = swDev.interfaces || [];

    for (var i = 0; i < ifaces.length; i++) {
        var iface = ifaces[i];
        var conn = portMap[iface.name];
        if (!conn) continue;

        var vlans = [];
        if (iface.access_vlan) vlans.push(iface.access_vlan);
        if (iface.trunk_vlans) vlans = vlans.concat(iface.trunk_vlans);
        if (iface.observed_vlans) {
            for (var v = 0; v < iface.observed_vlans.length; v++) {
                if (vlans.indexOf(iface.observed_vlans[v]) === -1) vlans.push(iface.observed_vlans[v]);
            }
        }

        for (var j = 0; j < vlans.length; j++) {
            var vid = vlans[j];
            if (!vlanMembers[vid]) vlanMembers[vid] = [];
            vlanMembers[vid].push({
                port: iface.name,
                host: conn.remoteName || conn.remoteId,
                hostId: conn.remoteId,
                hostType: conn.remoteType
            });
        }
    }

    var vlanIds = Object.keys(vlanMembers).sort(function(a, b) { return parseInt(a) - parseInt(b); });
    if (vlanIds.length === 0) return '';

    var colors = ['#0078d4', '#44b700', '#f7630c', '#a36efd', '#d13438', '#00b7c3', '#8764b8', '#ca5010', '#57a300', '#4f6bed'];

    var html = '<div class="info-panel wide collapsible">';
    html += '<div class="info-panel-title">VLAN Membership <span class="vlan-viz-count">' + vlanIds.length + ' VLANs</span><span class="panel-chevron">▾</span></div>';
    html += '<div class="panel-body"><div class="vlan-sets-container">';

    for (var k = 0; k < vlanIds.length; k++) {
        var vid = vlanIds[k];
        var members = vlanMembers[vid];
        var color = colors[k % colors.length];

        html += '<div class="vlan-set" style="border-color:' + color + '">';
        html += '<div class="vlan-set-header" style="background:' + color + '20;color:' + color + '">';
        html += '<span class="vlan-set-id">VLAN ' + esc(String(vid)) + '</span>';
        html += '<span class="vlan-set-count">' + members.length + ' port' + (members.length !== 1 ? 's' : '') + '</span>';
        html += '</div>';
        html += '<div class="vlan-set-members">';

        for (var m = 0; m < members.length; m++) {
            var member = members[m];
            var chipClass = 'vlan-member-chip ' + (member.hostType || 'unknown');
            html += '<div class="' + chipClass + '" data-device-id="' + esc(member.hostId) + '" data-device-type="' + esc(member.hostType) + '">';
            html += '<span class="vlan-member-name">' + esc(member.host) + '</span>';
            html += '<span class="vlan-member-port">' + esc(member.port) + '</span>';
            html += '</div>';
        }

        html += '</div></div>';
    }

    html += '</div></div></div>';
    return html;
}

function getPortVLANInfo(portName) {
    var topology = NM.state.topology;
    if (!topology || !topology.devices) return null;
    for (var i = 0; i < topology.devices.length; i++) {
        var dev = topology.devices[i];
        if (dev.type !== 'switch' || !dev.interfaces) continue;
        for (var j = 0; j < dev.interfaces.length; j++) {
            var iface = dev.interfaces[j];
            if (iface.name !== portName) continue;
            var info = {};
            if (iface.mode) info.mode = iface.mode;
            if (iface.access_vlan) info.accessVlan = iface.access_vlan;
            if (iface.native_vlan) info.nativeVlan = iface.native_vlan;
            if (iface.observed_vlans && iface.observed_vlans.length > 0) {
                info.observedVlans = iface.observed_vlans.join(', ');
            }
            if (iface.trunk_vlans && iface.trunk_vlans.length > 0) {
                info.trunkVlans = iface.trunk_vlans.length + ' VLANs';
            }
            return Object.keys(info).length > 0 ? info : null;
        }
    }
    return null;
}

// buildInterfaceVLANSummary returns interfaces that have any VLAN config or observed VLAN data.
function buildInterfaceVLANSummary(swDev) {
    var results = [];
    var ifaces = swDev.interfaces || [];
    for (var i = 0; i < ifaces.length; i++) {
        var iface = ifaces[i];
        var hasVlanData = iface.vlan_mode || iface.mode || iface.access_vlan || iface.native_vlan ||
            (iface.trunk_vlans && iface.trunk_vlans.length > 0) ||
            (iface.observed_vlans && iface.observed_vlans.length > 0);
        if (!hasVlanData) continue;

        results.push({
            name: iface.name || '',
            mode: iface.vlan_mode || iface.mode || '',
            accessVlan: iface.access_vlan || 0,
            nativeVlan: iface.native_vlan || 0,
            observedVlans: (iface.observed_vlans || []).join(', ')
        });
    }
    // Sort: physical ports first, then by name
    results.sort(function(a, b) {
        return (a.name || '').localeCompare(b.name || '', undefined, { numeric: true });
    });
    return results;
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

    // VLAN info from interface data
    var portVlanInfo = getPortVLANInfo(portName);
    if (portVlanInfo) {
        if (portVlanInfo.mode) {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Mode:</span><span class="port-tooltip-value">' + esc(portVlanInfo.mode) + '</span></div>';
        }
        if (portVlanInfo.accessVlan) {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Access VLAN:</span><span class="port-tooltip-value">' + portVlanInfo.accessVlan + '</span></div>';
        }
        if (portVlanInfo.nativeVlan) {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Native VLAN:</span><span class="port-tooltip-value">' + portVlanInfo.nativeVlan + '</span></div>';
        }
        if (portVlanInfo.observedVlans) {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Active VLANs:</span><span class="port-tooltip-value">' + esc(portVlanInfo.observedVlans) + '</span></div>';
        }
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
