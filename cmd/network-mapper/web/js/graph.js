// graph.js — Cytoscape.js renderer for network topology visualization

'use strict';

NM.graph = (() => {
    let cy = null;
    let currentLayoutMode = 'dagre'; // 'dagre' or 'cose'
    let storedTopology = null;

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

    // Device type icons (Fluent UI System Icons, MIT licensed)
    const typeIcons = {
        switch: '/img/icon-switch.svg',
        host:   '/img/icon-host.svg',
        vm:     '/img/icon-vm.svg',
        bmc:    '/img/icon-bmc.svg',
    };

    // Cytoscape style definitions
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
                'text-wrap': 'wrap',
                'text-max-width': '140px',
                'background-color': '#323130',
                'border-width': 1.5,
                'border-color': '#605e5c',
                'overlay-padding': 6,
                'transition-property': 'background-color, border-color, width, height, opacity',
                'transition-duration': '0.2s',
            },
        },
        // Compound switch parent node (the "switch box")
        {
            selector: 'node[type="switch-parent"]',
            style: {
                'shape': 'round-rectangle',
                'background-color': '#252423',
                'border-color': '#0078d4',
                'border-width': 2,
                'padding': '12px',
                'text-valign': 'top',
                'text-halign': 'center',
                'text-margin-y': -8,
                'font-size': '10px',
                'font-weight': '600',
                'color': '#ffffff',
                'min-width': '50px',
                'min-height': '40px',
            },
        },
        // Spine switch parent (bolder border)
        {
            selector: 'node[type="switch-parent"][role="spine"]',
            style: {
                'border-color': '#2899f5',
                'border-width': 2.5,
                'background-color': '#1e2a3a',
            },
        },
        // Port nodes (small dots inside switch)
        {
            selector: 'node[type="port"]',
            style: {
                'width': 12,
                'height': 12,
                'shape': 'ellipse',
                'label': '',
                'background-color': '#484644',
                'border-width': 1,
                'border-color': '#605e5c',
                'overlay-padding': 3,
            },
        },
        // Port connected to switch
        {
            selector: 'node[type="port"][connType="switch"]',
            style: {
                'background-color': '#0078d4',
                'border-color': '#2899f5',
            },
        },
        // Port connected to host
        {
            selector: 'node[type="port"][connType="host"]',
            style: {
                'background-color': '#44b700',
                'border-color': '#5ed600',
            },
        },
        // Port connected to BMC
        {
            selector: 'node[type="port"][connType="bmc"]',
            style: {
                'background-color': '#f7630c',
                'border-color': '#ff8c40',
            },
        },
        // Port connected to unknown
        {
            selector: 'node[type="port"][connType="unknown"]',
            style: {
                'background-color': '#8a8886',
                'border-color': '#a19f9d',
            },
        },
        // Port that is down
        {
            selector: 'node[type="port"][connType="down"]',
            style: {
                'background-color': '#323130',
                'border-color': '#484644',
                'opacity': 0.4,
            },
        },
        // Port hover highlight
        {
            selector: 'node[type="port"].highlight',
            style: {
                'width': 16,
                'height': 16,
                'border-width': 2,
                'border-color': '#ffffff',
                'overlay-opacity': 0.2,
            },
        },
        // Switch nodes (for legacy compatibility)
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
        // Host nodes
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
        // VM nodes
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
        // Dimmed nodes
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
                'line-color': '#0078d4',
                'curve-style': 'bezier',
                'opacity': 0.5,
                'transition-property': 'line-color, width, opacity',
                'transition-duration': '0.2s',
            },
        },
        // Switch-link edges (between ports)
        {
            selector: 'edge[type="switch-link"]',
            style: {
                'line-color': '#0078d4',
                'width': 2,
                'opacity': 0.6,
                'curve-style': 'bezier',
            },
        },
        // Highlighted edges
        {
            selector: 'edge.highlight',
            style: {
                'line-color': '#2899f5',
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
        // Down edges
        {
            selector: 'edge[operStatus="DOWN"]',
            style: {
                'line-color': '#d13438',
                'line-style': 'dashed',
                'opacity': 0.9,
            },
        },
    ];

    // Layout configurations
    const layouts = {
        preset: {
            name: 'preset',
            animate: false,
        },
        dagre: {
            name: 'dagre',
            animate: true,
            animationDuration: 400,
            rankDir: 'TB',
            nodeSep: 80,
            rankSep: 140,
            padding: 50,
        },
        'fabric-compound': {
            name: 'dagre',
            animate: true,
            animationDuration: 500,
            rankDir: 'TB',
            nodeSep: 30,
            rankSep: 100,
            padding: 20,
            // Sort: spine first (lower value = higher rank in dagre TB)
            sort: function(a, b) {
                const roleOrder = { spine: 0, leaf: 1, '': 2 };
                const ra = roleOrder[a.data('role') || ''] || 2;
                const rb = roleOrder[b.data('role') || ''] || 2;
                return ra - rb;
            },
        },
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
    };

    function init(containerId, elements, topology) {
        storedTopology = topology || null;

        if (cy) cy.destroy();

        cy = cytoscape({
            container: document.getElementById(containerId),
            elements: elements,
            style: graphStyle,
            layout: { name: 'preset' },
            minZoom: 0.1,
            maxZoom: 4,
            wheelSensitivity: 0.3,
            autoungrabify: true,
        });

        setupInteraction();
        return cy;
    }

    function render(elements, layoutName) {
        if (!cy) return;
        cy.batch(() => {
            cy.elements().remove();
            cy.add(elements);
        });
        runLayout(layoutName || currentLayoutMode);
    }

    function setupInteraction() {
        // Hover on edges: highlight
        cy.on('mouseover', 'edge', (evt) => {
            evt.target.addClass('highlight');
        });
        cy.on('mouseout', 'edge', (evt) => {
            evt.target.removeClass('highlight');
        });
    }

    function runLayout(name) {
        if (!cy) return;
        currentLayoutMode = name;
        const config = layouts[name] ? { ...layouts[name] } : { ...layouts['dagre'] };
        cy.layout(config).run();
    }

    function fitToScreen() {
        if (cy) cy.fit(cy.elements(), 50);
    }

    // Arrange port nodes vertically inside each switch parent (like a rack-mounted switch)
    function arrangePortsInRows() {
        if (!cy) return;
        const parents = cy.nodes('[type="switch-parent"]');
        parents.forEach((parent) => {
            const children = parent.children().sort((a, b) => {
                const aName = a.data('portName') || '';
                const bName = b.data('portName') || '';
                return aName.localeCompare(bName, undefined, { numeric: true });
            });
            const count = children.length;
            if (count === 0) return;

            const portSize = 12;
            const gap = 4;
            // Two columns, many rows (vertical orientation like a real switch)
            const cols = 2;
            const rows = Math.ceil(count / cols);
            const totalHeight = rows * (portSize + gap) - gap;
            const totalWidth = cols * (portSize + gap) - gap;
            const parentPos = parent.position();

            children.forEach((child, i) => {
                const col = i % cols;
                const row = Math.floor(i / cols);
                const x = parentPos.x - totalWidth / 2 + col * (portSize + gap) + portSize / 2;
                const y = parentPos.y - totalHeight / 2 + row * (portSize + gap) + portSize / 2;
                child.position({ x, y });
            });
        });
    }

    function searchNodes(query) {
        if (!cy) return [];
        cy.nodes().removeClass('search-match');
        if (!query || query.trim() === '') return [];

        const q = query.toLowerCase();
        const matches = [];
        cy.nodes().forEach((node) => {
            const label = (node.data('label') || '').toLowerCase();
            const id = (node.data('id') || '').toLowerCase();
            const chassisId = (node.data('chassis_id') || '').toLowerCase();
            if (label.includes(q) || id.includes(q) || chassisId.includes(q)) {
                node.addClass('search-match');
                matches.push(node.data());
            }
        });
        return matches;
    }

    function exportPNG() {
        if (!cy) return;
        const png = cy.png({ full: true, scale: 2, bg: '#1b1a19' });
        const link = document.createElement('a');
        link.href = png;
        link.download = 'network-topology.png';
        link.click();
    }

    function exportJSON() {
        if (!storedTopology) return;
        const json = JSON.stringify(storedTopology, null, 2);
        const blob = new Blob([json], { type: 'application/json' });
        const link = document.createElement('a');
        link.href = URL.createObjectURL(blob);
        link.download = 'network-topology.json';
        link.click();
        URL.revokeObjectURL(link.href);
    }

    function getInstance() { return cy; }
    function getCurrentLayout() { return currentLayoutMode; }
    function setTopology(topology) { storedTopology = topology; }

    return {
        init,
        render,
        runLayout,
        fitToScreen,
        arrangePortsInRows,
        searchNodes,
        exportPNG,
        exportJSON,
        getInstance,
        getCurrentLayout,
        setTopology,
    };
})();
