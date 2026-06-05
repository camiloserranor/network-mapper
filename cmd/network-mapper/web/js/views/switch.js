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
        html += '<div class="info-panel-title">Port VLAN Configuration ';
        html += '<span class="help-btn" title="Per-port VLAN settings from the switch running config. Mode=trunk allows multiple VLANs; mode=access allows a single VLAN. The \'Traffic Observed\' column shows VLANs where we actually detected MAC address activity — empty means no traffic was seen (the port may still be correctly configured).">?</span>';
        html += '<span class="panel-chevron">▾</span></div>';
        html += '<div class="panel-body">';

        if (ifaceVlanData.length > 0) {
            html += '<table class="conn-table"><thead><tr><th>Port</th><th>Mode</th><th title="The VLAN used when the port is in access mode">Access</th><th title="The default VLAN for untagged frames on a trunk port">Native</th><th title="VLANs where MAC address table entries were observed on this port — empty does NOT mean misconfiguration, just no detected traffic">Traffic Observed</th></tr></thead><tbody>';
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

    // QoS / RDMA Health panel
    html += buildQoSPanel(switchId, topology, esc);

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
            else if (remoteType === 'bmc') NM.state.ViewManager.navigateTo('bmc', remoteId);
        });
    });

    // Wire connection table rows
    container.querySelectorAll('.conn-row').forEach(row => {
        row.addEventListener('click', () => {
            const remoteId = row.dataset.remoteId;
            const remoteType = row.dataset.remoteType;
            if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
            else if (remoteType === 'bmc') NM.state.ViewManager.navigateTo('bmc', remoteId);
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
            else if (devType === 'bmc') NM.state.ViewManager.navigateTo('bmc', devId);
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

        svg += '<g class="svg-port" data-port="' + esc(portName) + '" data-device-id="' + esc(swDev.id) + '"';
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
    html += '<div class="info-panel-title">VLANs Allowed per Port ';
    html += '<span class="help-btn" title="This shows which VLANs each port is configured to carry, based on trunk/access VLAN settings. A port appears under a VLAN if it is allowed to forward traffic for that VLAN — this does NOT mean traffic is actively flowing.">?</span>';
    html += '<span class="vlan-viz-count">' + vlanIds.length + ' VLANs</span><span class="panel-chevron">▾</span></div>';
    html += '<div class="panel-body"><div class="vlan-sets-container">';

    for (var k = 0; k < vlanIds.length; k++) {
        var vid = vlanIds[k];
        var members = vlanMembers[vid];
        // Sort members: by host name alphabetically
        members.sort(function(a, b) {
            var na = (a.host || '').toLowerCase();
            var nb = (b.host || '').toLowerCase();
            if (na < nb) return -1;
            if (na > nb) return 1;
            return 0;
        });
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

    // Link health from telemetry counters
    var deviceId = port.dataset.deviceId || '';
    if (deviceId && portName) {
        var c = NM.data.getInterfaceCounters(deviceId, portName);
        if (c) {
            html += '<div class="port-tooltip-row" style="border-top:1px solid #333;margin-top:4px;padding-top:4px"><span class="port-tooltip-label">Rx:</span><span class="port-tooltip-value">' + fmtBytes(c.in_octets) + ' (' + fmtNum(c.in_pkts) + ' pkts)</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Tx:</span><span class="port-tooltip-value">' + fmtBytes(c.out_octets) + ' (' + fmtNum(c.out_pkts) + ' pkts)</span></div>';
            if (c.in_errors > 0 || c.out_errors > 0) {
                html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Errors:</span><span class="port-tooltip-value" style="color:#f85149">' + (c.in_errors + c.out_errors) + '</span></div>';
            }
            if (c.in_discards > 0 || c.out_discards > 0) {
                html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Drops:</span><span class="port-tooltip-value" style="color:#f0883e">' + (c.in_discards + c.out_discards) + '</span></div>';
            }
            if (c.crc_errors > 0) {
                html += '<div class="port-tooltip-row"><span class="port-tooltip-label">CRC Errors:</span><span class="port-tooltip-value" style="color:#f85149">' + c.crc_errors + '</span></div>';
            }
            if (c.pause_frames_in > 0 || c.pause_frames_out > 0) {
                html += '<div class="port-tooltip-row"><span class="port-tooltip-label">PFC Pause:</span><span class="port-tooltip-value" style="color:#f0883e">' + (c.pause_frames_in || 0) + ' in / ' + (c.pause_frames_out || 0) + ' out</span></div>';
            }
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

function fmtBytes(bytes) {
    if (!bytes) return '0 B';
    var sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.floor(Math.log(bytes) / Math.log(1024));
    return parseFloat((bytes / Math.pow(1024, i)).toFixed(1)) + ' ' + sizes[i];
}

function fmtNum(n) {
    if (!n) return '0';
    if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B';
    if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
    if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
    return String(n);
}

function buildQoSPanel(switchId, topology, esc) {
    var v2 = topology._v2;
    if (!v2 || !v2.fabric || !v2.fabric.switches) return '';

    var sw = null;
    var switches = v2.fabric.switches;
    for (var i = 0; i < switches.length; i++) {
        if (switches[i].id === switchId) { sw = switches[i]; break; }
    }
    if (!sw) return '';

    var qosStats = sw.qos_stats || [];
    var pfcConfig = sw.pfc_config || [];

    if (qosStats.length === 0 && pfcConfig.length === 0) return '';

    var html = '';
    html += '<div class="info-panel wide collapsible">';
    html += '<div class="info-panel-title">QoS / RDMA Health';

    // Summary badge
    var hasIssues = false;
    for (var q = 0; q < qosStats.length; q++) {
        var s = qosStats[q];
        if (s.pfc_pause_frames_tx || s.pfc_pause_frames_rx ||
            s.pfc_watchdog_drops || s.ecn_marked_packets || s.drop_packets) {
            hasIssues = true;
            break;
        }
    }
    if (hasIssues) {
        html += ' <span class="qos-badge qos-badge-warning">\u26a0 Issues</span>';
    } else if (qosStats.length > 0) {
        html += ' <span class="qos-badge qos-badge-ok">\u2713 Healthy</span>';
    }
    html += ' <span class="panel-chevron">\u25be</span></div>';
    html += '<div class="panel-body">';

    // PFC Configuration summary
    if (pfcConfig.length > 0) {
        html += '<h4 class="qos-section-title">PFC Configuration</h4>';
        html += '<table class="conn-table"><thead><tr>';
        html += '<th>Interface</th><th>Mode</th><th>Lossless CoS</th><th>DCBX TLV</th>';
        html += '</tr></thead><tbody>';
        for (var p = 0; p < pfcConfig.length; p++) {
            var pfc = pfcConfig[p];
            var modeClass = pfc.mode === 'on' ? 'pfc-on' : (pfc.mode === 'auto' ? 'pfc-auto' : 'pfc-off');
            html += '<tr>';
            html += '<td>' + esc(pfc.interface_name) + '</td>';
            html += '<td><span class="pfc-mode ' + modeClass + '">' + esc(pfc.mode || 'unknown') + '</span></td>';
            html += '<td>' + (pfc.lossless_cos && pfc.lossless_cos.length ? pfc.lossless_cos.join(', ') : '\u2014') + '</td>';
            html += '<td>' + (pfc.send_tlv ? '\u2713' : '\u2717') + '</td>';
            html += '</tr>';
        }
        html += '</tbody></table>';
    }

    // QoS per-queue stats
    if (qosStats.length > 0) {
        html += '<h4 class="qos-section-title">Per-Queue Counters</h4>';

        // Group by interface
        var byIface = {};
        for (var qi = 0; qi < qosStats.length; qi++) {
            var stat = qosStats[qi];
            var ifName = stat.interface_name || 'unknown';
            if (!byIface[ifName]) byIface[ifName] = [];
            byIface[ifName].push(stat);
        }

        var ifaceNames = Object.keys(byIface).sort(function(a, b) {
            return a.localeCompare(b, undefined, { numeric: true });
        });

        html += '<table class="conn-table qos-table"><thead><tr>';
        html += '<th>Interface</th><th>Queue</th><th>Dir</th>';
        html += '<th title="Total bytes transmitted on this queue">TX Bytes</th>';
        html += '<th title="Total packets transmitted on this queue">TX Pkts</th>';
        html += '<th title="PFC Pause Frames Received — the switch received requests from a neighbor to stop sending on this priority. High values indicate downstream congestion.">PFC Rx</th>';
        html += '<th title="PFC Pause Frames Transmitted — the switch asked its neighbor to stop sending on this priority. High values indicate local buffer pressure.">PFC Tx</th>';
        html += '<th title="PFC Watchdog Drops — packets dropped because a priority queue was stuck in paused state too long (PFC storm). Critical issue for RDMA traffic.">PFC WD</th>';
        html += '<th title="ECN Marked Packets — packets marked with Explicit Congestion Notification, warning the sender to reduce its rate before drops occur. Early congestion signal.">ECN Marked</th>';
        html += '<th title="Queue Drops — packets dropped because the queue overflowed. Indicates sustained congestion that exceeded the available buffer.">Drops</th>';
        html += '<th title="Current Queue Depth — how full the queue buffer is right now. High values relative to buffer size indicate risk of drops.">Q Depth</th>';
        html += '</tr></thead><tbody>';

        for (var fi = 0; fi < ifaceNames.length; fi++) {
            var ifStats = byIface[ifaceNames[fi]];
            for (var si = 0; si < ifStats.length; si++) {
                var qs = ifStats[si];
                var rowClass = '';
                if (qs.pfc_watchdog_drops || qs.drop_packets) rowClass = 'qos-row-critical';
                else if (qs.pfc_pause_frames_tx || qs.pfc_pause_frames_rx || qs.ecn_marked_packets) rowClass = 'qos-row-warning';

                html += '<tr class="' + rowClass + '">';
                html += '<td>' + esc(qs.interface_name) + '</td>';
                html += '<td>' + esc(qs.queue_name) + '</td>';
                html += '<td>' + esc(qs.direction || '') + '</td>';
                html += '<td>' + fmtBytes(qs.tx_bytes || 0) + '</td>';
                html += '<td>' + fmtNum(qs.tx_packets || 0) + '</td>';
                html += '<td>' + fmtNum(qs.pfc_pause_frames_rx || 0) + '</td>';
                html += '<td>' + fmtNum(qs.pfc_pause_frames_tx || 0) + '</td>';
                html += '<td>' + fmtNum(qs.pfc_watchdog_drops || 0) + '</td>';
                html += '<td>' + fmtNum(qs.ecn_marked_packets || 0) + '</td>';
                html += '<td>' + fmtNum(qs.drop_packets || 0) + '</td>';
                html += '<td>' + fmtBytes(qs.current_queue_depth_bytes || 0) + '</td>';
                html += '</tr>';
            }
        }
        html += '</tbody></table>';
    }

    html += '</div></div>';
    return html;
}
