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
                local_port: ch.local_port || ch.port || '',
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
            vlans: host.vlans || []
        };

        // Populate VLANs from connection port config (access, native, trunk)
        var conns = host.connections || [];
        var vlanSet = {};
        for (var vi2 = 0; vi2 < hostDev.vlans.length; vi2++) vlanSet[hostDev.vlans[vi2]] = true;
        for (var ci = 0; ci < conns.length; ci++) {
            var cn = conns[ci];
            if (cn.access_vlan && !vlanSet[cn.access_vlan]) { hostDev.vlans.push(cn.access_vlan); vlanSet[cn.access_vlan] = true; }
            if (cn.native_vlan && !vlanSet[cn.native_vlan]) { hostDev.vlans.push(cn.native_vlan); vlanSet[cn.native_vlan] = true; }
            var tvlans = cn.trunk_vlans || [];
            for (var tv = 0; tv < tvlans.length; tv++) {
                if (!vlanSet[tvlans[tv]]) { hostDev.vlans.push(tvlans[tv]); vlanSet[tvlans[tv]] = true; }
            }
        }

        // Build links from host connections (if not already in switch connected_hosts)
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

// Look up interface counters from the telemetry snapshot.
// Returns a counters object or null if unavailable.
NM.data.getInterfaceCounters = function(deviceId, interfaceName) {
    var telemetry = NM.state.telemetry;
    if (!telemetry || !telemetry.devices) return null;

    for (var i = 0; i < telemetry.devices.length; i++) {
        var dev = telemetry.devices[i];
        if (dev.id !== deviceId) continue;
        if (!dev.interfaces) return null;
        for (var j = 0; j < dev.interfaces.length; j++) {
            if (dev.interfaces[j].name === interfaceName) {
                return dev.interfaces[j].counters;
            }
        }
        return null;
    }
    return null;
};

// Get device health metrics from telemetry.
NM.data.getDeviceHealth = function(deviceId) {
    var telemetry = NM.state.telemetry;
    if (!telemetry || !telemetry.devices) return null;

    for (var i = 0; i < telemetry.devices.length; i++) {
        if (telemetry.devices[i].id === deviceId) {
            return telemetry.devices[i].health || null;
        }
    }
    return null;
};

// --- DCB Health Checks ---
// Computes network health findings from topology + telemetry data.
// Returns an array of finding objects: {severity, category, title, detail, deviceId, portName}
// severity: 'critical' | 'warning' | 'info'
// category: 'errors' | 'discards' | 'pfc' | 'mtu' | 'speed' | 'vlan' | 'link'
NM.data.computeHealthChecks = function(topology, telemetry) {
    if (!topology) return [];
    var findings = [];

    var devices = topology.devices || [];
    var links = topology.links || [];

    // Build device lookup
    var devMap = {};
    for (var i = 0; i < devices.length; i++) {
        devMap[devices[i].id] = devices[i];
    }

    // Build telemetry lookup
    var telDevMap = {};
    if (telemetry && telemetry.devices) {
        for (var i = 0; i < telemetry.devices.length; i++) {
            var td = telemetry.devices[i];
            telDevMap[td.id] = {};
            if (td.interfaces) {
                for (var j = 0; j < td.interfaces.length; j++) {
                    telDevMap[td.id][td.interfaces[j].name] = td.interfaces[j];
                }
            }
        }
    }

    function getTelIface(deviceId, portName) {
        var dev = telDevMap[deviceId];
        if (!dev) return null;
        return dev[portName] || null;
    }

    function getCounters(deviceId, portName) {
        var ti = getTelIface(deviceId, portName);
        return ti ? (ti.counters || null) : null;
    }

    function getInterface(deviceId, portName) {
        var dev = devMap[deviceId];
        if (!dev || !dev.interfaces) return null;
        for (var k = 0; k < dev.interfaces.length; k++) {
            if (dev.interfaces[k].name === portName) return dev.interfaces[k];
        }
        return null;
    }

    function deviceName(id) {
        var d = devMap[id];
        return d ? (d.system_name || d.id) : id;
    }

    function fmtNum(n) {
        if (n >= 1e12) return (n / 1e12).toFixed(1) + 'T';
        if (n >= 1e9) return (n / 1e9).toFixed(1) + 'G';
        if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
        if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
        return String(n);
    }

    // --- Counter-based checks (require telemetry) ---

    // Check 1: CRC errors
    for (var i = 0; i < devices.length; i++) {
        var dev = devices[i];
        if (dev.type !== 'switch') continue;
        var ifaces = dev.interfaces || [];
        for (var j = 0; j < ifaces.length; j++) {
            var iface = ifaces[j];
            var c = getCounters(dev.id, iface.name);
            if (!c) continue;
            if (c.crc_errors > 0) {
                findings.push({
                    severity: 'critical',
                    category: 'errors',
                    title: 'CRC errors detected',
                    detail: deviceName(dev.id) + ':' + iface.name + ' \u2014 ' + fmtNum(c.crc_errors) + ' CRC errors (physical layer issue)',
                    deviceId: dev.id,
                    portName: iface.name
                });
            }
        }
    }

    // Check 2: Interface errors (in_errors + out_errors)
    for (var i = 0; i < devices.length; i++) {
        var dev = devices[i];
        if (dev.type !== 'switch') continue;
        var ifaces = dev.interfaces || [];
        for (var j = 0; j < ifaces.length; j++) {
            var iface = ifaces[j];
            var c = getCounters(dev.id, iface.name);
            if (!c) continue;
            var totalErrors = (c.in_errors || 0) + (c.out_errors || 0);
            if (totalErrors > 0) {
                findings.push({
                    severity: 'warning',
                    category: 'errors',
                    title: 'Interface errors',
                    detail: deviceName(dev.id) + ':' + iface.name + ' \u2014 ' +
                            fmtNum(c.in_errors || 0) + ' in / ' + fmtNum(c.out_errors || 0) + ' out errors',
                    deviceId: dev.id,
                    portName: iface.name
                });
            }
        }
    }

    // Check 3: Interface discards (packet drops)
    for (var i = 0; i < devices.length; i++) {
        var dev = devices[i];
        if (dev.type !== 'switch') continue;
        var ifaces = dev.interfaces || [];
        for (var j = 0; j < ifaces.length; j++) {
            var iface = ifaces[j];
            var c = getCounters(dev.id, iface.name);
            if (!c) continue;
            var totalDiscards = (c.in_discards || 0) + (c.out_discards || 0);
            if (totalDiscards > 0) {
                findings.push({
                    severity: 'warning',
                    category: 'discards',
                    title: 'Packet discards',
                    detail: deviceName(dev.id) + ':' + iface.name + ' \u2014 ' +
                            fmtNum(c.in_discards || 0) + ' in / ' + fmtNum(c.out_discards || 0) + ' out discards',
                    deviceId: dev.id,
                    portName: iface.name
                });
            }
        }
    }

    // Check 4: PFC pause frames (flow-control congestion indicator)
    for (var i = 0; i < devices.length; i++) {
        var dev = devices[i];
        if (dev.type !== 'switch') continue;
        var ifaces = dev.interfaces || [];
        for (var j = 0; j < ifaces.length; j++) {
            var iface = ifaces[j];
            var c = getCounters(dev.id, iface.name);
            if (!c) continue;
            var totalPause = (c.pause_frames_in || 0) + (c.pause_frames_out || 0);
            if (totalPause > 0) {
                var sev = (c.in_discards > 0 || c.out_discards > 0 || c.in_errors > 0) ? 'warning' : 'info';
                findings.push({
                    severity: sev,
                    category: 'pfc',
                    title: 'PFC pause frames',
                    detail: deviceName(dev.id) + ':' + iface.name + ' \u2014 ' + fmtNum(totalPause) + ' pause frames' +
                            (c.in_discards > 0 ? ', ' + fmtNum(c.in_discards) + ' drops' : ''),
                    deviceId: dev.id,
                    portName: iface.name
                });
            }
        }
    }

    // --- Topology-based checks (work without telemetry) ---

    // Check 5: MTU mismatches across links
    for (var i = 0; i < links.length; i++) {
        var link = links[i];
        var localIf = getInterface(link.local_device, link.local_port);
        var remoteIf = getInterface(link.remote_device, link.remote_port);
        var localMtu = (localIf && localIf.mtu) || (link.mtu ? parseInt(link.mtu) : 0);
        var remoteMtu = (remoteIf && remoteIf.mtu) || 0;
        if (localMtu > 0 && remoteMtu > 0 && localMtu !== remoteMtu) {
            findings.push({
                severity: 'warning',
                category: 'mtu',
                title: 'MTU mismatch',
                detail: deviceName(link.local_device) + ':' + link.local_port + ' (MTU ' + localMtu + ') \u2194 ' +
                        deviceName(link.remote_device) + ':' + link.remote_port + ' (MTU ' + remoteMtu + ')',
                deviceId: link.local_device,
                portName: link.local_port
            });
        }
    }

    // Check 6: Speed mismatches across links (from telemetry speed field)
    for (var i = 0; i < links.length; i++) {
        var link = links[i];
        var localTel = getTelIface(link.local_device, link.local_port);
        var remoteTel = getTelIface(link.remote_device, link.remote_port);
        var localSpeed = (localTel && localTel.speed) || link.speed || '';
        var remoteSpeed = (remoteTel && remoteTel.speed) || '';
        if (localSpeed && remoteSpeed && localSpeed !== remoteSpeed) {
            findings.push({
                severity: 'warning',
                category: 'speed',
                title: 'Speed mismatch',
                detail: deviceName(link.local_device) + ':' + link.local_port + ' (' + localSpeed + ') \u2194 ' +
                        deviceName(link.remote_device) + ':' + link.remote_port + ' (' + remoteSpeed + ')',
                deviceId: link.local_device,
                portName: link.local_port
            });
        }
    }

    // Check 7: Native VLAN mismatch on inter-switch links
    for (var i = 0; i < links.length; i++) {
        var link = links[i];
        var localDev = devMap[link.local_device];
        var remoteDev = devMap[link.remote_device];
        if (!localDev || !remoteDev) continue;
        if (localDev.type !== 'switch' || remoteDev.type !== 'switch') continue;
        var localIf = getInterface(link.local_device, link.local_port);
        var remoteIf = getInterface(link.remote_device, link.remote_port);
        if (!localIf || !remoteIf) continue;
        var localNative = localIf.native_vlan || 0;
        var remoteNative = remoteIf.native_vlan || 0;
        if (localNative > 0 && remoteNative > 0 && localNative !== remoteNative) {
            findings.push({
                severity: 'warning',
                category: 'vlan',
                title: 'Native VLAN mismatch',
                detail: deviceName(link.local_device) + ':' + link.local_port + ' (native ' + localNative + ') \u2194 ' +
                        deviceName(link.remote_device) + ':' + link.remote_port + ' (native ' + remoteNative + ')',
                deviceId: link.local_device,
                portName: link.local_port
            });
        }
    }

    // Check 8: Port mode mismatch on inter-switch links (access vs trunk)
    for (var i = 0; i < links.length; i++) {
        var link = links[i];
        var localDev = devMap[link.local_device];
        var remoteDev = devMap[link.remote_device];
        if (!localDev || !remoteDev) continue;
        if (localDev.type !== 'switch' || remoteDev.type !== 'switch') continue;
        var localIf = getInterface(link.local_device, link.local_port);
        var remoteIf = getInterface(link.remote_device, link.remote_port);
        if (!localIf || !remoteIf) continue;
        var localMode = localIf.mode || '';
        var remoteMode = remoteIf.mode || '';
        if (localMode && remoteMode && localMode !== remoteMode) {
            findings.push({
                severity: 'warning',
                category: 'vlan',
                title: 'Port mode mismatch',
                detail: deviceName(link.local_device) + ':' + link.local_port + ' (' + localMode + ') \u2194 ' +
                        deviceName(link.remote_device) + ':' + link.remote_port + ' (' + remoteMode + ')',
                deviceId: link.local_device,
                portName: link.local_port
            });
        }
    }

    // Check 9: Link down — port has LLDP neighbor but is operationally down
    for (var i = 0; i < links.length; i++) {
        var link = links[i];
        var localIf = getInterface(link.local_device, link.local_port);
        var localTel = getTelIface(link.local_device, link.local_port);
        var operStatus = (localTel && localTel.oper_status) || (localIf && localIf.oper_status) || '';
        if (operStatus && operStatus.toUpperCase() === 'DOWN') {
            findings.push({
                severity: 'info',
                category: 'link',
                title: 'Connected port is down',
                detail: deviceName(link.local_device) + ':' + link.local_port + ' \u2192 ' +
                        deviceName(link.remote_device) + ' \u2014 port operationally DOWN but has known neighbor',
                deviceId: link.local_device,
                portName: link.local_port
            });
        }
    }

    // Check 10: QoS per-queue RDMA issues from v2 topology data
    var v2 = topology._v2;
    if (v2 && v2.fabric && v2.fabric.switches) {
        var v2Switches = v2.fabric.switches;
        for (var i = 0; i < v2Switches.length; i++) {
            var sw = v2Switches[i];
            var qosStats = sw.qos_stats || [];
            for (var q = 0; q < qosStats.length; q++) {
                var qs = qosStats[q];
                if (qs.pfc_watchdog_drops > 0) {
                    findings.push({
                        severity: 'critical',
                        category: 'pfc',
                        title: 'PFC Watchdog drops',
                        detail: (sw.name || sw.id) + ':' + qs.interface_name + ' queue ' + qs.queue_name +
                                ' \u2014 ' + fmtNum(qs.pfc_watchdog_drops) + ' watchdog-flushed packets (RDMA traffic loss)',
                        deviceId: sw.id,
                        portName: qs.interface_name
                    });
                }
                if (qs.ecn_marked_packets > 0) {
                    findings.push({
                        severity: 'warning',
                        category: 'pfc',
                        title: 'ECN congestion marking',
                        detail: (sw.name || sw.id) + ':' + qs.interface_name + ' queue ' + qs.queue_name +
                                ' \u2014 ' + fmtNum(qs.ecn_marked_packets) + ' ECN-marked packets',
                        deviceId: sw.id,
                        portName: qs.interface_name
                    });
                }
                if (qs.drop_packets > 0 && qs.queue_name && qs.queue_name.indexOf('q3') >= 0) {
                    findings.push({
                        severity: 'critical',
                        category: 'pfc',
                        title: 'RDMA queue drops',
                        detail: (sw.name || sw.id) + ':' + qs.interface_name + ' queue ' + qs.queue_name +
                                ' \u2014 ' + fmtNum(qs.drop_packets) + ' dropped packets on lossless queue',
                        deviceId: sw.id,
                        portName: qs.interface_name
                    });
                }
            }
        }
    }

    // Sort: critical first, then warning, then info
    var sevOrder = { critical: 0, warning: 1, info: 2 };
    findings.sort(function(a, b) { return (sevOrder[a.severity] || 9) - (sevOrder[b.severity] || 9); });

    return findings;
};
