// views/switch.js — Switch detail: HTML port diagram view

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
    const esc = NM.core.escapeHtml;

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon switch">\u2B21</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + esc(swDev.system_name || swDev.id) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Role: <strong>' + esc(role) + '</strong></span>';
    html += '<span>Ports: <strong>' + ifacesUp + '/' + ifaces.length + ' UP</strong></span>';
    if (swDev.management_address) html += '<span>Mgmt: <strong>' + esc(swDev.management_address) + '</strong></span>';
    if (swDev.software_version) html += '<span>Version: <strong>' + esc(swDev.software_version) + '</strong></span>';
    html += '</div></div></div>';

    // Port diagram
    html += '<div class="port-diagram">';
    html += '<div class="port-diagram-title">Physical Ports</div>';
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
        if (conn) {
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Connected to:</span><span class="port-tooltip-value">' + esc(conn.remoteName) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Remote port:</span><span class="port-tooltip-value">' + esc(conn.remotePort) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Type:</span><span class="port-tooltip-value">' + esc(conn.remoteType) + '</span></div>';
            if (conn.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + esc(conn.speed) + '</span></div>';
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
    html += '</div>'; // port-diagram

    container.innerHTML = html;

    // Wire click events
    container.querySelectorAll('.port-slot.connected').forEach(slot => {
        slot.addEventListener('click', () => {
            const remoteId = slot.dataset.remoteId;
            const remoteType = slot.dataset.remoteType;
            if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
        });
    });
};
