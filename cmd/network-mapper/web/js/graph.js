// graph.js — Cytoscape.js initialization, styling, and layout management

'use strict';

const NetworkGraph = (() => {
    let cy = null;
    let currentLayout = 'dagre';

    // Azure portal-inspired color map for device types
    const typeColors = {
        switch:  '#0078d4',
        host:    '#44b700',
        bmc:     '#f7630c',
        vm:      '#a36efd',
        unknown: '#8a8886',
    };

    const typeShapes = {
        switch:  'round-rectangle',
        host:    'round-rectangle',
        bmc:     'round-rectangle',
        vm:      'round-rectangle',
        unknown: 'ellipse',
    };

    // SVG icon data URIs — clean Fluent-style line icons
    const typeIcons = {
        // Network switch: rectangle with 4 port indicators
        switch: 'data:image/svg+xml,' + encodeURIComponent(
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">' +
            '<rect x="4" y="12" width="40" height="24" rx="3" stroke="#0078d4" stroke-width="2.5" fill="none"/>' +
            '<circle cx="13" cy="21" r="2.5" fill="#0078d4"/>' +
            '<circle cx="22" cy="21" r="2.5" fill="#0078d4"/>' +
            '<circle cx="31" cy="21" r="2.5" fill="#0078d4"/>' +
            '<circle cx="40" cy="21" r="2.5" fill="none" stroke="#0078d4" stroke-width="1.5"/>' +
            '<line x1="10" y1="30" x2="38" y2="30" stroke="#0078d4" stroke-width="1.5" stroke-linecap="round" stroke-dasharray="3 3"/>' +
            '</svg>'
        ),
        // Server / Arc machine: server tower with status light
        host: 'data:image/svg+xml,' + encodeURIComponent(
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">' +
            '<rect x="8" y="4" width="32" height="40" rx="3" stroke="#44b700" stroke-width="2.5" fill="none"/>' +
            '<line x1="8" y1="18" x2="40" y2="18" stroke="#44b700" stroke-width="1.5"/>' +
            '<line x1="8" y1="32" x2="40" y2="32" stroke="#44b700" stroke-width="1.5"/>' +
            '<circle cx="14" cy="11" r="2" fill="#44b700"/>' +
            '<line x1="20" y1="11" x2="34" y2="11" stroke="#44b700" stroke-width="2" stroke-linecap="round"/>' +
            '<circle cx="14" cy="25" r="2" fill="#44b700"/>' +
            '<line x1="20" y1="25" x2="34" y2="25" stroke="#44b700" stroke-width="2" stroke-linecap="round"/>' +
            '<circle cx="14" cy="39" r="2" fill="#44b700" opacity="0.4"/>' +
            '<line x1="20" y1="39" x2="34" y2="39" stroke="#44b700" stroke-width="2" stroke-linecap="round" opacity="0.4"/>' +
            '</svg>'
        ),
        // Virtual machine: monitor with VM badge
        vm: 'data:image/svg+xml,' + encodeURIComponent(
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">' +
            '<rect x="6" y="6" width="36" height="26" rx="3" stroke="#a36efd" stroke-width="2.5" fill="none"/>' +
            '<line x1="18" y1="32" x2="30" y2="32" stroke="#a36efd" stroke-width="2.5" stroke-linecap="round"/>' +
            '<line x1="24" y1="32" x2="24" y2="38" stroke="#a36efd" stroke-width="2.5"/>' +
            '<line x1="16" y1="38" x2="32" y2="38" stroke="#a36efd" stroke-width="2.5" stroke-linecap="round"/>' +
            '<path d="M16 15 L21 23 L26 15" stroke="#a36efd" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" fill="none"/>' +
            '<path d="M26 15 L29 23 L32 15" stroke="#a36efd" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" fill="none"/>' +
            '</svg>'
        ),
        // BMC: chip/board management icon
        bmc: 'data:image/svg+xml,' + encodeURIComponent(
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">' +
            '<rect x="12" y="12" width="24" height="24" rx="3" stroke="#f7630c" stroke-width="2.5" fill="none"/>' +
            '<rect x="18" y="18" width="12" height="12" rx="1.5" stroke="#f7630c" stroke-width="1.5" fill="#f7630c" fill-opacity="0.15"/>' +
            '<line x1="16" y1="8" x2="16" y2="12" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="24" y1="8" x2="24" y2="12" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="32" y1="8" x2="32" y2="12" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="16" y1="36" x2="16" y2="40" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="24" y1="36" x2="24" y2="40" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="32" y1="36" x2="32" y2="40" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="8" y1="16" x2="12" y2="16" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="8" y1="24" x2="12" y2="24" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="8" y1="32" x2="12" y2="32" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="36" y1="16" x2="40" y2="16" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="36" y1="24" x2="40" y2="24" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '<line x1="36" y1="32" x2="40" y2="32" stroke="#f7630c" stroke-width="2" stroke-linecap="round"/>' +
            '</svg>'
        ),
    };

    // Cytoscape style definitions — Azure portal dark theme
    const graphStyle = [
        // Default node style
        {
            selector: 'node',
            style: {
                'label': 'data(label)',
                'width': 48,
                'height': 48,
                'font-size': '11px',
                'font-family': 'Segoe UI, sans-serif',
                'color': '#d2d0ce',
                'text-valign': 'bottom',
                'text-halign': 'center',
                'text-margin-y': 8,
                'text-outline-width': 2,
                'text-outline-color': '#1b1a19',
                'background-color': '#323130',
                'border-width': 1.5,
                'border-color': '#605e5c',
                'overlay-padding': 6,
                'transition-property': 'background-color, border-color, width, height, opacity',
                'transition-duration': '0.2s',
            },
        },
        // Switch nodes
        {
            selector: 'node[type="switch"]',
            style: {
                'background-color': '#252423',
                'background-image': typeIcons.switch,
                'background-fit': 'contain',
                'background-clip': 'node',
                'background-width': '70%',
                'background-height': '70%',
                'shape': typeShapes.switch,
                'width': 56,
                'height': 42,
                'border-color': typeColors.switch,
                'border-width': 2,
            },
        },
        // Spine switch nodes (larger, bolder border)
        {
            selector: 'node[type="switch"][role="spine"]',
            style: {
                'width': 64,
                'height': 48,
                'border-width': 2.5,
                'border-color': '#2899f5',
            },
        },
        // Host nodes (Arc machines)
        {
            selector: 'node[type="host"]',
            style: {
                'background-color': '#252423',
                'background-image': typeIcons.host,
                'background-fit': 'contain',
                'background-clip': 'node',
                'background-width': '65%',
                'background-height': '65%',
                'shape': typeShapes.host,
                'width': 48,
                'height': 48,
                'border-color': typeColors.host,
                'border-width': 2,
            },
        },
        // BMC nodes
        {
            selector: 'node[type="bmc"]',
            style: {
                'background-color': '#252423',
                'background-image': typeIcons.bmc,
                'background-fit': 'contain',
                'background-clip': 'node',
                'background-width': '65%',
                'background-height': '65%',
                'shape': typeShapes.bmc,
                'width': 36,
                'height': 36,
                'border-color': typeColors.bmc,
                'border-width': 1.5,
            },
        },
        // Unknown nodes
        {
            selector: 'node[type="unknown"]',
            style: {
                'background-color': '#323130',
                'border-color': typeColors.unknown,
            },
        },
        // VM endpoint nodes
        {
            selector: 'node[type="vm"]',
            style: {
                'background-color': '#252423',
                'background-image': typeIcons.vm,
                'background-fit': 'contain',
                'background-clip': 'node',
                'background-width': '65%',
                'background-height': '65%',
                'shape': typeShapes.vm,
                'width': 32,
                'height': 32,
                'font-size': '9px',
                'border-color': typeColors.vm,
                'border-width': 1.5,
            },
        },
        // VLAN compound node style
        {
            selector: 'node.vlan-group',
            style: {
                'background-color': 'data(vlanColor)',
                'background-opacity': 0.08,
                'border-width': 1.5,
                'border-color': 'data(vlanColor)',
                'border-opacity': 0.4,
                'border-style': 'solid',
                'shape': 'round-rectangle',
                'padding': '30px',
                'label': 'data(label)',
                'font-size': '12px',
                'font-family': 'Segoe UI, sans-serif',
                'font-weight': '600',
                'color': 'data(vlanColor)',
                'text-valign': 'top',
                'text-halign': 'center',
                'text-margin-y': -8,
                'text-outline-width': 2,
                'text-outline-color': '#1b1a19',
                'events': 'no',
            },
        },
        // Compound parent node style (TOR grouping)
        {
            selector: 'node:parent',
            style: {
                'background-color': 'rgba(50, 49, 48, 0.5)',
                'border-width': 1,
                'border-color': 'rgba(96, 94, 92, 0.3)',
                'border-style': 'dashed',
                'shape': 'round-rectangle',
                'padding': '30px',
                'label': 'data(label)',
                'font-size': '10px',
                'font-family': 'Segoe UI, sans-serif',
                'color': 'rgba(161, 159, 157, 0.7)',
                'text-valign': 'top',
                'text-halign': 'center',
                'text-margin-y': -6,
                'text-outline-width': 0,
            },
        },
        // Selected node
        {
            selector: 'node:selected',
            style: {
                'border-width': 3,
                'border-color': '#0078d4',
                'overlay-color': '#0078d4',
                'overlay-opacity': 0.15,
            },
        },
        // Hovered node neighbor highlight
        {
            selector: 'node.highlight',
            style: {
                'border-width': 3,
                'border-color': '#0078d4',
                'overlay-color': '#0078d4',
                'overlay-opacity': 0.1,
            },
        },
        // Dimmed (non-highlighted) nodes
        {
            selector: 'node.dimmed',
            style: {
                'opacity': 0.25,
            },
        },
        // Default edge style
        {
            selector: 'edge',
            style: {
                'width': 1.5,
                'line-color': '#605e5c',
                'curve-style': 'bezier',
                'opacity': 0.6,
                'transition-property': 'line-color, width, opacity',
                'transition-duration': '0.2s',
            },
        },
        // Highlighted edges
        {
            selector: 'edge.highlight',
            style: {
                'line-color': '#0078d4',
                'width': 2.5,
                'opacity': 1,
            },
        },
        // Dimmed edges
        {
            selector: 'edge.dimmed',
            style: {
                'opacity': 0.1,
            },
        },
        // Edges with visible labels (on hover)
        {
            selector: 'edge.show-label',
            style: {
                'label': 'data(edgeLabel)',
                'font-size': '9px',
                'font-family': 'Segoe UI, sans-serif',
                'color': '#a19f9d',
                'text-rotation': 'autorotate',
                'text-outline-width': 1,
                'text-outline-color': '#1b1a19',
            },
        },
        // Search match
        {
            selector: 'node.search-match',
            style: {
                'border-width': 3,
                'border-color': '#fce100',
                'overlay-color': '#fce100',
                'overlay-opacity': 0.2,
            },
        },
        // Down edges (oper_status = DOWN)
        {
            selector: 'edge[oper_status="DOWN"]',
            style: {
                'line-color': '#d13438',
                'line-style': 'dashed',
                'opacity': 0.9,
            },
        },
    ];

    // Rank tiers: spine switches (0), leaf switches (1), hosts/BMC (2), VMs bottom (3)
    const typeRank = { bmc: 2, switch: 1, host: 2, vm: 3, unknown: 2 };

    function getNodeRank(node) {
        if (node.data('type') === 'switch' && node.data('role') === 'spine') return 0;
        return typeRank[node.data('type')] ?? 2;
    }

    // Layout configurations
    const layouts = {
        cose: {
            name: 'cose',
            animate: true,
            animationDuration: 800,
            animationEasing: 'ease-out',
            nodeRepulsion: function() { return 8000; },
            idealEdgeLength: function() { return 120; },
            edgeElasticity: function() { return 100; },
            gravity: 0.25,
            numIter: 1000,
            padding: 50,
            randomize: false,
            componentSpacing: 100,
            nestingFactor: 1.2,
        },
        dagre: {
            name: 'dagre',
            animate: false, // we animate manually after tier adjustment
            rankDir: 'TB',
            nodeSep: 80,
            rankSep: 140,
            padding: 50,
        },
    };

    // Tier Y positions for hierarchical layout
    const tierGap = 200;
    const tierY = { 0: 80, 1: 80 + tierGap, 2: 80 + tierGap * 2, 3: 80 + tierGap * 3 };

    function applyTieredLayout() {
        if (!cy) return;

        // Group nodes by tier (skip compound/parent nodes)
        const tiers = { 0: [], 1: [], 2: [], 3: [] };
        cy.nodes().forEach((n) => {
            if (n.isParent()) return;
            const rank = getNodeRank(n);
            tiers[rank].push(n);
        });

        // Sort nodes within each tier by label for consistent ordering
        for (const rank of [0, 1, 2, 3]) {
            tiers[rank].sort((a, b) => (a.data('label') || '').localeCompare(b.data('label') || ''));
        }

        // Calculate positions: center each tier horizontally
        const nodeSep = 120;
        const positions = {};

        for (const rank of [0, 1, 2, 3]) {
            const nodes = tiers[rank];
            const totalWidth = (nodes.length - 1) * nodeSep;
            const startX = -totalWidth / 2;

            nodes.forEach((n, i) => {
                positions[n.id()] = { x: startX + i * nodeSep, y: tierY[rank] };
            });
        }

        // Animate to tiered positions
        cy.nodes().forEach((n) => {
            const pos = positions[n.id()];
            if (pos) n.animate({ position: pos, duration: 600, easing: 'ease-in-out-quad' });
        });

        // Fit after animation
        setTimeout(() => cy.fit(50), 650);
    }

    function init(containerId, elements) {
        cy = cytoscape({
            container: document.getElementById(containerId),
            elements: elements,
            style: graphStyle,
            layout: { name: 'preset' }, // start with no layout; we apply tiered manually
            minZoom: 0.1,
            maxZoom: 4,
            wheelSensitivity: 0.3,
        });

        setupInteraction();

        // Apply default tiered layout
        applyTieredLayout();

        return cy;
    }

    function setupInteraction() {
        // Hover: highlight connected elements (skip VLAN compound nodes)
        cy.on('mouseover', 'node', (evt) => {
            const node = evt.target;
            if (node.hasClass('vlan-group')) return;
            const neighborhood = node.neighborhood().add(node);

            cy.elements().not('.vlan-group').addClass('dimmed');
            neighborhood.removeClass('dimmed');
            node.addClass('highlight');
            node.connectedEdges().addClass('highlight show-label');
            node.neighborhood('node').addClass('highlight');
        });

        cy.on('mouseout', 'node', () => {
            cy.elements().removeClass('dimmed highlight show-label');
        });

        // Hover on edges: show labels
        cy.on('mouseover', 'edge', (evt) => {
            evt.target.addClass('show-label highlight');
        });

        cy.on('mouseout', 'edge', (evt) => {
            evt.target.removeClass('show-label highlight');
        });
    }

    function runLayout(name) {
        if (!cy) return;

        currentLayout = name;

        if (name === 'dagre') {
            applyTieredLayout();
        } else {
            const config = layouts[name];
            if (config) cy.layout(config).run();
        }
    }

    function fitToScreen() {
        if (cy) cy.fit(50);
    }

    function filterByType(type) {
        if (!cy) return;
        if (type === 'all') {
            cy.elements().show();
        } else {
            cy.nodes().hide();
            cy.edges().hide();
            const matchingNodes = cy.nodes(`[type="${type}"]`);
            matchingNodes.show();
            matchingNodes.connectedEdges().show();
            matchingNodes.connectedEdges().connectedNodes().show();
        }
        runLayout(currentLayout);
    }

    function searchNodes(query) {
        if (!cy) return;
        cy.nodes().removeClass('search-match');

        if (!query || query.trim() === '') return;

        const q = query.toLowerCase();
        cy.nodes().forEach((node) => {
            const label = (node.data('label') || '').toLowerCase();
            const id = (node.data('id') || '').toLowerCase();
            const chassisId = (node.data('chassis_id') || '').toLowerCase();
            if (label.includes(q) || id.includes(q) || chassisId.includes(q)) {
                node.addClass('search-match');
            }
        });
    }

    function exportPNG() {
        if (!cy) return;
        const png = cy.png({ full: true, scale: 2, bg: '#0d1117' });
        const link = document.createElement('a');
        link.href = png;
        link.download = 'network-topology.png';
        link.click();
    }

    let grouped = false;

    function toggleGroupByTOR(elements) {
        if (!cy) return;
        grouped = !grouped;

        if (grouped) {
            // Build parent-child map: for each switch, find connected non-switch nodes
            const switches = new Set();
            cy.nodes().forEach(n => {
                if (n.data('type') === 'switch') switches.add(n.id());
            });

            // Track which non-switch node connects to which switch(es)
            const nodeToSwitches = {};
            cy.edges().forEach(e => {
                const src = e.source().id();
                const tgt = e.target().id();
                if (switches.has(src) && !switches.has(tgt)) {
                    (nodeToSwitches[tgt] = nodeToSwitches[tgt] || []).push(src);
                }
                if (switches.has(tgt) && !switches.has(src)) {
                    (nodeToSwitches[src] = nodeToSwitches[src] || []).push(tgt);
                }
            });

            // Add group parent nodes for each switch
            switches.forEach(swId => {
                const groupId = 'group-' + swId;
                if (cy.getElementById(groupId).length === 0) {
                    cy.add({
                        group: 'nodes',
                        data: {
                            id: groupId,
                            label: swId + ' group',
                            type: 'group',
                        },
                    });
                }

                // Move the switch into its own group
                cy.getElementById(swId).move({ parent: groupId });
            });

            // Move each child into its primary switch group
            for (const [nodeId, swIds] of Object.entries(nodeToSwitches)) {
                const primarySwitch = swIds[0]; // first connected switch
                const groupId = 'group-' + primarySwitch;
                cy.getElementById(nodeId).move({ parent: groupId });
            }
        } else {
            // Ungroup: move all nodes to root and remove group nodes
            cy.nodes().forEach(n => {
                if (n.isChild()) n.move({ parent: null });
            });
            cy.nodes('[type="group"]').remove();
        }

        // Re-apply layout
        runLayout(currentLayout);
        return grouped;
    }

    function isGrouped() { return grouped; }

    function getInstance() {
        return cy;
    }

    function getCurrentLayout() {
        return currentLayout;
    }

    function updateElements(newElements) {
        if (!cy) return;

        const existingIds = new Set();
        cy.elements().forEach(e => existingIds.add(e.id()));

        const newIds = new Set();
        newElements.forEach(e => newIds.add(e.data.id));

        // Remove elements that no longer exist
        cy.elements().forEach(e => {
            if (!newIds.has(e.id())) {
                e.remove();
            }
        });

        // Add or update elements
        for (const el of newElements) {
            const existing = cy.getElementById(el.data.id);
            if (existing.length > 0) {
                // Update data fields
                existing.data(el.data);
            } else {
                cy.add(el);
            }
        }

        // Re-run layout for new nodes
        const addedCount = newElements.filter(e => !existingIds.has(e.data.id)).length;
        const removedCount = [...existingIds].filter(id => !newIds.has(id)).length;
        if (addedCount > 0 || removedCount > 0) {
            runLayout(currentLayout);
        }
    }

    // VLAN color palette — muted tones consistent with Azure portal
    const vlanPalette = [
        '#0078d4', '#00b7c3', '#57a300', '#e3008c', '#8764b8',
        '#f7630c', '#107c10', '#ca5010', '#005b70', '#4f6bed',
        '#c239b3', '#498205', '#005a9e', '#d13438', '#7a7574',
    ];

    function getVLANColor(vlanId) {
        return vlanPalette[vlanId % vlanPalette.length];
    }

    let vlanGrouped = false;

    function toggleGroupByVLAN(topology) {
        if (!cy || !topology) return;

        if (vlanGrouped) {
            // Ungroup: move all nodes back to root and remove VLAN group nodes
            cy.nodes().forEach(n => {
                if (n.isChild()) {
                    n.move({ parent: null });
                }
                n.style('display', 'element');
            });
            cy.edges().forEach(e => e.style('display', 'element'));
            cy.nodes('.vlan-group').remove();
            vlanGrouped = false;
            grouped = false;
            runLayout(currentLayout);
            return;
        }

        // First, undo any TOR grouping
        if (grouped) {
            cy.nodes().forEach(n => {
                if (n.isChild()) n.move({ parent: null });
            });
            cy.nodes(':parent').remove();
            grouped = false;
        }

        // Build VLAN membership: each device gets exactly ONE primary VLAN (first in list)
        // Switches are excluded — they span all VLANs and stay at top level
        const devicePrimaryVLAN = {};
        if (topology.devices) {
            topology.devices.forEach(d => {
                if (d.type === 'switch') return; // switches stay ungrouped
                if (d.vlans && d.vlans.length > 0) {
                    devicePrimaryVLAN[d.id] = d.vlans[0];
                }
            });
        }
        if (topology.endpoints) {
            topology.endpoints.forEach(ep => {
                const epId = 'vm-' + ep.mac.replace(/:/g, '');
                if (ep.vlans && ep.vlans.length > 0) {
                    devicePrimaryVLAN[epId] = ep.vlans[0];
                }
            });
        }

        // Get all VLAN IDs and names
        const allVLANs = new Set();
        const vlanNames = {};
        if (topology.vlans) {
            topology.vlans.forEach(v => {
                allVLANs.add(v.id);
                vlanNames[v.id] = v.name || `VLAN ${v.id}`;
            });
        }
        Object.values(devicePrimaryVLAN).forEach(v => allVLANs.add(v));

        if (allVLANs.size === 0) return;

        // Create VLAN compound nodes
        const sortedVLANs = [...allVLANs].sort((a, b) => a - b);
        const vlanNodes = [];
        sortedVLANs.forEach(vid => {
            const label = vlanNames[vid] || `VLAN ${vid}`;
            vlanNodes.push({
                group: 'nodes',
                data: {
                    id: `vlan-${vid}`,
                    label: label,
                    vlanColor: getVLANColor(vid),
                },
                classes: 'vlan-group',
            });
        });
        cy.add(vlanNodes);

        // Move each device into its primary VLAN (exactly one)
        // Hide nodes that don't belong to any VLAN (switches, BMCs)
        cy.nodes().forEach(n => {
            if (n.hasClass('vlan-group')) return;
            const primaryVLAN = devicePrimaryVLAN[n.id()];
            if (primaryVLAN != null) {
                n.move({ parent: `vlan-${primaryVLAN}` });
                n.style('display', 'element');
            } else {
                n.style('display', 'none');
            }
        });
        // Hide edges where either endpoint is hidden
        cy.edges().forEach(e => {
            const srcHidden = e.source().style('display') === 'none';
            const tgtHidden = e.target().style('display') === 'none';
            e.style('display', (srcHidden || tgtHidden) ? 'none' : 'element');
        });

        vlanGrouped = true;
        grouped = true;

        // Use a manual column layout so VLAN regions don't overlap
        applyVLANColumnLayout(sortedVLANs);
    }

    function applyVLANColumnLayout(sortedVLANs) {
        if (!cy) return;

        const colWidth = 300;
        const nodeSep = 80;
        const topMargin = 60;
        const totalWidth = sortedVLANs.length * colWidth;
        const startX = -totalWidth / 2 + colWidth / 2;

        const positions = {};

        // Position children within each VLAN column
        sortedVLANs.forEach((vid, colIdx) => {
            const colX = startX + colIdx * colWidth;
            const parent = cy.getElementById(`vlan-${vid}`);
            if (!parent || parent.length === 0) return;

            const children = parent.children().sort((a, b) =>
                (a.data('label') || '').localeCompare(b.data('label') || '')
            );

            children.forEach((n, rowIdx) => {
                positions[n.id()] = {
                    x: colX,
                    y: topMargin + rowIdx * nodeSep,
                };
            });
        });

        // Animate all nodes to their positions
        cy.nodes().forEach(n => {
            if (n.isParent()) return;
            const pos = positions[n.id()];
            if (pos) n.animate({ position: pos, duration: 600, easing: 'ease-in-out-quad' });
        });

        setTimeout(() => cy.fit(50), 650);
    }

    function isVLANGrouped() {
        return vlanGrouped;
    }

    return {
        init,
        runLayout,
        fitToScreen,
        filterByType,
        searchNodes,
        exportPNG,
        toggleGroupByTOR,
        toggleGroupByVLAN,
        isGrouped,
        isVLANGrouped,
        getInstance,
        getCurrentLayout,
        updateElements,
    };
})();
