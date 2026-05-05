// views/switch.js — Switch detail: port diagram + full device info

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

    // --- Switch diagram (visual port block with name inside) ---
    html += '<div class="switch-diagram-container">';
    html += '<div class="switch-diagram-chassis ' + role + '">';
    html += '<div class="switch-diagram-nameplate">';
    html += '<strong>' + esc(swDev.system_name || swDev.id) + '</strong>';
    html += '<span class="switch-diagram-summary">';
    html += esc(role.toUpperCase()) + ' \u00B7 ' + ifacesUp + '/' + ifaces.length + ' ports UP';
    if (hostCount > 0) html += ' \u00B7 ' + hostCount + ' hosts';
    if (swDev.management_address) html += ' \u00B7 ' + esc(swDev.management_address);
    html += '</span>';
    html += '</div>';

    // Port grid inside the chassis
    html += '<div class="port-grid">';
    for (const iface of ifaces) {
        const portName = iface.name || '';
        const conn = portMap[portName];
        const isUp = iface.oper_status === 'UP';
        let classes = 'port-slot';
        if (conn) classes += ' connected conn-' + conn.remoteType;
        if (!isUp) classes += ' down';

        const shortName = portName.replace(/.*\//, '').replace(/Ethernet/, 'E');

        html += '<div class="' + classes + '"';
        if (conn) {
            html += ' data-remote-id="' + esc(conn.remoteId) + '"';
            html += ' data-remote-type="' + esc(conn.remoteType) + '"';
        }
        html += '>';
        html += '<span>' + esc(shortName) + '</span>';

        // Tooltip
        html += '<div class="port-tooltip">';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Port:</span><span class="port-tooltip-value">' + esc(portName) + '</span></div>';
        if (iface.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + esc(iface.speed) + '</span></div>';
        if (iface.mtu) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">MTU:</span><span class="port-tooltip-value">' + esc(String(iface.mtu)) + '</span></div>';
        if (conn) {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Connected to:</span><span class="port-tooltip-value">' + esc(conn.remoteName) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Remote port:</span><span class="port-tooltip-value">' + esc(conn.remotePort) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Type:</span><span class="port-tooltip-value">' + esc(conn.remoteType) + '</span></div>';
            if (conn.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Link speed:</span><span class="port-tooltip-value">' + esc(conn.speed) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + (isUp ? 'UP' : 'DOWN') + '</span></div>';
        } else {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + (isUp ? 'UP (no LLDP)' : 'DOWN') + '</span></div>';
        }
        html += '</div>';
        html += '</div>';
    }
    html += '</div>'; // port-grid

    // Legend
    html += '<div class="port-legend">';
    html += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:var(--node-switch);background:rgba(0,120,212,0.12)"></div>Switch</div>';
    html += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:var(--node-host);background:rgba(68,183,0,0.12)"></div>Host</div>';
    html += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:var(--node-bmc);background:rgba(247,99,12,0.12)"></div>BMC</div>';
    html += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:var(--node-unknown);background:rgba(138,136,134,0.12)"></div>Unknown</div>';
    html += '<div class="port-legend-item"><div class="port-legend-swatch" style="border-color:var(--border);background:var(--bg-surface)"></div>Empty</div>';
    html += '</div>';
    html += '</div>'; // switch-diagram-chassis
    html += '</div>'; // switch-diagram-container

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

    // Interface summary
    html += '<div class="info-panel">';
    html += '<div class="info-panel-title">Interface Summary</div>';
    html += infoRow('Total Interfaces', ifaces.length);
    html += infoRow('UP', ifacesUp);
    html += infoRow('DOWN', ifaces.length - ifacesUp);
    html += infoRow('Health', Math.round((ifacesUp / Math.max(ifaces.length, 1)) * 100) + '%');
    html += infoRow('Connected Hosts', hostCount);

    // Count switch-to-switch links
    const switchLinks = Object.values(portMap).filter(c => c.remoteType === 'switch').length;
    html += infoRow('Switch Uplinks', switchLinks);
    html += '</div>';

    // VLAN info if available
    const vlans = swDev.vlans || [];
    if (vlans.length > 0) {
        html += '<div class="info-panel">';
        html += '<div class="info-panel-title">VLANs (' + vlans.length + ')</div>';
        html += '<div class="info-panel-content" style="font-size:12px;color:var(--text-secondary)">' + esc(vlans.join(', ')) + '</div>';
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
            const status = link.oper_status || '—';
            const speed = link.speed || '—';

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

    // Wire click events on port slots
    container.querySelectorAll('.port-slot.connected').forEach(slot => {
        slot.addEventListener('click', () => {
            const remoteId = slot.dataset.remoteId;
            const remoteType = slot.dataset.remoteType;
            if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
        });
    });

    // Wire click events on connection rows
    container.querySelectorAll('.conn-row').forEach(row => {
        row.addEventListener('click', () => {
            const remoteId = row.dataset.remoteId;
            const remoteType = row.dataset.remoteType;
            if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
        });
    });
};

function infoRow(label, value) {
    const esc = NM.core.escapeHtml;
    return '<div class="info-row"><span class="info-row-label">' + esc(String(label)) + '</span><span class="info-row-value">' + esc(String(value)) + '</span></div>';
}
