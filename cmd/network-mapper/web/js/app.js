// app.js — Multi-view topology application
// Manages three views: Fabric Overview, Switch Detail, Host Detail

'use strict';

let currentTopology = null;

// ---- Data helpers ----

function classifySwitches(topology) {
    const deviceTypes = {};
    for (const d of (topology.devices || [])) {
        deviceTypes[d.id] = d.type || 'unknown';
    }

    const switchNeighborTypes = {};
    for (const link of (topology.links || [])) {
        const lt = deviceTypes[link.local_device] || 'unknown';
        const rt = deviceTypes[link.remote_device] || 'unknown';
        if (lt === 'switch') {
            if (!switchNeighborTypes[link.local_device]) switchNeighborTypes[link.local_device] = new Set();
            switchNeighborTypes[link.local_device].add(rt);
        }
        if (rt === 'switch') {
            if (!switchNeighborTypes[link.remote_device]) switchNeighborTypes[link.remote_device] = new Set();
            switchNeighborTypes[link.remote_device].add(lt);
        }
    }

    const roles = {};
    for (const d of (topology.devices || [])) {
        if (d.type !== 'switch') continue;
        const neighbors = switchNeighborTypes[d.id] || new Set();
        const hasNonSwitch = [...neighbors].some(t => t !== 'switch');
        roles[d.id] = hasNonSwitch ? 'leaf' : 'spine';
    }
    return roles;
}

function countHostsPerSwitch(topology) {
    const counts = {};
    for (const link of (topology.links || [])) {
        const localDev = (topology.devices || []).find(d => d.id === link.local_device);
        const remoteDev = (topology.devices || []).find(d => d.id === link.remote_device);
        if (!localDev || !remoteDev) continue;
        if (localDev.type === 'switch' && remoteDev.type === 'host') {
            counts[link.local_device] = (counts[link.local_device] || 0) + 1;
        }
        if (remoteDev.type === 'switch' && localDev.type === 'host') {
            counts[link.remote_device] = (counts[link.remote_device] || 0) + 1;
        }
    }
    return counts;
}

function countVMsPerHost(topology) {
    const vmCounts = {};
    for (const ep of (topology.endpoints || [])) {
        if (ep.host_device) {
            vmCounts[ep.host_device] = (vmCounts[ep.host_device] || 0) + 1;
        }
    }
    return vmCounts;
}

function getConnectedHosts(topology, switchId) {
    const hosts = [];
    for (const link of (topology.links || [])) {
        let hostId = null;
        if (link.local_device === switchId) hostId = link.remote_device;
        else if (link.remote_device === switchId) hostId = link.local_device;
        if (!hostId) continue;
        const dev = (topology.devices || []).find(d => d.id === hostId);
        if (dev && dev.type === 'host') hosts.push(dev);
    }
    return hosts;
}

function getConnectedSwitches(topology, hostId) {
    const switches = [];
    for (const link of (topology.links || [])) {
        let swId = null;
        if (link.local_device === hostId) swId = link.remote_device;
        else if (link.remote_device === hostId) swId = link.local_device;
        if (!swId) continue;
        const dev = (topology.devices || []).find(d => d.id === swId);
        if (dev && dev.type === 'switch') switches.push(dev);
    }
    return switches;
}

function getHostVMs(topology, hostId) {
    return (topology.endpoints || []).filter(ep => ep.host_device === hostId);
}

function getVMData(vmId) {
    if (!currentTopology) return null;
    // vmId format is "vm-<mac_no_colons>"
    const mac = vmId.replace(/^vm-/, '').replace(/(.{2})(?=.)/g, '$1:');
    return (currentTopology.endpoints || []).find(ep => {
        const normalizedMac = ep.mac.replace(/:/g, '');
        return vmId === 'vm-' + normalizedMac;
    }) || null;
}

// ---- View Manager ----

const ViewManager = (() => {
    let currentView = { view: 'fabric', deviceId: null };

    function getCurrentView() { return currentView; }

    function navigateTo(view, deviceId) {
        currentView = { view, deviceId: deviceId || null };
        updateBreadcrumb();
        renderCurrentView();
    }

    function navigateToFabric() {
        currentView = { view: 'fabric', deviceId: null };
        updateBreadcrumb();
        renderCurrentView();
    }

    function updateBreadcrumb() {
        const trail = document.getElementById('breadcrumb-trail');
        if (!trail) return;

        let html = '<span class="crumb clickable" data-index="0">Fabric Overview</span>';

        if (currentView.view === 'switch') {
            const name = getDeviceName(currentView.deviceId);
            html += ' <span class="crumb-sep">\u203A</span> ';
            html += '<span class="crumb current">Switch: ' + escapeHtml(name) + '</span>';
        } else if (currentView.view === 'host') {
            const switches = getConnectedSwitches(currentTopology, currentView.deviceId);
            if (switches.length > 0) {
                const sw = switches[0];
                const swName = sw.system_name || sw.id;
                html += ' <span class="crumb-sep">\u203A</span> ';
                html += '<span class="crumb clickable" data-view="switch" data-id="' + escapeHtml(sw.id) + '">Switch: ' + escapeHtml(swName) + '</span>';
            }
            html += ' <span class="crumb-sep">\u203A</span> ';
            const name = getDeviceName(currentView.deviceId);
            html += '<span class="crumb current">Host: ' + escapeHtml(name) + '</span>';
        } else if (currentView.view === 'vm') {
            // VM breadcrumb: Fabric > Switch > Host > VM
            const vmData = getVMData(currentView.deviceId);
            if (vmData && vmData.host_device) {
                const hostSwitches = getConnectedSwitches(currentTopology, vmData.host_device);
                if (hostSwitches.length > 0) {
                    const sw = hostSwitches[0];
                    html += ' <span class="crumb-sep">\u203A</span> ';
                    html += '<span class="crumb clickable" data-view="switch" data-id="' + escapeHtml(sw.id) + '">Switch: ' + escapeHtml(sw.system_name || sw.id) + '</span>';
                }
                const hostDev = (currentTopology.devices || []).find(d => d.id === vmData.host_device);
                if (hostDev) {
                    html += ' <span class="crumb-sep">\u203A</span> ';
                    html += '<span class="crumb clickable" data-view="host" data-id="' + escapeHtml(hostDev.id) + '">Host: ' + escapeHtml(hostDev.system_name || hostDev.id) + '</span>';
                }
            }
            html += ' <span class="crumb-sep">\u203A</span> ';
            const vmLabel = vmData ? ((vmData.ips && vmData.ips.length > 0) ? vmData.ips[0] : vmData.mac) : currentView.deviceId;
            html += '<span class="crumb current">VM: ' + escapeHtml(vmLabel) + '</span>';
        }

        trail.innerHTML = html;

        // Wire click handlers
        trail.querySelectorAll('.crumb.clickable').forEach(el => {
            el.addEventListener('click', () => {
                const view = el.dataset.view;
                const id = el.dataset.id;
                const index = el.dataset.index;
                if (index !== undefined) {
                    navigateToFabric();
                } else if (view && id) {
                    currentView = { view, deviceId: id };
                    updateBreadcrumb();
                    renderCurrentView();
                }
            });
        });
    }

    function getDeviceName(deviceId) {
        if (!currentTopology) return deviceId || '';
        const dev = (currentTopology.devices || []).find(d => d.id === deviceId);
        return dev ? (dev.system_name || dev.id) : deviceId || '';
    }

    function renderCurrentView() {
        if (!currentTopology) return;
        Popup.hide();
        Sidebar.hide();

        const cyEl = document.getElementById('cy');
        const detailEl = document.getElementById('detail-view');

        if (currentView.view === 'fabric') {
            // Fabric uses Cytoscape with compound switch nodes
            cyEl.classList.remove('hidden');
            detailEl.classList.add('hidden');
            detailEl.innerHTML = '';
            renderFabricView();
        } else {
            // Detail views use HTML diagrams
            cyEl.classList.add('hidden');
            detailEl.classList.remove('hidden');
            switch (currentView.view) {
                case 'switch': renderSwitchView(currentView.deviceId); break;
                case 'host':   renderHostView(currentView.deviceId); break;
                case 'vm':     renderVMView(currentView.deviceId); break;
            }
        }
    }

    return {
        getCurrentView,
        navigateTo,
        navigateToFabric,
        renderCurrentView,
    };
})();

// ---- View Renderers ----

function renderFabricView() {
    const topology = currentTopology;
    const roles = classifySwitches(topology);
    const hostCounts = countHostsPerSwitch(topology);
    const elements = [];

    const switches = (topology.devices || []).filter(d => d.type === 'switch');

    // Build port connection map for all switches
    const portMaps = {};
    for (const sw of switches) portMaps[sw.id] = {};

    for (const link of (topology.links || [])) {
        const localDev = (topology.devices || []).find(d => d.id === link.local_device);
        const remoteDev = (topology.devices || []).find(d => d.id === link.remote_device);
        if (!localDev || !remoteDev) continue;

        if (localDev.type === 'switch' && portMaps[link.local_device]) {
            portMaps[link.local_device][link.local_port] = {
                remoteId: link.remote_device,
                remoteName: remoteDev.system_name || remoteDev.id,
                remoteType: remoteDev.type || 'unknown',
                remotePort: link.remote_port || '',
                speed: link.speed || '',
            };
        }
        if (remoteDev.type === 'switch' && portMaps[link.remote_device]) {
            portMaps[link.remote_device][link.remote_port] = {
                remoteId: link.local_device,
                remoteName: localDev.system_name || localDev.id,
                remoteType: localDev.type || 'unknown',
                remotePort: link.local_port || '',
                speed: link.speed || '',
            };
        }
    }

    // Create compound switch nodes with port children
    for (const sw of switches) {
        const ifaces = sw.interfaces || [];
        const role = roles[sw.id] || 'leaf';
        const hCount = hostCounts[sw.id] || 0;
        const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;

        // Parent switch node
        elements.push({
            data: {
                id: sw.id,
                label: (sw.system_name || sw.id) + (hCount > 0 ? ' (' + hCount + ' hosts)' : ''),
                type: 'switch-parent',
                role: role,
                deviceType: 'switch',
                system_name: sw.system_name || '',
                interfaces_up: ifacesUp,
                interfaces_total: ifaces.length,
            },
        });

        // Port child nodes — create from connected interfaces OR from link data
        const createdPorts = new Set();
        const connectedPorts = ifaces.filter(iface => {
            const portName = iface.name || '';
            return portMaps[sw.id][portName] !== undefined;
        });

        for (const iface of connectedPorts) {
            const portName = iface.name || '';
            const conn = portMaps[sw.id][portName];
            const isUp = iface.oper_status === 'UP';
            const connType = conn ? conn.remoteType : (isUp ? 'none' : 'down');
            createdPorts.add(portName);

            elements.push({
                data: {
                    id: sw.id + '::port::' + portName,
                    parent: sw.id,
                    label: '',
                    type: 'port',
                    portName: portName,
                    connType: connType,
                    remoteId: conn ? conn.remoteId : '',
                    remoteName: conn ? conn.remoteName : '',
                    remoteType: conn ? conn.remoteType : '',
                    remotePort: conn ? conn.remotePort : '',
                    speed: conn ? conn.speed : '',
                    operStatus: isUp ? 'UP' : 'DOWN',
                    switchId: sw.id,
                },
            });
        }

        // Create port nodes from link data if no interface info exists
        for (const [portName, conn] of Object.entries(portMaps[sw.id])) {
            if (createdPorts.has(portName)) continue;
            createdPorts.add(portName);

            elements.push({
                data: {
                    id: sw.id + '::port::' + portName,
                    parent: sw.id,
                    label: '',
                    type: 'port',
                    portName: portName,
                    connType: conn.remoteType || 'unknown',
                    remoteId: conn.remoteId || '',
                    remoteName: conn.remoteName || '',
                    remoteType: conn.remoteType || '',
                    remotePort: conn.remotePort || '',
                    speed: conn.speed || '',
                    operStatus: 'UP',
                    switchId: sw.id,
                },
            });
        }
    }

    // Edges between ports (switch-to-switch links only)
    const addedEdges = new Set();
    for (const link of (topology.links || [])) {
        const localDev = (topology.devices || []).find(d => d.id === link.local_device);
        const remoteDev = (topology.devices || []).find(d => d.id === link.remote_device);
        if (!localDev || !remoteDev) continue;
        if (localDev.type !== 'switch' || remoteDev.type !== 'switch') continue;

        const sourcePort = link.local_device + '::port::' + link.local_port;
        const targetPort = link.remote_device + '::port::' + link.remote_port;
        const edgeKey = [sourcePort, targetPort].sort().join('|');
        if (addedEdges.has(edgeKey)) continue;
        addedEdges.add(edgeKey);

        elements.push({
            data: {
                id: 'edge::' + sourcePort + '::' + targetPort,
                source: sourcePort,
                target: targetPort,
                type: 'switch-link',
                localPort: link.local_port || '',
                remotePort: link.remote_port || '',
                speed: link.speed || '',
                operStatus: link.oper_status || 'UP',
            },
        });
    }

    // Render with preset positions (manual placement for compact view)
    // Classify: switches with the most total links are likely leaves (they connect to hosts/unknowns)
    // Switches that only connect to other switches are spines
    const switchLinkCounts = {};
    const switchToSwitchCounts = {};
    for (const sw of switches) {
        switchLinkCounts[sw.id] = Object.keys(portMaps[sw.id]).length;
        switchToSwitchCounts[sw.id] = 0;
    }
    for (const link of (topology.links || [])) {
        const localDev = (topology.devices || []).find(d => d.id === link.local_device);
        const remoteDev = (topology.devices || []).find(d => d.id === link.remote_device);
        if (!localDev || !remoteDev) continue;
        if (localDev.type === 'switch' && remoteDev.type === 'switch') {
            switchToSwitchCounts[localDev.id] = (switchToSwitchCounts[localDev.id] || 0) + 1;
            switchToSwitchCounts[remoteDev.id] = (switchToSwitchCounts[remoteDev.id] || 0) + 1;
        }
    }

    // Spine = switch where ALL links go to other switches (or very few total links)
    // Leaf = switch with many non-switch connections
    const spines = [];
    const leaves = [];
    for (const sw of switches) {
        const total = switchLinkCounts[sw.id] || 0;
        const toSwitch = switchToSwitchCounts[sw.id] || 0;
        if (total <= toSwitch + 1) {
            spines.push(sw.id);
        } else {
            leaves.push(sw.id);
        }
    }
    // If all are classified the same, just split by link count (fewer links = spine)
    if (spines.length === 0 || leaves.length === 0) {
        const sorted = [...switches].sort((a, b) => 
            (switchLinkCounts[a.id] || 0) - (switchLinkCounts[b.id] || 0)
        );
        spines.length = 0;
        leaves.length = 0;
        const splitAt = Math.max(1, Math.floor(sorted.length / 2));
        sorted.forEach((sw, i) => {
            if (i < splitAt) spines.push(sw.id);
            else leaves.push(sw.id);
        });
    }

    // Calculate positions: compact grid
    const hGap = 160;  // horizontal gap between switches
    const vGap = 200;  // vertical gap between spine and leaf rows

    // Center each row
    function rowPositions(ids, yPos) {
        const totalWidth = (ids.length - 1) * hGap;
        const startX = -totalWidth / 2;
        const positions = {};
        ids.forEach((id, i) => {
            positions[id] = { x: startX + i * hGap, y: yPos };
        });
        return positions;
    }

    const spinePositions = rowPositions(spines, 0);
    const leafPositions = rowPositions(leaves, vGap);
    const allPositions = { ...spinePositions, ...leafPositions };

    // Set positions on parent switch elements
    for (const el of elements) {
        if (el.data && el.data.type === 'switch-parent' && allPositions[el.data.id]) {
            el.position = allPositions[el.data.id];
        }
    }

    // Update roles on elements based on our classification
    for (const el of elements) {
        if (el.data && el.data.type === 'switch-parent') {
            el.data.role = spines.includes(el.data.id) ? 'spine' : 'leaf';
        }
    }

    NetworkGraph.render(elements, 'preset');

    const cy = NetworkGraph.getInstance();

    // Position ports inside parents after render
    NetworkGraph.arrangePortsInRows();

    // Remove previous fabric-specific handlers to avoid stacking
    cy.off('mouseover', 'node[type="port"]');
    cy.off('mouseout', 'node[type="port"]');
    cy.off('tap', 'node[type="port"]');
    cy.off('tap', 'node[type="switch-parent"]');

    // Port hover → show tooltip + highlight
    cy.on('mouseover', 'node[type="port"]', (evt) => {
        const node = evt.target;
        node.addClass('highlight');
        node.connectedEdges().addClass('highlight');
        showPortTooltip(node);
    });
    cy.on('mouseout', 'node[type="port"]', (evt) => {
        const node = evt.target;
        node.removeClass('highlight');
        node.connectedEdges().removeClass('highlight');
        hidePortTooltip();
    });

    // Port click → navigate
    cy.on('tap', 'node[type="port"]', (evt) => {
        const node = evt.target;
        const remoteId = node.data('remoteId');
        const remoteType = node.data('remoteType');
        if (!remoteId) return;
        if (remoteType === 'switch') ViewManager.navigateTo('switch', remoteId);
        else if (remoteType === 'host') ViewManager.navigateTo('host', remoteId);
    });

    // Switch parent click → navigate to switch detail
    cy.on('tap', 'node[type="switch-parent"]', (evt) => {
        ViewManager.navigateTo('switch', evt.target.data('id'));
    });

    setTimeout(() => NetworkGraph.fitToScreen(), 100);
}

// Port tooltip helpers
function showPortTooltip(node) {
    let tooltip = document.getElementById('port-hover-tooltip');
    if (!tooltip) {
        tooltip = document.createElement('div');
        tooltip.id = 'port-hover-tooltip';
        tooltip.className = 'port-hover-tooltip';
        document.body.appendChild(tooltip);
    }

    const data = node.data();
    let html = '<div class="port-tooltip-row"><span class="port-tooltip-label">Port:</span><span class="port-tooltip-value">' + escapeHtml(data.portName) + '</span></div>';
    html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + data.operStatus + '</span></div>';
    if (data.remoteId) {
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Connected to:</span><span class="port-tooltip-value">' + escapeHtml(data.remoteName) + '</span></div>';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Remote port:</span><span class="port-tooltip-value">' + escapeHtml(data.remotePort) + '</span></div>';
        html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Type:</span><span class="port-tooltip-value">' + escapeHtml(data.remoteType) + '</span></div>';
        if (data.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + escapeHtml(data.speed) + '</span></div>';
    }

    tooltip.innerHTML = html;
    tooltip.style.display = 'block';

    const pos = node.renderedPosition();
    const cyContainer = document.getElementById('cy').getBoundingClientRect();
    tooltip.style.left = (cyContainer.left + pos.x + 12) + 'px';
    tooltip.style.top = (cyContainer.top + pos.y - 10) + 'px';
}

function hidePortTooltip() {
    const tooltip = document.getElementById('port-hover-tooltip');
    if (tooltip) tooltip.style.display = 'none';
}

function renderSwitchView(switchId) {
    const topology = currentTopology;
    const swDev = (topology.devices || []).find(d => d.id === switchId);
    if (!swDev) return;

    const container = document.getElementById('detail-view');
    const roles = classifySwitches(topology);
    const role = roles[switchId] || 'leaf';
    const ifaces = swDev.interfaces || [];
    const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;

    // Build a map of port -> connected device info from links
    const portMap = {};
    for (const link of (topology.links || [])) {
        if (link.local_device === switchId) {
            const remote = (topology.devices || []).find(d => d.id === link.remote_device);
            portMap[link.local_port] = {
                remoteId: link.remote_device,
                remoteName: remote ? (remote.system_name || remote.id) : link.remote_device,
                remoteType: remote ? (remote.type || 'unknown') : 'unknown',
                remotePort: link.remote_port || '',
                operStatus: link.oper_status || '',
                speed: link.speed || '',
            };
        } else if (link.remote_device === switchId) {
            const remote = (topology.devices || []).find(d => d.id === link.local_device);
            portMap[link.remote_port] = {
                remoteId: link.local_device,
                remoteName: remote ? (remote.system_name || remote.id) : link.local_device,
                remoteType: remote ? (remote.type || 'unknown') : 'unknown',
                remotePort: link.local_port || '',
                operStatus: link.oper_status || '',
                speed: link.speed || '',
            };
        }
    }

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon switch">\u2B21</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + escapeHtml(swDev.system_name || swDev.id) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Role: <strong>' + escapeHtml(role) + '</strong></span>';
    html += '<span>Ports: <strong>' + ifacesUp + '/' + ifaces.length + ' UP</strong></span>';
    if (swDev.management_address) html += '<span>Mgmt: <strong>' + escapeHtml(swDev.management_address) + '</strong></span>';
    if (swDev.software_version) html += '<span>Version: <strong>' + escapeHtml(swDev.software_version) + '</strong></span>';
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
        if (conn) {
            classes += ' connected conn-' + conn.remoteType;
        }
        if (!isUp) classes += ' down';

        // Short port label (last segment)
        const shortName = portName.replace(/.*\//, '').replace(/Ethernet/, 'E');

        html += '<div class="' + classes + '"';
        if (conn) {
            html += ' data-remote-id="' + escapeHtml(conn.remoteId) + '"';
            html += ' data-remote-type="' + escapeHtml(conn.remoteType) + '"';
        }
        html += '>';
        html += '<span>' + escapeHtml(shortName) + '</span>';

        // Tooltip
        if (conn) {
            html += '<div class="port-tooltip">';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Port:</span><span class="port-tooltip-value">' + escapeHtml(portName) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Connected to:</span><span class="port-tooltip-value">' + escapeHtml(conn.remoteName) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Remote port:</span><span class="port-tooltip-value">' + escapeHtml(conn.remotePort) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Type:</span><span class="port-tooltip-value">' + escapeHtml(conn.remoteType) + '</span></div>';
            if (conn.speed) html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Speed:</span><span class="port-tooltip-value">' + escapeHtml(conn.speed) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + (isUp ? 'UP' : 'DOWN') + '</span></div>';
            html += '</div>';
        } else {
            html += '<div class="port-tooltip">';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Port:</span><span class="port-tooltip-value">' + escapeHtml(portName) + '</span></div>';
            html += '<div class="port-tooltip-row"><span class="port-tooltip-label">Status:</span><span class="port-tooltip-value">' + (isUp ? 'UP (no LLDP)' : 'DOWN') + '</span></div>';
            html += '</div>';
        }

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

    // Wire click events on connected ports
    container.querySelectorAll('.port-slot.connected').forEach(slot => {
        slot.addEventListener('click', () => {
            const remoteId = slot.dataset.remoteId;
            const remoteType = slot.dataset.remoteType;
            if (remoteType === 'switch') ViewManager.navigateTo('switch', remoteId);
            else if (remoteType === 'host') ViewManager.navigateTo('host', remoteId);
        });
    });
}

function renderHostView(hostId) {
    const topology = currentTopology;
    const hostDev = (topology.devices || []).find(d => d.id === hostId);
    if (!hostDev) return;

    const container = document.getElementById('detail-view');
    const ifaces = hostDev.interfaces || [];
    const ifacesUp = ifaces.filter(i => i.oper_status === 'UP').length;
    const vms = getHostVMs(topology, hostId);

    // Build link map for this host
    const portMap = {};
    for (const link of (topology.links || [])) {
        if (link.local_device === hostId) {
            const remote = (topology.devices || []).find(d => d.id === link.remote_device);
            portMap[link.local_port] = {
                remoteId: link.remote_device,
                remoteName: remote ? (remote.system_name || remote.id) : link.remote_device,
                remoteType: remote ? (remote.type || 'unknown') : 'unknown',
                remotePort: link.remote_port || '',
            };
        } else if (link.remote_device === hostId) {
            const remote = (topology.devices || []).find(d => d.id === link.local_device);
            portMap[link.remote_port] = {
                remoteId: link.local_device,
                remoteName: remote ? (remote.system_name || remote.id) : link.local_device,
                remoteType: remote ? (remote.type || 'unknown') : 'unknown',
                remotePort: link.local_port || '',
            };
        }
    }

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon host">\u2395</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + escapeHtml(hostDev.system_name || hostDev.id) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>NICs: <strong>' + ifacesUp + '/' + ifaces.length + ' UP</strong></span>';
    html += '<span>VMs: <strong>' + vms.length + '</strong></span>';
    if (hostDev.management_address) html += '<span>Mgmt: <strong>' + escapeHtml(hostDev.management_address) + '</strong></span>';
    if (hostDev.system_description) html += '<span>' + escapeHtml(hostDev.system_description) + '</span>';
    html += '</div></div></div>';

    // NIC diagram
    html += '<div class="nic-diagram">';
    html += '<div class="nic-diagram-title">Network Interfaces</div>';
    html += '<div class="nic-list">';

    for (const iface of ifaces) {
        const portName = iface.name || '';
        const conn = portMap[portName];
        const isUp = iface.oper_status === 'UP';
        const typeClass = conn ? conn.remoteType : '';

        html += '<div class="nic-card" data-remote-id="' + (conn ? escapeHtml(conn.remoteId) : '') + '" data-remote-type="' + (conn ? escapeHtml(conn.remoteType) : '') + '">';
        html += '<div class="nic-card-status ' + (isUp ? 'up' : 'down') + '"></div>';
        html += '<div class="nic-card-port">' + escapeHtml(portName) + '</div>';

        if (conn) {
            html += '<div class="nic-card-remote">\u2192 ' + escapeHtml(conn.remoteName) + ' (' + escapeHtml(conn.remotePort) + ')</div>';
            html += '<div class="nic-card-type ' + escapeHtml(typeClass) + '">' + escapeHtml(conn.remoteType) + '</div>';
        } else {
            html += '<div class="nic-card-remote" style="color:var(--text-muted)">' + (isUp ? 'No LLDP neighbor' : 'Down') + '</div>';
        }

        html += '</div>';
    }
    html += '</div></div>';

    // VM Cloud
    if (vms.length > 0) {
        html += '<div class="vm-cloud">';
        html += '<div class="vm-cloud-title">Virtual Machines</div>';

        // Stats
        const uniqueVlans = new Set();
        for (const vm of vms) { (vm.vlans || []).forEach(v => uniqueVlans.add(v)); }

        html += '<div class="vm-cloud-stats">';
        html += '<div class="vm-stat"><div class="vm-stat-value">' + vms.length + '</div><div class="vm-stat-label">Total VMs</div></div>';
        html += '<div class="vm-stat"><div class="vm-stat-value">' + uniqueVlans.size + '</div><div class="vm-stat-label">VLANs</div></div>';
        html += '</div>';

        // VM list (compact chips)
        html += '<div class="vm-list-compact">';
        const MAX_DISPLAY = 200;
        const displayVMs = vms.slice(0, MAX_DISPLAY);
        for (const vm of displayVMs) {
            const vmId = 'vm-' + vm.mac.replace(/:/g, '');
            const label = (vm.ips && vm.ips.length > 0) ? vm.ips[0] : vm.mac;
            html += '<div class="vm-chip" data-vm-id="' + escapeHtml(vmId) + '">';
            html += '<span class="vm-chip-dot"></span>';
            html += escapeHtml(label);
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
                if (remoteType === 'switch') ViewManager.navigateTo('switch', remoteId);
                else if (remoteType === 'host') ViewManager.navigateTo('host', remoteId);
            });
        }
    });

    // Wire VM chip clicks
    container.querySelectorAll('.vm-chip[data-vm-id]').forEach(chip => {
        chip.addEventListener('click', () => {
            ViewManager.navigateTo('vm', chip.dataset.vmId);
        });
    });
}

function renderVMView(vmId) {
    const topology = currentTopology;
    const vmData = getVMData(vmId);
    const container = document.getElementById('detail-view');

    if (!vmData) {
        container.innerHTML = '<div class="device-header"><div class="device-header-info"><div class="device-header-name">VM not found</div></div></div>';
        return;
    }

    const label = (vmData.ips && vmData.ips.length > 0) ? vmData.ips[0] : vmData.mac;
    const hostDev = (topology.devices || []).find(d => d.id === vmData.host_device);
    const switchDev = vmData.switch_id ? (topology.devices || []).find(d => d.id === vmData.switch_id) : null;

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon vm">\u2B22</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + escapeHtml(label) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Type: <strong>Virtual Machine</strong></span>';
    html += '</div></div></div>';

    // VM Info
    html += '<div class="vm-info-card">';
    html += '<div class="vm-info-card-title">Network Information</div>';

    html += '<div class="vm-info-row"><span class="vm-info-label">MAC Address</span><span class="vm-info-value">' + escapeHtml(vmData.mac) + '</span></div>';

    if (vmData.ips && vmData.ips.length > 0) {
        html += '<div class="vm-info-row"><span class="vm-info-label">IP Address(es)</span><span class="vm-info-value">' + escapeHtml(vmData.ips.join(', ')) + '</span></div>';
    }

    if (vmData.vlans && vmData.vlans.length > 0) {
        html += '<div class="vm-info-row"><span class="vm-info-label">VLAN(s)</span><span class="vm-info-value">' + escapeHtml(vmData.vlans.join(', ')) + '</span></div>';
    }

    html += '</div>';

    // Location info
    html += '<div class="vm-info-card">';
    html += '<div class="vm-info-card-title">Location</div>';

    if (hostDev) {
        html += '<div class="vm-info-row"><span class="vm-info-label">Host</span><span class="vm-info-value"><span class="link" data-view="host" data-id="' + escapeHtml(hostDev.id) + '">' + escapeHtml(hostDev.system_name || hostDev.id) + '</span></span></div>';
    }

    if (vmData.host_port) {
        html += '<div class="vm-info-row"><span class="vm-info-label">Host Port</span><span class="vm-info-value">' + escapeHtml(vmData.host_port) + '</span></div>';
    }

    if (switchDev) {
        html += '<div class="vm-info-row"><span class="vm-info-label">Switch</span><span class="vm-info-value"><span class="link" data-view="switch" data-id="' + escapeHtml(switchDev.id) + '">' + escapeHtml(switchDev.system_name || switchDev.id) + '</span></span></div>';
    }

    html += '</div>';

    container.innerHTML = html;

    // Wire link clicks
    container.querySelectorAll('.link[data-view]').forEach(link => {
        link.addEventListener('click', () => {
            ViewManager.navigateTo(link.dataset.view, link.dataset.id);
        });
    });
}

// ---- Inventory Panel ----

const Inventory = (() => {
    let topology = null;
    let filterText = '';

    const groupConfig = [
        { type: 'switch',  label: 'Switches',  color: '#0078d4' },
        { type: 'host',    label: 'Hosts',      color: '#44b700' },
        { type: 'bmc',     label: 'BMCs',       color: '#f7630c' },
        { type: 'unknown', label: 'Unknown',    color: '#8a8886' },
        { type: 'vm',      label: 'VMs',        color: '#a36efd' },
    ];

    const collapsedGroups = new Set(['vm']);

    function init(topologyData) {
        topology = topologyData;
        render();
        wireEvents();
    }

    function update(topologyData) {
        topology = topologyData;
        render();
    }

    function wireEvents() {
        const toggleBtn = document.getElementById('inventory-toggle');
        const expandBtn = document.getElementById('inventory-expand-btn');
        const panel = document.getElementById('inventory-panel');

        if (toggleBtn) {
            toggleBtn.addEventListener('click', () => {
                panel.classList.add('collapsed');
                expandBtn.classList.remove('hidden');
            });
        }
        if (expandBtn) {
            expandBtn.addEventListener('click', () => {
                panel.classList.remove('collapsed');
                expandBtn.classList.add('hidden');
            });
        }

        const filterInput = document.getElementById('inventory-search');
        if (filterInput) {
            filterInput.addEventListener('input', () => {
                filterText = filterInput.value.toLowerCase();
                render();
            });
        }
    }

    function render() {
        const container = document.getElementById('inventory-groups');
        if (!container || !topology) return;

        const devices = topology.devices || [];
        const endpoints = topology.endpoints || [];

        let html = '';
        for (const group of groupConfig) {
            let items;
            if (group.type === 'vm') {
                items = endpoints.map(ep => ({
                    id: 'vm-' + ep.mac.replace(/:/g, ''),
                    name: (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac,
                    type: 'vm',
                    hostId: ep.host_device || '',
                }));
            } else {
                items = devices.filter(d => d.type === group.type).map(d => ({
                    id: d.id,
                    name: d.system_name || d.id,
                    type: d.type,
                }));
            }

            if (filterText) {
                items = items.filter(i => i.name.toLowerCase().includes(filterText) || i.id.toLowerCase().includes(filterText));
            }

            if (items.length === 0 && !filterText) continue;

            const isCollapsed = collapsedGroups.has(group.type);
            html += '<div class="inv-group' + (isCollapsed ? ' collapsed' : '') + '">';
            html += '<div class="inv-group-header" data-type="' + group.type + '">';
            html += '<span class="inv-group-chevron">\u25BE</span>';
            html += '<span class="inv-group-dot" style="background:' + group.color + '"></span>';
            html += '<span>' + group.label + '</span>';
            html += '<span class="inv-group-count">' + items.length + '</span>';
            html += '</div>';

            html += '<div class="inv-group-items">';
            for (const item of items) {
                html += '<div class="inv-item" data-id="' + escapeHtml(item.id) + '" data-type="' + item.type + '" data-host="' + (item.hostId || '') + '">';
                html += '<span class="inv-item-label">' + escapeHtml(item.name) + '</span>';
                html += '</div>';
            }
            html += '</div>';
            html += '</div>';
        }

        container.innerHTML = html;

        // Wire group toggle
        container.querySelectorAll('.inv-group-header').forEach(header => {
            header.addEventListener('click', () => {
                const type = header.dataset.type;
                if (collapsedGroups.has(type)) collapsedGroups.delete(type);
                else collapsedGroups.add(type);
                render();
            });
        });

        // Wire item click -> navigate to appropriate view
        container.querySelectorAll('.inv-item').forEach(item => {
            item.addEventListener('click', () => {
                const id = item.dataset.id;
                const type = item.dataset.type;
                if (type === 'switch') {
                    ViewManager.navigateTo('switch', id);
                } else if (type === 'host') {
                    ViewManager.navigateTo('host', id);
                } else if (type === 'vm') {
                    ViewManager.navigateTo('vm', id);
                }
            });
        });
    }

    return { init, update };
})();

// ---- Helpers ----

function showError(message) {
    const overlay = document.getElementById('error-overlay');
    const msgEl = document.getElementById('error-message');
    overlay.classList.remove('hidden');
    msgEl.textContent = message;
}

let dismissedSignature = null;

function showWarnings(failures) {
    const banner = document.getElementById('warning-banner');
    const summary = document.getElementById('warning-summary');
    const details = document.getElementById('warning-details');
    const toggle = document.getElementById('warning-toggle');

    if (!failures || failures.length === 0) {
        banner.classList.add('hidden');
        return;
    }

    const sig = failures.map(f => f.switch + '|' + f.phase + '|' + f.message).sort().join('\n');
    if (sig === dismissedSignature) return;

    const groups = {};
    for (const f of failures) {
        const key = f.switch || 'unknown';
        if (!groups[key]) groups[key] = [];
        groups[key].push(f);
    }

    const switchNames = Object.keys(groups);
    const unreachable = switchNames.filter(s => groups[s].some(f => f.phase === 'connect'));
    const degraded = switchNames.filter(s => !groups[s].some(f => f.phase === 'connect'));

    const parts = [];
    if (unreachable.length > 0) parts.push(unreachable.length + ' switch' + (unreachable.length > 1 ? 'es' : '') + ' unreachable');
    if (degraded.length > 0) parts.push(degraded.length + ' switch' + (degraded.length > 1 ? 'es' : '') + ' with partial data');
    summary.textContent = parts.join(', ') + ' \u2014 topology may be incomplete';

    const sortedNames = [...unreachable, ...degraded];
    let html = '';
    for (const sw of sortedNames) {
        const isUnreachable = unreachable.includes(sw);
        const sevClass = isUnreachable ? 'unreachable' : 'degraded';
        const sevLabel = isUnreachable ? 'Unreachable' : 'Partial data';
        html += '<div class="warning-switch-group">';
        html += '<div class="warning-switch-name"><span class="severity-dot ' + sevClass + '" title="' + sevLabel + '"></span>' + escapeHtml(sw) + '</div>';
        for (const f of groups[sw]) {
            html += '<div class="warning-phase"><span class="phase-label">' + escapeHtml(f.phase) + ':</span> ' + escapeHtml(f.message) + '</div>';
        }
        html += '</div>';
    }
    details.innerHTML = html;

    toggle.onclick = () => {
        details.classList.toggle('hidden');
        toggle.textContent = details.classList.contains('hidden') ? '\u25BE' : '\u25B4';
    };
    document.getElementById('warning-dismiss').onclick = () => {
        dismissedSignature = sig;
        banner.classList.add('hidden');
        details.classList.add('hidden');
        toggle.textContent = '\u25BE';
    };
    banner.classList.remove('hidden');
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// ---- Main initialization ----

(async function () {
    try {
        const topology = await fetchTopology();
        currentTopology = topology;

        Toolbar.init();
        showWarnings(topology.partial_failures);

        // Initialize Cytoscape (kept for potential export functionality)
        NetworkGraph.init('cy', [], topology);

        Sidebar.init(topology);
        Popup.init(topology);
        Inventory.init(topology);

        // Render initial view
        ViewManager.navigateToFabric();

    } catch (err) {
        showError(err.message);
    }
})();

async function fetchTopology() {
    const resp = await fetch('/api/topology');
    if (!resp.ok) {
        let msg = 'HTTP ' + resp.status;
        try { const body = await resp.json(); if (body.error) msg = body.error; } catch (_) {}
        throw new Error(msg);
    }
    return resp.json();
}
