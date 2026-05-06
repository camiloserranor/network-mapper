// views/host.js — Host detail: NIC diagram + VM cloud

'use strict';

NM.views.renderHost = function(hostId) {
    const topology = NM.state.topology;
    const hostDev = (topology.devices || []).find(d => d.id === hostId);
    if (!hostDev) return;

    const container = document.getElementById('detail-view');
    const portMap = NM.data.buildPortMap(topology, hostId);
    const esc = NM.core.escapeHtml;

    // NICs are derived from LLDP links (host has no gNMI agent)
    const nics = Object.entries(portMap).map(([portName, conn]) => ({
        name: portName,
        conn: conn,
        isUp: (conn.operStatus || '').toUpperCase() === 'UP' || !!conn.remoteId,
    }));
    const vms = NM.data.getHostVMs(topology, hostId);

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon host">\u2395</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + esc(hostDev.system_name || hostDev.id) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Connections: <strong>' + nics.length + '</strong></span>';
    html += '<span>VMs: <strong>' + vms.length + '</strong></span>';
    if (hostDev.management_address) html += '<span>Mgmt: <strong>' + esc(hostDev.management_address) + '</strong></span>';
    if (hostDev.system_description) html += '<span>' + esc(hostDev.system_description) + '</span>';
    html += '</div></div></div>';

    // NIC diagram — built from LLDP-discovered links
    html += '<div class="nic-diagram">';
    html += '<div class="nic-diagram-title">Network Connections <span style="color:var(--text-muted);font-size:11px">(discovered via LLDP)</span></div>';
    html += '<div class="nic-list">';

    for (const nic of nics) {
        const conn = nic.conn;
        const typeClass = conn ? conn.remoteType : '';

        html += '<div class="nic-card" data-remote-id="' + (conn ? esc(conn.remoteId) : '') + '" data-remote-type="' + (conn ? esc(conn.remoteType) : '') + '">';
        html += '<div class="nic-card-status ' + (nic.isUp ? 'up' : 'down') + '"></div>';
        html += '<div class="nic-card-port">' + esc(nic.name) + '</div>';

        if (conn) {
            html += '<div class="nic-card-remote">\u2192 ' + esc(conn.remoteName) + ' (' + esc(conn.remotePort) + ')</div>';
            html += '<div class="nic-card-type ' + esc(typeClass) + '">' + esc(conn.remoteType) + '</div>';
            if (conn.speed) html += '<div class="nic-card-speed">' + esc(conn.speed) + '</div>';
        }
        html += '</div>';
    }

    if (nics.length === 0) {
        html += '<div style="color:var(--text-muted);padding:12px">No LLDP connections discovered for this host</div>';
    }
    html += '</div></div>';

    // VM Cloud
    if (vms.length > 0) {
        html += '<div class="vm-cloud">';
        html += '<div class="vm-cloud-title">Virtual Machines</div>';

        const uniqueVlans = new Set();
        for (const vm of vms) { (vm.vlans || []).forEach(v => uniqueVlans.add(v)); }

        html += '<div class="vm-cloud-stats">';
        html += '<div class="vm-stat"><div class="vm-stat-value">' + vms.length + '</div><div class="vm-stat-label">Total VMs</div></div>';
        html += '<div class="vm-stat"><div class="vm-stat-value">' + uniqueVlans.size + '</div><div class="vm-stat-label">VLANs</div></div>';
        html += '</div>';

        html += '<div class="vm-list-compact">';
        const MAX_DISPLAY = 200;
        const displayVMs = vms.slice(0, MAX_DISPLAY);
        for (const vm of displayVMs) {
            const vmId = 'vm-' + vm.mac.replace(/:/g, '');
            const label = (vm.ips && vm.ips.length > 0) ? vm.ips[0] : vm.mac;
            html += '<div class="vm-chip" data-vm-id="' + esc(vmId) + '">';
            html += '<span class="vm-chip-dot"></span>';
            html += esc(label);
            html += '</div>';
        }
        if (vms.length > MAX_DISPLAY) {
            html += '<div class="vm-chip" style="color:var(--text-muted);cursor:default">+' + (vms.length - MAX_DISPLAY) + ' more</div>';
        }
        html += '</div>';
        html += '</div>';
    }

    container.innerHTML = html;

    // Wire NIC card clicks
    container.querySelectorAll('.nic-card').forEach(card => {
        const remoteId = card.dataset.remoteId;
        const remoteType = card.dataset.remoteType;
        if (remoteId) {
            card.addEventListener('click', () => {
                if (remoteType === 'switch') NM.state.ViewManager.navigateTo('switch', remoteId);
                else if (remoteType === 'host') NM.state.ViewManager.navigateTo('host', remoteId);
            });
        }
    });

    // Wire VM chip clicks
    container.querySelectorAll('.vm-chip[data-vm-id]').forEach(chip => {
        chip.addEventListener('click', () => {
            NM.state.ViewManager.navigateTo('vm', chip.dataset.vmId);
        });
    });
};
