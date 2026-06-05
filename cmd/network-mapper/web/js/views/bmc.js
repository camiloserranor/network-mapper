// views/bmc.js — BMC (Baseboard Management Controller) detail view

'use strict';

NM.views.renderBMC = function(bmcId) {
    var topology = NM.state.topology;
    var bmcDev = (topology.devices || []).find(function(d) { return d.id === bmcId; });
    if (!bmcDev) return;

    var container = document.getElementById('detail-view');
    var esc = NM.core.escapeHtml;

    // Find links for this BMC
    var links = (topology.links || []).filter(function(l) {
        return l.local_device === bmcId || l.remote_device === bmcId;
    });

    // Build connection info from links
    var connections = links.map(function(l) {
        var switchId = l.local_device === bmcId ? l.remote_device : l.local_device;
        var switchPort = l.local_device === bmcId ? l.remote_port : l.local_port;
        var swDev = (topology.devices || []).find(function(d) { return d.id === switchId; });
        var switchName = swDev ? (swDev.system_name || swDev.id) : switchId;
        return {
            switchId: switchId,
            switchName: switchName,
            switchPort: switchPort,
            operStatus: l.oper_status || '',
            speed: l.speed || '',
            mtu: l.mtu || '',
            isUp: (l.oper_status || '').toUpperCase() === 'UP'
        };
    });

    var html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon bmc">\u2699</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + esc(bmcDev.system_name || bmcDev.id) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Type: <strong>BMC</strong></span>';
    html += '<span>Connections: <strong>' + connections.length + '</strong></span>';
    if (bmcDev.management_address) html += '<span>Mgmt: <strong>' + esc(bmcDev.management_address) + '</strong></span>';
    html += '</div></div></div>';

    // Device identity panel
    if (bmcDev.chassis_id || bmcDev.system_description || bmcDev.management_address) {
        html += '<div class="info-panel">';
        html += '<div class="info-panel-title">Device Identity</div>';
        if (bmcDev.chassis_id) html += infoRow('Chassis ID', bmcDev.chassis_id);
        if (bmcDev.management_address) html += infoRow('Management IP', bmcDev.management_address);
        if (bmcDev.system_description) html += infoRow('Description', bmcDev.system_description);
        if (bmcDev.software_version) html += infoRow('Software', bmcDev.software_version);
        html += '</div>';
    }

    // Network connections
    html += '<div class="nic-diagram">';
    html += '<div class="nic-diagram-title">Network Connections <span style="color:var(--text-muted);font-size:11px">(discovered via LLDP)</span></div>';
    html += '<div class="nic-list">';

    for (var i = 0; i < connections.length; i++) {
        var conn = connections[i];
        var switchId = conn.switchId;
        var switchPort = conn.switchPort;

        html += '<div class="nic-card" data-remote-id="' + esc(switchId) + '" data-remote-type="switch">';

        html += '<div class="nic-card-header">';
        html += '<div class="nic-card-status ' + (conn.isUp ? 'up' : 'down') + '"></div>';
        html += '<div class="nic-card-port">' + esc(switchPort || '?') + '</div>';
        html += '<div class="nic-card-remote">\u2192 ' + esc(conn.switchName) + '</div>';
        html += '</div>';

        html += '<div class="nic-card-badges">';
        if (conn.speed) html += '<div class="nic-card-speed">' + esc(conn.speed) + '</div>';
        var mtu = 0;
        if (switchId && switchPort) mtu = NM.data.getInterfaceMTU(switchId, switchPort);
        if (!mtu && conn.mtu) mtu = parseInt(conn.mtu) || 0;
        if (mtu > 0) {
            var mtuLabel = mtu >= 9000 ? 'Jumbo (' + mtu + ')' : 'MTU ' + mtu;
            var mtuClass = mtu >= 9000 ? 'nic-card-mtu jumbo' : 'nic-card-mtu';
            html += '<div class="' + mtuClass + '" title="Maximum Transmission Unit">' + mtuLabel + '</div>';
        }
        html += '</div>';

        // Counters
        var c = null;
        if (switchId && switchPort) {
            c = NM.data.getInterfaceCounters(switchId, switchPort);
        }
        if (c) {
            html += '<div class="nic-card-counters">';
            html += '<span title="Received bytes">\u2193 Rx ' + formatBytes(c.in_octets) + '</span>';
            html += '<span title="Transmitted bytes">\u2191 Tx ' + formatBytes(c.out_octets) + '</span>';
            if (c.in_errors > 0 || c.out_errors > 0) {
                html += '<span class="counter-errors" title="Interface errors">\u26a0 ' + (c.in_errors + c.out_errors) + ' errors</span>';
            }
            html += '</div>';
        }

        html += '</div>';
    }

    if (connections.length === 0) {
        html += '<div style="color:var(--text-muted);padding:12px">No connections discovered for this BMC</div>';
    }
    html += '</div></div>';

    container.innerHTML = html;

    // Wire NIC card clicks
    container.querySelectorAll('.nic-card').forEach(function(card) {
        var remoteId = card.dataset.remoteId;
        if (remoteId) {
            card.addEventListener('click', function() {
                NM.state.ViewManager.navigateTo('switch', remoteId);
            });
        }
    });
};
