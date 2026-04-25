// graph.js — Cytoscape.js initialization, styling, and layout management

'use strict';

const NetworkGraph = (() => {
    let cy = null;
    let currentLayout = 'dagre';

    // Color map for device types
    const typeColors = {
        switch:  '#2196F3',
        host:    '#4CAF50',
        bmc:     '#FF9800',
        unknown: '#9E9E9E',
    };

    const typeShapes = {
        switch:  'round-rectangle',
        host:    'ellipse',
        bmc:     'diamond',
        unknown: 'ellipse',
    };

    // Cytoscape style definitions
    const graphStyle = [
        // Default node style
        {
            selector: 'node',
            style: {
                'label': 'data(label)',
                'width': 50,
                'height': 50,
                'font-size': '11px',
                'color': '#e6e6e6',
                'text-valign': 'bottom',
                'text-halign': 'center',
                'text-margin-y': 8,
                'text-outline-width': 2,
                'text-outline-color': '#0d1117',
                'background-color': '#9E9E9E',
                'border-width': 2,
                'border-color': 'rgba(255, 255, 255, 0.1)',
                'overlay-padding': 6,
                'transition-property': 'background-color, border-color, width, height, opacity',
                'transition-duration': '0.2s',
            },
        },
        // Switch nodes
        {
            selector: 'node[type="switch"]',
            style: {
                'background-color': typeColors.switch,
                'shape': typeShapes.switch,
                'width': 60,
                'height': 40,
                'border-color': 'rgba(33, 150, 243, 0.4)',
            },
        },
        // Host nodes
        {
            selector: 'node[type="host"]',
            style: {
                'background-color': typeColors.host,
                'shape': typeShapes.host,
                'border-color': 'rgba(76, 175, 80, 0.4)',
            },
        },
        // BMC nodes
        {
            selector: 'node[type="bmc"]',
            style: {
                'background-color': typeColors.bmc,
                'shape': typeShapes.bmc,
                'width': 35,
                'height': 35,
                'border-color': 'rgba(255, 152, 0, 0.4)',
            },
        },
        // Unknown nodes
        {
            selector: 'node[type="unknown"]',
            style: {
                'background-color': typeColors.unknown,
                'border-color': 'rgba(158, 158, 158, 0.3)',
            },
        },
        // Compound parent node style
        {
            selector: 'node:parent',
            style: {
                'background-color': 'rgba(42, 45, 62, 0.5)',
                'border-width': 1,
                'border-color': 'rgba(255, 255, 255, 0.08)',
                'border-style': 'dashed',
                'shape': 'round-rectangle',
                'padding': '30px',
                'label': 'data(label)',
                'font-size': '10px',
                'color': 'rgba(139, 143, 163, 0.7)',
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
                'border-color': '#e94560',
                'overlay-color': '#e94560',
                'overlay-opacity': 0.15,
            },
        },
        // Hovered node neighbor highlight
        {
            selector: 'node.highlight',
            style: {
                'border-width': 3,
                'border-color': '#e94560',
                'overlay-color': '#e94560',
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
                'width': 2,
                'line-color': '#3a3d52',
                'curve-style': 'bezier',
                'opacity': 0.7,
                'transition-property': 'line-color, width, opacity',
                'transition-duration': '0.2s',
            },
        },
        // Highlighted edges
        {
            selector: 'edge.highlight',
            style: {
                'line-color': '#e94560',
                'width': 3,
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
                'color': '#8b8fa3',
                'text-rotation': 'autorotate',
                'text-outline-width': 1,
                'text-outline-color': '#0d1117',
            },
        },
        // Search match
        {
            selector: 'node.search-match',
            style: {
                'border-width': 3,
                'border-color': '#FFD600',
                'overlay-color': '#FFD600',
                'overlay-opacity': 0.2,
            },
        },
        // Down edges (oper_status = DOWN)
        {
            selector: 'edge[oper_status="DOWN"]',
            style: {
                'line-color': '#e94560',
                'line-style': 'dashed',
                'opacity': 0.9,
            },
        },
    ];

    // Rank tiers: BMC on top (0), switches middle (1), hosts bottom (2)
    const typeRank = { bmc: 0, switch: 1, host: 2, unknown: 2 };

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
    const tierY = { 0: 80, 1: 80 + tierGap, 2: 80 + tierGap * 2 };

    function applyTieredLayout() {
        if (!cy) return;

        // Group nodes by tier
        const tiers = { 0: [], 1: [], 2: [] };
        cy.nodes().forEach((n) => {
            const rank = typeRank[n.data('type')] ?? 2;
            tiers[rank].push(n);
        });

        // Sort nodes within each tier by label for consistent ordering
        for (const rank of [0, 1, 2]) {
            tiers[rank].sort((a, b) => (a.data('label') || '').localeCompare(b.data('label') || ''));
        }

        // Calculate positions: center each tier horizontally
        const nodeSep = 120;
        const positions = {};

        for (const rank of [0, 1, 2]) {
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
        // Hover: highlight connected elements
        cy.on('mouseover', 'node', (evt) => {
            const node = evt.target;
            const neighborhood = node.neighborhood().add(node);

            cy.elements().addClass('dimmed');
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

    return {
        init,
        runLayout,
        fitToScreen,
        filterByType,
        searchNodes,
        exportPNG,
        toggleGroupByTOR,
        isGrouped,
        getInstance,
        getCurrentLayout,
        updateElements,
    };
})();
