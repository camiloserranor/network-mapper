// graph.js — Cytoscape.js initialization, styling, and layout management

'use strict';

const NetworkGraph = (() => {
    let cy = null;
    let currentLayout = 'dagre';
    let storedTopology = null;

    // Tree expand/collapse state
    const expandedNodes = new Set();
    // Maps parent ID → Set of child node IDs currently in the graph
    const expandedChildren = {};

    // Tier hierarchy: spine(0) > leaf(1) > host/bmc(2) > unknown(3) > vm(4)
    // Switches without a role default to tier 1 (leaf)
    const typeRank = { bmc: 2, switch: 1, host: 2, unknown: 3, vm: 4 };

    function getNodeTier(node) {
        const type = node.data('type');
        if (type === 'switch') {
            return node.data('role') === 'spine' ? 0 : 1;
        }
        return typeRank[type] ?? 3;
    }

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

    // Device type icons — official Microsoft Fluent UI System Icons (MIT licensed).
    // Source: https://github.com/microsoft/fluentui-system-icons
    //
    // Icons are stored as standalone SVG files in web/img/ rather than inlined as
    // data URIs in JavaScript. This keeps the JS clean and makes the icons easy to
    // update or swap independently. The Go server embeds the web/ directory via
    // go:embed, so /img/*.svg URLs are served same-origin with no CORS issues and
    // work in air-gapped environments. See ICON_DECISIONS.md for full rationale.
    const typeIcons = {
        switch: '/img/icon-switch.svg',   // ic_fluent_router_20_regular
        host:   '/img/icon-host.svg',     // ic_fluent_server_20_filled
        vm:     '/img/icon-vm.svg',       // ic_fluent_desktop_20_regular
        bmc:    '/img/icon-bmc.svg',      // ic_fluent_developer_board_20_regular
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
        // Unknown nodes — smaller and dimmed to reduce visual noise
        {
            selector: 'node[type="unknown"]',
            style: {
                'background-color': '#323130',
                'border-color': typeColors.unknown,
                'width': 28,
                'height': 28,
                'opacity': 0.4,
                'font-size': '9px',
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
        // Expandable node (has hidden children) — shows ▸ badge
        {
            selector: 'node.expandable',
            style: {
                'label': 'data(expandLabel)',
                'text-wrap': 'wrap',
                'text-max-width': '120px',
            },
        },
        // Expanded node — bright pulsing glow to clearly indicate expanded state
        {
            selector: 'node.expanded',
            style: {
                'label': 'data(expandLabel)',
                'text-wrap': 'wrap',
                'text-max-width': '120px',
                'border-width': 4,
                'border-color': '#3aafff',
                'border-opacity': 1,
                'overlay-color': '#3aafff',
                'overlay-opacity': 0.25,
                'overlay-padding': 12,
                'shadow-blur': 20,
                'shadow-color': '#3aafff',
                'shadow-offset-x': 0,
                'shadow-offset-y': 0,
                'shadow-opacity': 0.6,
            },
        },
    ];

    // (typeRank is defined at top of module)

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

    // Minimum horizontal space reserved for a single leaf node.
    const TREE_NODE_W  = 110;
    // Cell size for VM grid layout (VMs are shown in a compact grid under their host).
    const TREE_VM_CELL = 55;
    // Max columns in the VM grid — keeps the grid compact even for thousands of VMs.
    const TREE_VM_COLS = 12;
    // Vertical gap between a parent and its children tier.
    const TREE_TIER_H  = tierGap;
    // Horizontal gap between adjacent sibling subtrees.
    const TREE_GAP     = 30;

    /**
     * Tree-aware hierarchical layout.
     *
     * Positions every visible node using a classic tree drawing algorithm:
     *
     *   1. Identify root nodes — visible nodes not parented under any
     *      expanded node.  ALL roots share the same top row (y = 80)
     *      regardless of device tier; this centres spines at the tree
     *      apex.
     *   2. Sort roots: spine switches first, then leaf switches, then
     *      everything else — each sub-group alphabetical by label.
     *   3. Bottom-up: compute the horizontal width each subtree needs.
     *      Leaf nodes ➜ TREE_NODE_W.
     *      VMs ➜ compact grid  (TREE_VM_COLS × TREE_VM_CELL).
     *      Others ➜ sum of children widths + gaps.
     *   4. Top-down: each node is centred over its subtree.  Children
     *      are placed at parent_y + TREE_TIER_H.
     *   5. Animate all visible nodes to computed positions.
     *
     * Called after every expand / collapse / hierarchy-button click.
     */
    function layoutVisibleTree(fitAfter) {
        if (!cy) return;

        const visible = cy.nodes(':visible').filter(
            n => !n.isParent() && n.data('type') !== 'vlan-summary'
        );
        if (visible.length === 0) return;

        // --- 1. Find root nodes (not a child of any expanded node) ---
        const isChild = new Set();
        for (const children of Object.values(expandedChildren)) {
            if (children) for (const cid of children) isChild.add(cid);
        }
        const roots = [];
        visible.forEach(n => { if (!isChild.has(n.id())) roots.push(n); });

        // Sort: spines first, then other switches, then by label
        roots.sort((a, b) => {
            const ra = rootSortKey(a), rb = rootSortKey(b);
            if (ra !== rb) return ra - rb;
            return (a.data('label') || '').localeCompare(b.data('label') || '');
        });

        function rootSortKey(n) {
            if (n.data('type') === 'switch' && n.data('role') === 'spine') return 0;
            if (n.data('type') === 'switch') return 1;
            if (n.data('type') === 'host') return 2;
            return 3;
        }

        // --- 2. Bottom-up: compute subtree widths ---
        function subtreeWidth(nodeId) {
            const children = expandedChildren[nodeId];
            if (!children || children.size === 0) return TREE_NODE_W;

            const childArr = [...children];

            // VM children → grid layout width
            if (childArr[0] && childArr[0].startsWith('vm-')) {
                const cols = Math.min(childArr.length, TREE_VM_COLS);
                return Math.max(cols * TREE_VM_CELL, TREE_NODE_W);
            }

            // Non-VM children → sum of subtrees + gaps
            let total = 0;
            for (const cid of childArr) {
                total += subtreeWidth(cid) + TREE_GAP;
            }
            return Math.max(total - TREE_GAP, TREE_NODE_W);
        }

        // --- 3. Top-down: assign positions ---
        const positions = {};

        function positionSubtree(nodeId, cx, y) {
            positions[nodeId] = { x: cx, y };

            const children = expandedChildren[nodeId];
            if (!children || children.size === 0) return;

            const childArr = [...children];
            const childY = y + TREE_TIER_H;

            // VMs → compact grid centred under parent
            if (childArr[0] && childArr[0].startsWith('vm-')) {
                const cols = Math.min(childArr.length, TREE_VM_COLS);
                const gridW = (cols - 1) * TREE_VM_CELL;
                const sx = cx - gridW / 2;
                for (let i = 0; i < childArr.length; i++) {
                    positions[childArr[i]] = {
                        x: sx + (i % cols) * TREE_VM_CELL,
                        y: childY + Math.floor(i / cols) * TREE_VM_CELL,
                    };
                }
                return;
            }

            // Sort children for stable ordering
            childArr.sort((a, b) => {
                const na = cy.getElementById(a), nb = cy.getElementById(b);
                return (na.data('label') || '').localeCompare(nb.data('label') || '');
            });

            const totalW = childArr.reduce((s, c) => s + subtreeWidth(c) + TREE_GAP, 0) - TREE_GAP;
            let curX = cx - totalW / 2;

            for (const cid of childArr) {
                const w = subtreeWidth(cid);
                positionSubtree(cid, curX + w / 2, childY);
                curX += w + TREE_GAP;
            }
        }

        // --- 4. Lay out roots grouped by tier, each group centred ---
        //
        // Tier-0 roots (spines) get the top row. Their expanded children
        // land at tier-1 Y.  Independent tier-1 roots (leaf switches not
        // parented to an expanded spine) are placed in a SEPARATE band to
        // the right of any spine-subtree children at that level so they
        // never overlap.

        const ROOT_Y = 80;

        // Group roots by their sort key (spine=0, leaf=1, host=2, other=3)
        const rootGroups = {};
        for (const root of roots) {
            const k = rootSortKey(root);
            if (!rootGroups[k]) rootGroups[k] = [];
            rootGroups[k].push(root);
        }

        // Track the rightmost X occupied at each Y level by earlier groups
        let occupiedRight = -Infinity;

        const groupKeys = Object.keys(rootGroups).map(Number).sort();
        for (const gk of groupKeys) {
            const group = rootGroups[gk];
            const groupY = ROOT_Y + gk * TREE_TIER_H;

            const totalW = group.reduce(
                (s, r) => s + subtreeWidth(r.id()) + TREE_GAP, 0
            ) - TREE_GAP;

            // If a previous tier's subtrees already occupy space at this Y
            // (e.g. expanded spine children at tier-1 Y), offset this group
            // so it starts after the occupied region.
            let startX;
            if (occupiedRight > -Infinity) {
                // Centre the group, but push right if it would collide
                const centred = -totalW / 2;
                startX = Math.max(centred, occupiedRight + TREE_GAP * 2);
            } else {
                startX = -totalW / 2;
            }

            let curX = startX;
            for (const root of group) {
                const w = subtreeWidth(root.id());
                positionSubtree(root.id(), curX + w / 2, groupY);
                curX += w + TREE_GAP;
            }

            // Update occupied bounds from all positions set so far
            occupiedRight = -Infinity;
            for (const pos of Object.values(positions)) {
                if (pos.x > occupiedRight) occupiedRight = pos.x;
            }
            occupiedRight += TREE_NODE_W / 2;
        }

        // --- 5. Animate ---
        visible.forEach(n => {
            const pos = positions[n.id()];
            if (pos) n.animate({ position: pos, duration: 400, easing: 'ease-in-out-quad' });
        });

        if (fitAfter) {
            setTimeout(() => cy.fit(cy.elements(':visible'), 50), 450);
        }
    }

    // Called by runLayout('dagre') — always fit when user explicitly switches layout
    function applyTieredLayout() {
        layoutVisibleTree(true);
    }

    function init(containerId, elements, topology) {
        storedTopology = topology || null;
        expandedNodes.clear();

        cy = cytoscape({
            container: document.getElementById(containerId),
            elements: elements,
            style: graphStyle,
            layout: { name: 'preset' },
            minZoom: 0.1,
            maxZoom: 4,
            wheelSensitivity: 0.3,
        });

        setupInteraction();

        // Apply initial tree view: show only switches, hide everything else
        applyInitialTreeView();

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
            const config = { ...layouts[name], eles: cy.elements(':visible') };
            if (config.name) cy.layout(config).run();
        }
    }

    function fitToScreen() {
        if (cy) cy.fit(cy.elements(':visible'), 50);
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

    let grouped = false;

    function isGrouped() { return grouped; }

    function getInstance() {
        return cy;
    }

    function getCurrentLayout() {
        return currentLayout;
    }

    function updateElements(newElements) {
        if (!cy) return;

        // Build set of base elements (non-VM device nodes + edges)
        const newIds = new Set();
        newElements.forEach(e => newIds.add(e.data.id));

        // Collect IDs of dynamically-added VM nodes so we can protect their edges
        const vmNodeIds = new Set();
        cy.nodes().forEach(n => {
            if (n.data('type') === 'vm') vmNodeIds.add(n.id());
        });

        cy.batch(() => {
            // Remove elements that no longer exist, but skip:
            // - VM nodes (managed by expand/collapse)
            // - Edges connected to VM nodes (dynamically created)
            // - VLAN summary/group nodes
            cy.elements().forEach(e => {
                if (e.data('type') === 'vm') return;
                if (e.data('type') === 'vlan-summary') return;
                if (e.hasClass('vlan-group')) return;
                // Protect edges that connect to VM nodes
                if (e.isEdge()) {
                    if (vmNodeIds.has(e.source().id()) || vmNodeIds.has(e.target().id())) return;
                }
                if (!newIds.has(e.id())) {
                    e.remove();
                }
            });

            // Add or update elements — preserve current display state
            for (const el of newElements) {
                const existing = cy.getElementById(el.data.id);
                if (existing.length > 0) {
                    // Preserve the expand badge — incoming data has the plain label
                    const savedExpandLabel = existing.data('expandLabel');
                    existing.data(el.data);
                    if (savedExpandLabel) existing.data('expandLabel', savedExpandLabel);
                    // Refresh child counts that may have changed
                    if (existing.isNode()) updateExpandLabel(existing);
                } else {
                    cy.add(el);
                    const added = cy.getElementById(el.data.id);
                    if (added.length > 0 && added.isNode()) {
                        const type = added.data('type');
                        if (type !== 'switch') {
                            added.style('display', 'none');
                        }
                        updateExpandLabel(added);
                    }
                }
            }
        });
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
    let vlanSummaryNodes = []; // IDs of summary nodes added during VLAN view

    function toggleGroupByVLAN(topology) {
        if (!cy || !topology) return;

        if (vlanGrouped) {
            // Remove summary nodes and restore tree view
            cy.batch(() => {
                for (const id of vlanSummaryNodes) {
                    const n = cy.getElementById(id);
                    if (n.length) { n.connectedEdges().remove(); n.remove(); }
                }
                cy.nodes('.vlan-group').remove();
            });
            vlanSummaryNodes = [];
            vlanGrouped = false;

            // Restore tree view
            applyInitialTreeView();
            const toReExpand = [...expandedNodes];
            expandedNodes.clear();
            for (const id of toReExpand) expandNode(id);
            runLayout(currentLayout);
            return;
        }

        // Collect VLAN info
        const allVLANs = new Set();
        const vlanNames = {};
        if (topology.vlans) {
            topology.vlans.forEach(v => { allVLANs.add(v.id); vlanNames[v.id] = v.name || `VLAN ${v.id}`; });
        }

        // Count devices per VLAN per type
        const vlanCounts = {}; // vid → { switch: N, host: N, vm: N, ... }
        function addCount(vid, type) {
            allVLANs.add(vid);
            if (!vlanCounts[vid]) vlanCounts[vid] = {};
            vlanCounts[vid][type] = (vlanCounts[vid][type] || 0) + 1;
        }
        for (const d of (topology.devices || [])) {
            if (d.vlans && d.vlans.length > 0) addCount(d.vlans[0], d.type || 'unknown');
        }
        for (const ep of (topology.endpoints || [])) {
            if (ep.vlans && ep.vlans.length > 0) addCount(ep.vlans[0], 'vm');
        }

        if (allVLANs.size === 0) return;

        const sortedVLANs = [...allVLANs].sort((a, b) => a - b);

        // Hide all real nodes/edges
        cy.batch(() => {
            cy.nodes().style('display', 'none');
            cy.edges().style('display', 'none');
        });

        // Create VLAN compound nodes + summary child nodes
        const toAdd = [];
        vlanSummaryNodes = [];
        sortedVLANs.forEach(vid => {
            const label = vlanNames[vid] || `VLAN ${vid}`;
            const counts = vlanCounts[vid] || {};
            const parts = [];
            if (counts.switch) parts.push(`${counts.switch} switches`);
            if (counts.host) parts.push(`${counts.host} hosts`);
            if (counts.vm) parts.push(`${counts.vm} VMs`);
            if (counts.bmc) parts.push(`${counts.bmc} BMCs`);
            if (counts.unknown) parts.push(`${counts.unknown} unknown`);
            const summaryText = parts.length > 0 ? parts.join('\n') : 'empty';

            toAdd.push({
                group: 'nodes',
                data: { id: `vlan-${vid}`, label: label, vlanColor: getVLANColor(vid) },
                classes: 'vlan-group',
            });
            const summaryId = `vlan-summary-${vid}`;
            vlanSummaryNodes.push(summaryId);
            toAdd.push({
                group: 'nodes',
                data: { id: summaryId, label: `${label}\n${summaryText}`, parent: `vlan-${vid}`, type: 'vlan-summary' },
            });
        });

        cy.batch(() => { cy.add(toAdd); });

        // Style summary nodes
        cy.style().selector('node[type="vlan-summary"]').style({
            'shape': 'round-rectangle',
            'width': 160,
            'height': 80,
            'background-color': '#2a2a3e',
            'border-width': 1,
            'border-color': '#444',
            'color': '#e0e0e0',
            'font-size': 11,
            'text-wrap': 'wrap',
            'text-valign': 'center',
            'text-halign': 'center',
            'label': 'data(label)',
        }).update();

        vlanGrouped = true;

        // Column layout for VLAN groups
        const colWidth = 220;
        const totalWidth = sortedVLANs.length * colWidth;
        const startX = -totalWidth / 2 + colWidth / 2;
        sortedVLANs.forEach((vid, i) => {
            const n = cy.getElementById(`vlan-summary-${vid}`);
            if (n.length) n.position({ x: startX + i * colWidth, y: 80 });
        });
        setTimeout(() => cy.fit(50), 100);
    }

    function isVLANGrouped() {
        return vlanGrouped;
    }

    function setTopology(topology) {
        storedTopology = topology;
    }

    // ---- Tree expand/collapse ----

    // Show all switches initially; mark expandable nodes with child count badges.
    // Spine switches (tier 0) connect to leaf switches (tier 1) in the hierarchy.
    function applyInitialTreeView() {
        if (!cy) return;

        cy.batch(() => {
            cy.nodes().forEach(n => {
                if (n.data('type') === 'switch') {
                    n.style('display', 'element');
                    updateExpandLabel(n);
                } else {
                    n.style('display', 'none');
                }
            });
            // Show only switch↔switch edges
            cy.edges().forEach(e => {
                const srcType = e.source().data('type');
                const tgtType = e.target().data('type');
                if (srcType === 'switch' && tgtType === 'switch') {
                    e.style('display', 'element');
                } else {
                    e.style('display', 'none');
                }
            });
        });
    }

    // Update a node's label to show expand/collapse badge with child count
    function updateExpandLabel(node) {
        const childCount = node.data('childCount') || 0;
        const vmCount = node.data('vmCount') || 0;
        const baseName = node.data('system_name') || node.data('label') || node.id();
        const isExpanded = expandedNodes.has(node.id());

        let badge = '';
        const type = node.data('type');
        const role = node.data('role');

        if (type === 'switch' && role === 'spine' && childCount > 0) {
            badge = isExpanded ? `\n▾ ${childCount} leaf switches` : `\n▸ ${childCount} leaf switches`;
        } else if (type === 'switch' && childCount > 0) {
            badge = isExpanded ? `\n▾ ${childCount} connected` : `\n▸ ${childCount} connected`;
        } else if (type === 'host' && vmCount > 0) {
            badge = isExpanded ? `\n▾ ${vmCount} VMs` : `\n▸ ${vmCount} VMs`;
        } else if (childCount > 0) {
            badge = isExpanded ? `\n▾ ${childCount}` : `\n▸ ${childCount}`;
        }

        node.data('expandLabel', baseName + badge);

        if (childCount > 0 || vmCount > 0) {
            if (isExpanded) {
                node.removeClass('expandable').addClass('expanded');
            } else {
                node.removeClass('expanded').addClass('expandable');
            }
        }
    }

    // Get the direct children IDs of a node from topology links
    function getChildIds(nodeId) {
        if (!storedTopology) return [];

        const nodeType = getDeviceType(nodeId);
        const nodeRole = getDeviceRole(nodeId);
        const nodeEffectiveRank = (nodeType === 'switch' && nodeRole === 'spine') ? 0 : (typeRank[nodeType] ?? 3);
        const children = [];

        // Check LLDP links for device-to-device children
        for (const link of (storedTopology.links || [])) {
            let childId = null;
            if (link.local_device === nodeId) {
                childId = link.remote_device;
            } else if (link.remote_device === nodeId) {
                childId = link.local_device;
            }
            if (!childId) continue;

            const childType = getDeviceType(childId);
            const childRole = getDeviceRole(childId);
            const childEffectiveRank = (childType === 'switch' && childRole === 'spine') ? 0 : (typeRank[childType] ?? 3);
            // Child must be in a lower tier (higher rank number)
            if (childEffectiveRank > nodeEffectiveRank) {
                children.push(childId);
            }
        }

        // For hosts, also include VMs from endpoints
        if (nodeType === 'host' && storedTopology.endpoints) {
            for (const ep of storedTopology.endpoints) {
                if (ep.host_device === nodeId) {
                    children.push('vm-' + ep.mac.replace(/:/g, ''));
                }
            }
        }

        return children;
    }

    // Look up device type from topology data
    function getDeviceType(deviceId) {
        if (!storedTopology) return 'unknown';
        if (deviceId.startsWith('vm-')) return 'vm';
        const device = (storedTopology.devices || []).find(d => d.id === deviceId);
        return device ? (device.type || 'unknown') : 'unknown';
    }

    // Look up switch role (spine/leaf) from the Cytoscape node data
    function getDeviceRole(deviceId) {
        if (!cy) return '';
        const node = cy.getElementById(deviceId);
        if (node && node.length) return node.data('role') || '';
        return '';
    }

    // Create a VM node element from endpoint data
    function createVMElement(ep) {
        const epId = 'vm-' + ep.mac.replace(/:/g, '');
        const label = (ep.ips && ep.ips.length > 0) ? ep.ips[0] : ep.mac;
        return {
            group: 'nodes',
            data: {
                id: epId,
                label: label,
                expandLabel: label,
                type: 'vm',
                mac: ep.mac,
                ips: ep.ips || [],
                vlans: ep.vlans || [],
                host_device: ep.host_device || '',
                host_port: ep.host_port || '',
                switch_id: ep.switch_id || '',
                childCount: 0,
                vmCount: 0,
            },
        };
    }

    // Create a VM→host edge element
    function createVMEdge(ep) {
        const epId = 'vm-' + ep.mac.replace(/:/g, '');
        return {
            group: 'edges',
            data: {
                id: `${ep.host_device}::vm::${epId}`,
                source: ep.host_device,
                target: epId,
                source_type: 'mac-table',
                edgeLabel: ep.host_port || '',
                oper_status: 'UP',
            },
        };
    }

    function toggleExpand(nodeId) {
        if (expandedNodes.has(nodeId)) {
            collapseNode(nodeId);
        } else {
            expandNode(nodeId);
        }
    }

    function expandNode(nodeId) {
        if (!cy || !storedTopology) return;
        if (expandedNodes.has(nodeId)) return;

        const node = cy.getElementById(nodeId);
        if (node.length === 0) return;

        expandedNodes.add(nodeId);
        const childIds = getChildIds(nodeId);
        if (childIds.length === 0) {
            updateExpandLabel(node);
            return;
        }

        cy.batch(() => {
            const addedIds = new Set();
            const nodeType = getDeviceType(nodeId);

            for (const childId of childIds) {
                const existing = cy.getElementById(childId);
                if (existing.length > 0) {
                    // Node already exists (just hidden), show it
                    existing.style('display', 'element');
                    addedIds.add(childId);
                } else if (childId.startsWith('vm-')) {
                    // VM nodes are created on-demand
                    const ep = (storedTopology.endpoints || []).find(
                        e => ('vm-' + e.mac.replace(/:/g, '')) === childId
                    );
                    if (ep) {
                        cy.add(createVMElement(ep));
                        cy.add(createVMEdge(ep));
                        addedIds.add(childId);
                    }
                }
            }

            // Show edges connecting the parent to its now-visible children
            cy.edges().forEach(e => {
                const src = e.source().id();
                const tgt = e.target().id();
                if ((src === nodeId && addedIds.has(tgt)) ||
                    (tgt === nodeId && addedIds.has(src))) {
                    e.style('display', 'element');
                }
            });

            // Update expand labels on newly shown children
            for (const childId of addedIds) {
                const child = cy.getElementById(childId);
                if (child.length > 0) {
                    updateExpandLabel(child);
                }
            }

            updateExpandLabel(node);
        });

        expandedChildren[nodeId] = new Set(childIds);

        // Layout depends on active layout mode
        if (currentLayout === 'dagre') {
            layoutVisibleTree();
        } else {
            // For force / other layouts: place children near parent, then re-run
            const parentPos = node.position();
            for (const childId of childIds) {
                const child = cy.getElementById(childId);
                if (child.length > 0 && child.style('display') !== 'none') {
                    child.position({
                        x: parentPos.x + (Math.random() - 0.5) * 200,
                        y: parentPos.y + 100 + Math.random() * 150,
                    });
                }
            }
            const cfg = { ...layouts[currentLayout], eles: cy.elements(':visible') };
            if (cfg.name) cy.layout(cfg).run();
        }
    }

    function collapseNode(nodeId) {
        if (!cy) return;
        if (!expandedNodes.has(nodeId)) return;

        expandedNodes.delete(nodeId);
        const childIds = expandedChildren[nodeId] || new Set();

        cy.batch(() => {
            // Recursively collapse children first
            for (const childId of childIds) {
                if (expandedNodes.has(childId)) {
                    collapseNode(childId);
                }
            }

            for (const childId of childIds) {
                const child = cy.getElementById(childId);
                if (child.length === 0) continue;

                if (child.data('type') === 'vm') {
                    // Remove VM nodes entirely to free memory
                    child.connectedEdges().remove();
                    child.remove();
                } else {
                    // Hide device nodes and their edges
                    child.connectedEdges().style('display', 'none');
                    child.style('display', 'none');
                }
            }

            const node = cy.getElementById(nodeId);
            if (node.length > 0) {
                updateExpandLabel(node);
            }
        });

        delete expandedChildren[nodeId];

        // Re-layout depending on active mode
        if (currentLayout === 'dagre') {
            layoutVisibleTree();
        } else {
            const cfg = { ...layouts[currentLayout], eles: cy.elements(':visible') };
            if (cfg.name) cy.layout(cfg).run();
        }
    }

    // Collapse all nodes back to switches-only view
    function collapseAll() {
        if (!cy) return;
        const toCollapse = [...expandedNodes];
        // Collapse in reverse order (children before parents)
        toCollapse.reverse();
        for (const nodeId of toCollapse) {
            collapseNode(nodeId);
        }
    }

    return {
        init,
        runLayout,
        fitToScreen,
        searchNodes,
        exportPNG,
        exportJSON,
        toggleGroupByVLAN,
        isGrouped,
        isVLANGrouped,
        getInstance,
        getCurrentLayout,
        updateElements,
        setTopology,
        toggleExpand,
        expandNode,
        collapseNode,
        collapseAll,
    };
})();
