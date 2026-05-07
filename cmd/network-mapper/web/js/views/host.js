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

    // Gather VLAN info for this host
    const hostVlanIds = hostDev.vlans || [];
    const vlanDetails = (topology.vlans || []).filter(v => hostVlanIds.includes(v.id));
    // Also gather VLANs from endpoints if device-level vlans is empty
    const endpointVlans = new Set();
    for (const vm of vms) {
        (vm.vlans || []).forEach(v => endpointVlans.add(v));
    }
    const allVlanIds = new Set([...hostVlanIds, ...endpointVlans]);
    const allVlanDetails = (topology.vlans || []).filter(v => allVlanIds.has(v.id));

    // Find link counters for this host's connections
    const linkData = (topology.links || []).filter(
        l => l.local_device === hostId || l.remote_device === hostId
    );

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon host">\u2395</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + esc(hostDev.system_name || hostDev.id) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Connections: <strong>' + nics.length + '</strong></span>';
    html += '<span>VMs: <strong>' + vms.length + '</strong></span>';
    html += '<span>VLANs: <strong>' + allVlanIds.size + '</strong></span>';
    if (hostDev.management_address) html += '<span>Mgmt: <strong>' + esc(hostDev.management_address) + '</strong></span>';
    if (hostDev.system_description) html += '<span>' + esc(hostDev.system_description) + '</span>';
    html += '</div></div></div>';

    // Device identity panel (if we have useful info)
    if (hostDev.chassis_id || hostDev.software_version || hostDev.management_address) {
        html += '<div class="info-panel">';
        html += '<div class="info-panel-title">Device Identity</div>';
        if (hostDev.chassis_id) html += infoRow('Chassis ID', hostDev.chassis_id);
        if (hostDev.management_address) html += infoRow('Management IP', hostDev.management_address);
        if (hostDev.system_description) html += infoRow('Description', hostDev.system_description);
        if (hostDev.software_version) html += infoRow('Software', hostDev.software_version);
        html += '</div>';
    }

    // NIC diagram — built from LLDP-discovered links
    html += '<div class="nic-diagram">';
    html += '<div class="nic-diagram-title">Network Connections <span style="color:var(--text-muted);font-size:11px">(discovered via LLDP)</span></div>';
    html += '<div class="nic-list">';

    for (const nic of nics) {
        const conn = nic.conn;
        const typeClass = conn ? conn.remoteType : '';
        // Find matching link for counters
        const link = linkData.find(l =>
            (l.local_device === hostId && l.remote_port === nic.name) ||
            (l.remote_device === hostId && l.remote_port === nic.name)
        );

        html += '<div class="nic-card" data-remote-id="' + (conn ? esc(conn.remoteId) : '') + '" data-remote-type="' + (conn ? esc(conn.remoteType) : '') + '">';
        html += '<div class="nic-card-status ' + (nic.isUp ? 'up' : 'down') + '"></div>';
        html += '<div class="nic-card-port">' + esc(nic.name) + '</div>';

        if (conn) {
            html += '<div class="nic-card-remote">\u2192 ' + esc(conn.remoteName) + ' (' + esc(conn.remotePort) + ')</div>';
            html += '<div class="nic-card-type ' + esc(typeClass) + '">' + esc(conn.remoteType) + '</div>';
            if (conn.speed) html += '<div class="nic-card-speed">' + esc(conn.speed) + '</div>';
        }

        // Show counters if available
        if (link && link.counters) {
            const c = link.counters;
            html += '<div class="nic-card-counters">';
            html += '<span>\u2193 ' + formatBytes(c.in_octets) + '</span>';
            html += '<span>\u2191 ' + formatBytes(c.out_octets) + '</span>';
            if (c.in_errors > 0 || c.out_errors > 0) {
                html += '<span class="counter-errors">\u26a0 ' + (c.in_errors + c.out_errors) + ' errors</span>';
            }
            html += '</div>';
        }

        html += '</div>';
    }

    if (nics.length === 0) {
        html += '<div style="color:var(--text-muted);padding:12px">No LLDP connections discovered for this host</div>';
    }
    html += '</div></div>';

    // VLAN Membership
    if (allVlanIds.size > 0) {
        html += '<div class="host-vlans">';
        html += '<div class="host-vlans-title">VLAN Membership</div>';
        html += '<div class="host-vlans-grid">';

        // Sort VLANs by ID
        const sortedVlans = [...allVlanIds].sort((a, b) => a - b);
        for (const vid of sortedVlans) {
            const detail = allVlanDetails.find(v => v.id === vid);
            const vlanName = detail ? detail.name : '';
            const gateway = detail ? (detail.gateway || '') : '';
            const vmCount = vms.filter(vm => (vm.vlans || []).includes(vid)).length;

            html += '<div class="vlan-card">';
            html += '<div class="vlan-card-id">VLAN ' + vid + '</div>';
            if (vlanName) html += '<div class="vlan-card-name">' + esc(vlanName) + '</div>';
            html += '<div class="vlan-card-meta">';
            if (gateway) html += '<span class="vlan-card-gw">' + esc(gateway) + '</span>';
            if (vmCount > 0) html += '<span class="vlan-card-vms">' + vmCount + ' VM' + (vmCount > 1 ? 's' : '') + '</span>';
            html += '</div>';
            html += '</div>';
        }
        html += '</div></div>';
    }

    // VM Cloud
    if (vms.length > 0) {
        html += '<div class="vm-cloud">';
        html += '<div class="vm-cloud-title">Virtual Machines</div>';

        html += '<div class="vm-cloud-stats">';
        html += '<div class="vm-stat"><div class="vm-stat-value">' + vms.length + '</div><div class="vm-stat-label">Total VMs</div></div>';
        html += '<div class="vm-stat"><div class="vm-stat-value">' + allVlanIds.size + '</div><div class="vm-stat-label">VLANs</div></div>';
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

// Helper: format bytes to human readable
function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
}

// Helper: info row (reused from switch view pattern)
function infoRow(label, value) {
    const esc = NM.core.escapeHtml;
    return '<div class="info-row"><span class="info-label">' + esc(label) + '</span><span class="info-value">' + esc(value) + '</span></div>';
}
