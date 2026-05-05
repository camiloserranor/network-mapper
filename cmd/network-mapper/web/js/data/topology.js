// data/topology.js — Pure data helper functions (no DOM, no side effects)

'use strict';

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
