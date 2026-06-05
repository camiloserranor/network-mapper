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

        // Show counters from telemetry or link data
        var c = null;
        var switchId = '';
        var switchPort = '';
        if (link) {
            // Counters belong to the switch port connected to this host
            switchId = link.local_device === hostId ? link.remote_device : link.local_device;
            switchPort = link.local_device === hostId ? link.remote_port : link.local_port;
            c = NM.data.getInterfaceCounters(switchId, switchPort);
            if (!c && link.counters) c = link.counters;
        }

        // MTU / Jumbo frame indicator
        var mtu = 0;
        if (switchId && switchPort) mtu = NM.data.getInterfaceMTU(switchId, switchPort);
        if (!mtu && link && link.mtu) mtu = parseInt(link.mtu) || 0;
        if (mtu > 0) {
            var mtuLabel = mtu >= 9000 ? 'Jumbo (' + mtu + ')' : mtu.toString();
            var mtuClass = mtu >= 9000 ? 'nic-card-mtu jumbo' : 'nic-card-mtu';
            html += '<div class="' + mtuClass + '" title="Maximum Transmission Unit — 9000+ indicates jumbo frames for RDMA/storage traffic">MTU ' + mtuLabel + '</div>';
        } else {
            html += '<div class="nic-card-mtu unavailable" title="MTU data not available from this switch">MTU —</div>';
        }

        if (c) {
            html += '<div class="nic-card-counters">';
            html += '<span title="Received bytes">\u2193 Rx ' + formatBytes(c.in_octets) + '</span>';
            html += '<span title="Transmitted bytes">\u2191 Tx ' + formatBytes(c.out_octets) + '</span>';
            if (c.in_pkts || c.out_pkts) {
                html += '<span title="Packets (in/out)">\u2194 ' + formatNumber(c.in_pkts || 0) + ' / ' + formatNumber(c.out_pkts || 0) + ' pkts</span>';
            }
            if (c.in_errors > 0 || c.out_errors > 0) {
                html += '<span class="counter-errors" title="Interface errors">\u26a0 ' + (c.in_errors + c.out_errors) + ' errors</span>';
            }
            if (c.in_discards > 0 || c.out_discards > 0) {
                html += '<span class="counter-errors" title="Dropped packets">\u2717 ' + (c.in_discards + c.out_discards) + ' drops</span>';
            }
            if (c.pause_frames_in > 0 || c.pause_frames_out > 0) {
                html += '<span class="counter-pfc" title="PFC pause frames (congestion indicator)">\u23f8 PFC ' + ((c.pause_frames_in || 0) + (c.pause_frames_out || 0)) + '</span>';
            }
            if (c.crc_errors > 0) {
                html += '<span class="counter-errors" title="CRC errors (physical layer issue)">\u26a0 CRC ' + c.crc_errors + '</span>';
            }
            html += '</div>';
        }

        // Per-queue QoS stats (ECN, PFC watchdog, drops) for this port
        if (switchId && switchPort) {
            var qosStats = NM.data.getQoSStatsForPort(switchId, switchPort);
            var hasQoSIssues = false;
            for (var qi = 0; qi < qosStats.length; qi++) {
                var qs = qosStats[qi];
                if (qs.pfc_pause_frames_rx || qs.pfc_pause_frames_tx || qs.pfc_watchdog_drops || qs.ecn_marked_packets || qs.drop_packets) {
                    hasQoSIssues = true;
                    break;
                }
            }
            if (qosStats.length > 0) {
                html += '<div class="nic-card-qos">';
                html += '<div class="nic-card-qos-title" title="Per-queue QoS metrics from the switch port — critical for RDMA lossless traffic">QoS Queues';
                if (hasQoSIssues) html += ' <span class="qos-badge qos-badge-warning">\u26a0</span>';
                else html += ' <span class="qos-badge qos-badge-ok">\u2713</span>';
                html += '</div>';
                for (var qi2 = 0; qi2 < qosStats.length; qi2++) {
                    var q = qosStats[qi2];
                    var qClass = '';
                    if (q.pfc_watchdog_drops || q.drop_packets) qClass = ' qos-critical';
                    else if (q.pfc_pause_frames_rx || q.pfc_pause_frames_tx || q.ecn_marked_packets) qClass = ' qos-warning';
                    html += '<div class="nic-card-qos-row' + qClass + '">';
                    html += '<span class="qos-queue-name" title="Queue name / traffic class">' + esc(q.queue_name || '') + '</span>';
                    if (q.pfc_pause_frames_rx || q.pfc_pause_frames_tx) {
                        html += '<span title="PFC Pause Frames — switch asked the sender to slow down (Rx) or was asked to slow down (Tx). High values indicate congestion.">\u23f8 PFC ' + formatNumber(q.pfc_pause_frames_rx || 0) + '/' + formatNumber(q.pfc_pause_frames_tx || 0) + '</span>';
                    }
                    if (q.ecn_marked_packets) {
                        html += '<span title="ECN Marked Packets — packets marked with Explicit Congestion Notification, warning the sender to reduce rate before drops occur">\u26a0 ECN ' + formatNumber(q.ecn_marked_packets) + '</span>';
                    }
                    if (q.pfc_watchdog_drops) {
                        html += '<span class="counter-errors" title="PFC Watchdog Drops — packets dropped because a PFC storm was detected (queue stuck in paused state too long). Critical RDMA issue.">\u2717 WD ' + formatNumber(q.pfc_watchdog_drops) + '</span>';
                    }
                    if (q.drop_packets) {
                        html += '<span class="counter-errors" title="Queue Drops — packets dropped due to queue overflow. Indicates sustained congestion exceeding buffer capacity.">\u2717 Drops ' + formatNumber(q.drop_packets) + '</span>';
                    }
                    html += '</div>';
                }
                html += '</div>';
            }
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

function formatNumber(n) {
    if (!n) return '0';
    if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B';
    if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
    if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
    return String(n);
}

// Helper: info row (reused from switch view pattern)
function infoRow(label, value) {
    const esc = NM.core.escapeHtml;
    return '<div class="info-row"><span class="info-label">' + esc(label) + '</span><span class="info-value">' + esc(value) + '</span></div>';
}
