// sidebar.js — Detail panel for inspecting nodes and edges

'use strict';

const Sidebar = (() => {
    const sidebar = () => document.getElementById('sidebar');
    const title = () => document.getElementById('sidebar-title');
    const content = () => document.getElementById('sidebar-content');
    const closeBtn = () => document.getElementById('sidebar-close');

    let topology = null;
    let lastShowTime = 0;

    function init(topologyData) {
        topology = topologyData;
        closeBtn().addEventListener('click', () => {
            lastShowTime = 0; // allow explicit close
            hide();
        });
    }

    function setTopology(topologyData) {
        topology = topologyData;
    }

    function show() {
        sidebar().classList.remove('hidden');
        lastShowTime = Date.now();
    }

    function hide() {
        const elapsed = lastShowTime ? Date.now() - lastShowTime : 9999;
        if (lastShowTime && elapsed < 500) return;
        sidebar().classList.add('hidden');
        const cy = NetworkGraph.getInstance();
        if (cy) cy.elements().unselect();
    }

    function showNode(nodeData) {
        show();

        const type = nodeData.type || 'unknown';
        title().innerHTML = `<span class="type-badge ${type}">${type}</span> ${escapeHtml(nodeData.label || nodeData.id)}`;

        let html = '';

        if (type === 'vm') {
            // VM endpoint details
            html += '<div class="detail-section">';
            html += '<h3>VM Endpoint</h3>';
            if (nodeData.mac) html += detailRow('MAC Address', nodeData.mac);
            if (nodeData.ips && nodeData.ips.length > 0) {
                html += detailRow('IP Addresses', nodeData.ips.join(', '));
            }
            if (nodeData.host_device) html += detailRow('Host Device', nodeData.host_device);
            if (nodeData.host_port) html += detailRow('Host Port', nodeData.host_port);
            if (nodeData.switch_id) html += detailRow('Switch', nodeData.switch_id);
            if (nodeData.vlans && nodeData.vlans.length > 0) {
                html += detailRow('VLANs', nodeData.vlans.join(', '));
            }
            html += '</div>';
        } else {
            // Device info section
            html += '<div class="detail-section">';
            html += '<h3>Device Info</h3>';
            html += detailRow('ID', nodeData.id);
            if (nodeData.chassis_id) html += detailRow('Chassis ID', nodeData.chassis_id);
            if (nodeData.system_name) html += detailRow('System Name', nodeData.system_name);
            if (nodeData.system_description) html += detailRow('Description', nodeData.system_description);
            if (nodeData.mgmt_addr) html += detailRow('Management IP', nodeData.mgmt_addr);
            if (nodeData.software_version) html += detailRow('Software Version', nodeData.software_version);
            if (nodeData.uptime) html += detailRow('Uptime', nodeData.uptime);
            html += '</div>';

            // VLAN membership
            if (nodeData.vlans && nodeData.vlans.length > 0) {
                html += '<div class="detail-section">';
                html += '<h3>VLAN Membership</h3>';
                html += detailRow('VLANs', nodeData.vlans.join(', '));
                html += '</div>';
            }

            // Interface health summary
            if (nodeData.interfaces_total > 0) {
                html += '<div class="detail-section">';
                html += '<h3>Interface Health</h3>';
                const pct = Math.round((nodeData.interfaces_up / nodeData.interfaces_total) * 100);
                html += detailRow('Up / Total', `${nodeData.interfaces_up} / ${nodeData.interfaces_total}`);
                html += detailRow('Health', `${pct}%`);
                html += '</div>';
            }

            // Interface details from topology data
            if (topology) {
                const device = (topology.devices || []).find(d => d.id === nodeData.id);
                if (device && device.interfaces && device.interfaces.length > 0) {
                    html += '<div class="detail-section">';
                    html += `<h3>Interfaces (${device.interfaces.length})</h3>`;
                    html += '<ul class="interface-list">';
                    for (const iface of device.interfaces) {
                        const statusColor = iface.oper_status === 'UP' ? '#4CAF50' : (iface.oper_status === 'DOWN' ? '#e94560' : '#9E9E9E');
                        const statusDot = `<span style="display:inline-block;width:7px;height:7px;border-radius:50%;background:${statusColor};margin-right:4px;vertical-align:middle;"></span>`;
                        let detail = iface.speed || '';
                        if (iface.mtu) detail += detail ? ` · MTU ${iface.mtu}` : `MTU ${iface.mtu}`;
                        html += `<li>
                            <span class="port">${statusDot}${escapeHtml(iface.name)}</span>
                            <span class="remote">${escapeHtml(detail)}</span>
                        </li>`;
                    }
                    html += '</ul>';
                    html += '</div>';
                }

                // Show VM endpoints behind this host
                if (device && topology.endpoints) {
                    const eps = topology.endpoints.filter(ep => ep.host_device === nodeData.id);
                    if (eps.length > 0) {
                        html += '<div class="detail-section">';
                        html += `<h3>VM Endpoints (${eps.length})</h3>`;
                        html += '<ul class="interface-list">';
                        for (const ep of eps) {
                            const ipStr = (ep.ips && ep.ips.length > 0) ? ep.ips.join(', ') : 'no IP';
                            html += `<li>
                                <span class="port">${escapeHtml(ep.mac)}</span>
                                <span class="remote">${escapeHtml(ipStr)}</span>
                            </li>`;
                        }
                        html += '</ul>';
                        html += '</div>';
                    }
                }
            }

            // Find connections for this device
            if (topology && topology.links) {
                const connections = topology.links.filter(
                    (l) => l.local_device === nodeData.id || l.remote_device === nodeData.id
                );

                if (connections.length > 0) {
                    html += '<div class="detail-section">';
                    html += `<h3>Connections (${connections.length})</h3>`;
                    html += '<ul class="interface-list">';

                    for (const link of connections) {
                        const isLocal = link.local_device === nodeData.id;
                        const localPort = isLocal ? link.local_port : link.remote_port;
                        const remoteDevice = isLocal ? link.remote_device : link.local_device;
                        const remotePort = isLocal ? link.remote_port : link.local_port;

                        html += `<li>
                            <span class="port">${escapeHtml(localPort)}</span>
                            <span class="remote">→ ${escapeHtml(remoteDevice)} (${escapeHtml(remotePort)})</span>
                        </li>`;
                    }

                    html += '</ul>';
                    html += '</div>';
                }

                // Stats
                const connectedDevices = new Set();
                connections.forEach((l) => {
                    connectedDevices.add(l.local_device === nodeData.id ? l.remote_device : l.local_device);
                });

                html += '<div class="detail-section">';
                html += '<h3>Statistics</h3>';
                html += detailRow('Links', connections.length);
                html += detailRow('Connected Devices', connectedDevices.size);
                html += '</div>';
            }

            // Annotations
            if (nodeData.annotations && Object.keys(nodeData.annotations).length > 0) {
                html += '<div class="detail-section">';
                html += '<h3>Annotations</h3>';
                for (const [key, value] of Object.entries(nodeData.annotations)) {
                    html += detailRow(key, value);
                }
                html += '</div>';
            }
        }

        content().innerHTML = html;
    }

    function showEdge(edgeData) {
        show();

        title().innerHTML = `🔗 Link`;

        let html = '<div class="detail-section">';
        html += '<h3>Link Details</h3>';
        html += detailRow('Source Device', edgeData.source);
        html += detailRow('Source Port', edgeData.local_port || '—');
        html += detailRow('Target Device', edgeData.target);
        html += detailRow('Target Port', edgeData.remote_port || '—');
        if (edgeData.remote_chassis_id) html += detailRow('Remote Chassis', edgeData.remote_chassis_id);
        if (edgeData.oper_status) html += detailRow('Status', edgeData.oper_status);
        if (edgeData.speed) html += detailRow('Speed', edgeData.speed);
        if (edgeData.mtu) html += detailRow('MTU', edgeData.mtu);
        if (edgeData.source_type) html += detailRow('Source', edgeData.source_type);
        if (edgeData.discovered_at) html += detailRow('Discovered', edgeData.discovered_at);
        html += '</div>';

        // Show interface counters if available
        if (topology) {
            const link = (topology.links || []).find(l =>
                l.local_device === edgeData.source && l.local_port === edgeData.local_port &&
                l.remote_device === edgeData.target && l.remote_port === edgeData.remote_port
            );
            if (link && link.counters) {
                html += '<div class="detail-section">';
                html += '<h3>Counters</h3>';
                const c = link.counters;
                if (c.in_octets != null) html += detailRow('Rx Bytes', formatBytes(c.in_octets));
                if (c.out_octets != null) html += detailRow('Tx Bytes', formatBytes(c.out_octets));
                if (c.in_pkts != null) html += detailRow('Rx Packets', formatNumber(c.in_pkts));
                if (c.out_pkts != null) html += detailRow('Tx Packets', formatNumber(c.out_pkts));
                if (c.in_errors != null && c.in_errors > 0) html += detailRow('Rx Errors', c.in_errors);
                if (c.out_errors != null && c.out_errors > 0) html += detailRow('Tx Errors', c.out_errors);
                if (c.in_discards != null && c.in_discards > 0) html += detailRow('Rx Drops', c.in_discards);
                if (c.out_discards != null && c.out_discards > 0) html += detailRow('Tx Drops', c.out_discards);
                html += '</div>';
            }
        }

        content().innerHTML = html;
    }

    function formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return parseFloat((bytes / Math.pow(1024, i)).toFixed(1)) + ' ' + sizes[i];
    }

    function formatNumber(n) {
        if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B';
        if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
        if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
        return String(n);
    }

    function detailRow(label, value) {
        return `<div class="detail-row">
            <span class="label">${escapeHtml(String(label))}</span>
            <span class="value">${escapeHtml(String(value))}</span>
        </div>`;
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    return { init, show, hide, showNode, showEdge, setTopology };
})();
