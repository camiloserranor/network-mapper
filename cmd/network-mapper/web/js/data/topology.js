// data/topology.js — Pure data helper functions (no DOM, no side effects)

'use strict';

// --- V2 Schema Adapter ---
// Converts v2 hierarchical topology to the flat v1 format used by views.
// This lets all existing views work unchanged with v2 data.

NM.data.isV2 = function(topology) {
    return topology && topology.schema_version === '2.0';
};

// adaptV2 converts a v2 topology to v1-compatible flat format.
// If already v1, returns as-is.
NM.data.adaptV2 = function(topology) {
    if (!NM.data.isV2(topology)) return topology;

    var devices = [];
    var links = [];
    var endpoints = [];
    var vlans = [];
    var switchIds = {};

    // Fabric switches → devices + links
    var fabricSwitches = (topology.fabric && topology.fabric.switches) || [];
    for (var i = 0; i < fabricSwitches.length; i++) {
        var sw = fabricSwitches[i];
        switchIds[sw.id] = true;
        var dev = {
            id: sw.id,
            type: 'switch',
            chassis_id: sw.chassis_id || '',
            system_name: sw.name || sw.id,
            system_description: sw.system_description || '',
            management_address: sw.management_address || '',
            software_version: sw.software_version || '',
            uptime: sw.uptime || '',
            cpu_utilization: (sw.health && sw.health.cpu_utilization_pct) || 0,
            memory_used: (sw.health && sw.health.memory_used_bytes) || 0,
            memory_total: (sw.health && sw.health.memory_total_bytes) || 0,
            interfaces: sw.interfaces || [],
            bgp_sessions: sw.bgp_sessions || [],
            annotations: sw.annotations || {},
            vlans: []
        };
        devices.push(dev);

        // Peer links → links (switch↔switch)
        var peerLinks = sw.peer_links || [];
        for (var j = 0; j < peerLinks.length; j++) {
            var pl = peerLinks[j];
            links.push({
                local_device: sw.id,
                local_port: pl.local_port,
                remote_device: pl.remote_switch,
                remote_port: pl.remote_port,
                oper_status: pl.oper_status || '',
                speed: pl.speed || '',
                mtu: pl.mtu || '',
                source: 'lldp'
            });
        }

        // Connected hosts → links (switch↔host)
        var connHosts = sw.connected_hosts || [];
        for (var k = 0; k < connHosts.length; k++) {
            var ch = connHosts[k];
            links.push({
                local_device: sw.id,
                local_port: ch.port,
                remote_device: ch.host_id,
                remote_port: '',
                oper_status: ch.oper_status || '',
                speed: '',
                mtu: ch.mtu || '',
                source: 'lldp'
            });
        }
    }

    // Compute hosts → devices + endpoints
    var hosts = (topology.compute && topology.compute.hosts) || [];
    for (var h = 0; h < hosts.length; h++) {
        var host = hosts[h];
        var hostDev = {
            id: host.id,
            type: 'host',
            chassis_id: host.chassis_id || '',
            system_name: host.name || host.id,
            system_description: '',
            management_address: host.management_address || '',
            software_version: '',
            uptime: '',
            interfaces: [],
            bgp_sessions: [],
            annotations: host.annotations || {},
            vlans: []
        };

        // Build links from host connections (if not already in switch connected_hosts)
        var conns = host.connections || [];
        for (var c = 0; c < conns.length; c++) {
            var conn = conns[c];
            // Check if this link already exists from switch side
            var exists = false;
            for (var li = 0; li < links.length; li++) {
                if (links[li].local_device === conn.switch_id && links[li].local_port === conn.switch_port && links[li].remote_device === host.id) {
                    exists = true;
                    // Enrich with speed from connection
                    if (conn.speed) links[li].speed = conn.speed;
                    break;
                }
            }
            if (!exists) {
                links.push({
                    local_device: conn.switch_id,
                    local_port: conn.switch_port,
                    remote_device: host.id,
                    remote_port: '',
                    oper_status: conn.oper_status || '',
                    speed: conn.speed || '',
                    mtu: conn.mtu || '',
                    source: 'lldp'
                });
            }
        }

        // Host endpoints
        var hostEps = host.endpoints || [];
        for (var e = 0; e < hostEps.length; e++) {
            var ep = hostEps[e];
            endpoints.push({
                mac: ep.mac,
                ips: ep.ips || [],
                vlans: ep.vlans || [],
                host_device: host.id,
                host_port: ep.learned_on_port || '',
                switch_id: ep.learned_on_switch || '',
                type: ep.type || 'unknown'
            });
        }

        devices.push(hostDev);
    }

    // Unattributed endpoints
    var unattr = topology.compute && topology.compute.unattributed_endpoints;
    if (unattr && unattr.items) {
        for (var u = 0; u < unattr.items.length; u++) {
            var uep = unattr.items[u];
            endpoints.push({
                mac: uep.mac,
                ips: uep.ips || [],
                vlans: uep.vlans || [],
                host_device: '',
                host_port: uep.learned_on_port || '',
                switch_id: uep.learned_on_switch || '',
                type: uep.type || 'unknown'
            });
        }
    }

    // Unknown devices → devices
    var unknowns = (topology.unknown_devices && topology.unknown_devices.items) || [];
    for (var ud = 0; ud < unknowns.length; ud++) {
        var unk = unknowns[ud];
        var unkDev = {
            id: unk.id,
            type: 'unknown',
            chassis_id: unk.chassis_id || '',
            system_name: unk.id,
            system_description: unk.system_description || '',
            management_address: unk.management_address || '',
            interfaces: [],
            bgp_sessions: [],
            annotations: {}
        };
        devices.push(unkDev);

        // Add attachment links
        var attachments = unk.connected_to || [];
        for (var a = 0; a < attachments.length; a++) {
            var att = attachments[a];
            links.push({
                local_device: att.switch,
                local_port: att.port,
                remote_device: unk.id,
                remote_port: '',
                oper_status: att.oper_status || '',
                speed: '',
                mtu: att.mtu || '',
                source: 'lldp'
            });
        }
    }

    // VLANs
    var vlanItems = (topology.vlans && topology.vlans.items) || [];
    for (var vi = 0; vi < vlanItems.length; vi++) {
        var vlan = vlanItems[vi];
        var memberPorts = [];
        var sourceSw = '';
        var vlanSwitches = vlan.switches || [];
        for (var vs = 0; vs < vlanSwitches.length; vs++) {
            if (!sourceSw) sourceSw = vlanSwitches[vs].switch_name;
            var accessPorts = vlanSwitches[vs].access_ports || [];
            var trunkPorts = vlanSwitches[vs].trunk_ports || [];
            memberPorts = memberPorts.concat(accessPorts).concat(trunkPorts);
        }
        vlans.push({
            id: vlan.id,
            name: '',
            status: '',
            gateway: '',
            member_ports: memberPorts,
            source_switch: sourceSw
        });

        // Assign VLAN to host devices
        var vlanHosts = vlan.hosts || [];
        for (var vh = 0; vh < vlanHosts.length; vh++) {
            for (var di = 0; di < devices.length; di++) {
                if (devices[di].chassis_id === vlanHosts[vh].chassis_id ||
                    devices[di].management_address === vlanHosts[vh].management_ip) {
                    if (!devices[di].vlans) devices[di].vlans = [];
                    if (devices[di].vlans.indexOf(vlan.id) === -1) {
                        devices[di].vlans.push(vlan.id);
                    }
                }
            }
        }
    }

    return {
        schema_version: '1.0',
        collected_at: topology.metadata ? topology.metadata.collected_at : '',
        source_switches: topology.metadata ? topology.metadata.source_switches : [],
        partial_failures: topology.warnings || [],
        devices: devices,
        links: links,
        vlans: vlans,
        endpoints: endpoints,
        // Keep v2 reference for views that want to use v2 directly
        _v2: topology
    };
};

// Classify switches as spine or leaf based on neighbor types
NM.data.classifySwitches = function(topology) {
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
};

// Count hosts connected to each switch
NM.data.countHostsPerSwitch = function(topology) {
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
};

// Count VMs per host
NM.data.countVMsPerHost = function(topology) {
    const vmCounts = {};
    for (const ep of (topology.endpoints || [])) {
        if (ep.host_device) {
            vmCounts[ep.host_device] = (vmCounts[ep.host_device] || 0) + 1;
        }
    }
    return vmCounts;
};

// Get hosts connected to a switch
NM.data.getConnectedHosts = function(topology, switchId) {
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
};

// Get switches connected to a host
NM.data.getConnectedSwitches = function(topology, hostId) {
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
};

// Get VMs on a host
NM.data.getHostVMs = function(topology, hostId) {
    return (topology.endpoints || []).filter(ep => ep.host_device === hostId);
};

// Get VM data by vmId (format: "vm-<mac_no_colons>")
NM.data.getVMData = function(vmId) {
    const topology = NM.state.topology;
    if (!topology) return null;
    return (topology.endpoints || []).find(ep => {
        const normalizedMac = ep.mac.replace(/:/g, '');
        return vmId === 'vm-' + normalizedMac;
    }) || null;
};

// Build port connection map for a switch from link data
NM.data.buildPortMap = function(topology, switchId) {
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
    return portMap;
};
