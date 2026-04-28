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

    // Official Microsoft Fluent UI System Icons (MIT licensed)
    // Source: https://github.com/microsoft/fluentui-system-icons
    // Recolored per device type using our Azure-inspired palette
    function fluentIcon(svgPath, color) {
        return 'data:image/svg+xml,' + encodeURIComponent(
            '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none">' +
            '<path d="' + svgPath + '" fill="' + color + '"/></svg>'
        );
    }
    const fluentPaths = {
        // ic_fluent_router_20_regular
        router: 'M3.5 9.5C3.5 5.91015 6.41015 3 10 3C13.5899 3 16.5 5.91015 16.5 9.5C16.5 9.77614 16.7239 10 17 10C17.2761 10 17.5 9.77614 17.5 9.5C17.5 5.35786 14.1421 2 10 2C5.85786 2 2.5 5.35786 2.5 9.5C2.5 9.77614 2.72386 10 3 10C3.27614 10 3.5 9.77614 3.5 9.5ZM10 5.5C7.79086 5.5 6 7.29086 6 9.5C6 9.77614 5.77614 10 5.5 10C5.22386 10 5 9.77614 5 9.5C5 6.73858 7.23858 4.5 10 4.5C12.7614 4.5 15 6.73858 15 9.5C15 9.77614 14.7761 10 14.5 10C14.2239 10 14 9.77614 14 9.5C14 7.29086 12.2091 5.5 10 5.5ZM7.75 9.25C7.75 8.00736 8.75736 7 10 7C11.2426 7 12.25 8.00736 12.25 9.25C12.25 10.3208 11.502 11.2169 10.5 11.4442V13H14.5C15.8807 13 17 14.1193 17 15.5C17 16.8807 15.8807 18 14.5 18H5.5C4.11929 18 3 16.8807 3 15.5C3 14.1193 4.11929 13 5.5 13H9.5V11.4442C8.49801 11.2169 7.75 10.3208 7.75 9.25ZM10 8C9.30964 8 8.75 8.55964 8.75 9.25C8.75 9.94036 9.30964 10.5 10 10.5C10.6904 10.5 11.25 9.94036 11.25 9.25C11.25 8.55964 10.6904 8 10 8ZM5.5 14C4.67157 14 4 14.6716 4 15.5C4 16.3284 4.67157 17 5.5 17H14.5C15.3284 17 16 16.3284 16 15.5C16 14.6716 15.3284 14 14.5 14H5.5Z',
        // ic_fluent_server_20_filled
        server: 'M7.5 2C6.11929 2 5 3.11929 5 4.5V15.5C5 16.8807 6.11929 18 7.5 18H12.5C13.8807 18 15 16.8807 15 15.5V4.5C15 3.11929 13.8807 2 12.5 2H7.5ZM7 5.5C7 5.22386 7.22386 5 7.5 5H12.5C12.7761 5 13 5.22386 13 5.5C13 5.77614 12.7761 6 12.5 6H7.5C7.22386 6 7 5.77614 7 5.5ZM7 12.5C7 12.2239 7.22386 12 7.5 12H12.5C12.7761 12 13 12.2239 13 12.5C13 12.7761 12.7761 13 12.5 13H7.5C7.22386 13 7 12.7761 7 12.5ZM7 14.5C7 14.2239 7.22386 14 7.5 14H12.5C12.7761 14 13 14.2239 13 14.5C13 14.7761 12.7761 15 12.5 15H7.5C7.22386 15 7 14.7761 7 14.5Z',
        // ic_fluent_desktop_20_regular
        desktop: 'M4 2C2.89543 2 2 2.89543 2 4V13C2 14.1046 2.89543 15 4 15H7V17H5.5C5.22386 17 5 17.2239 5 17.5C5 17.7761 5.22386 18 5.5 18H14.5C14.7761 18 15 17.7761 15 17.5C15 17.2239 14.7761 17 14.5 17H13V15H16C17.1046 15 18 14.1046 18 13V4C18 2.89543 17.1046 2 16 2H4ZM12 15V17H8V15H12ZM3 4C3 3.44772 3.44772 3 4 3H16C16.5523 3 17 3.44772 17 4V13C17 13.5523 16.5523 14 16 14H4C3.44772 14 3 13.5523 3 13V4Z',
        // ic_fluent_developer_board_20_regular
        developerBoard: 'M10 7C8.34315 7 7 8.34315 7 10C7 11.6569 8.34315 13 10 13C11.6569 13 13 11.6569 13 10C13 8.34315 11.6569 7 10 7ZM8 10C8 8.89543 8.89543 8 10 8C11.1046 8 12 8.89543 12 10C12 11.1046 11.1046 12 10 12C8.89543 12 8 11.1046 8 10ZM7.5 2C7.77614 2 8 2.22386 8 2.5V4H9.5V2.5C9.5 2.22386 9.72386 2 10 2C10.2761 2 10.5 2.22386 10.5 2.5V4H12V2.5C12 2.22386 12.2239 2 12.5 2C12.7761 2 13 2.22386 13 2.5V4H13.5C14.8807 4 16 5.11929 16 6.5V7H17.5C17.7761 7 18 7.22386 18 7.5C18 7.77614 17.7761 8 17.5 8H16V9.5H17.5C17.7761 9.5 18 9.72386 18 10C18 10.2761 17.7761 10.5 17.5 10.5H16V12H17.5C17.7761 12 18 12.2239 18 12.5C18 12.7761 17.7761 13 17.5 13H16V13.5C16 14.8807 14.8807 16 13.5 16H13V17.5C13 17.7761 12.7761 18 12.5 18C12.2239 18 12 17.7761 12 17.5V16H10.5V17.5C10.5 17.7761 10.2761 18 10 18C9.72386 18 9.5 17.7761 9.5 17.5V16H8V17.5C8 17.7761 7.77614 18 7.5 18C7.22386 18 7 17.7761 7 17.5V16H6.5C5.11929 16 4 14.8807 4 13.5V13H2.5C2.22386 13 2 12.7761 2 12.5C2 12.2239 2.22386 12 2.5 12H4V10.5H2.5C2.22386 10.5 2 10.2761 2 10C2 9.72386 2.22386 9.5 2.5 9.5H4V8H2.5C2.22386 8 2 7.77614 2 7.5C2 7.22386 2.22386 7 2.5 7H4V6.5C4 5.11929 5.11929 4 6.5 4H7V2.5C7 2.22386 7.22386 2 7.5 2ZM15 6.5C15 5.67157 14.3284 5 13.5 5H6.5C5.67157 5 5 5.67157 5 6.5V13.5C5 14.3284 5.67157 15 6.5 15H13.5C14.3284 15 15 14.3284 15 13.5V6.5Z',
    };
    const typeIcons = {
        switch: fluentIcon(fluentPaths.router, typeColors.switch),
        host:   fluentIcon(fluentPaths.server, typeColors.host),
        vm:     fluentIcon(fluentPaths.desktop, typeColors.vm),
        bmc:    fluentIcon(fluentPaths.developerBoard, typeColors.bmc),
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
