// popup.js — Floating card that appears near the clicked node

'use strict';

const Popup = (() => {
    const card = () => document.getElementById('popup-card');
    const titleEl = () => document.getElementById('popup-title');
    const bodyEl = () => document.getElementById('popup-body');
    const closeBtn = () => document.getElementById('popup-close');
    const detailsBtn = () => document.getElementById('popup-details-btn');

    let topology = null;
    let currentNodeData = null;
    let currentEdgeData = null;

    function init(topologyData) {
        topology = topologyData;

        // Prevent ALL pointer/mouse events on the popup card from
        // propagating — stops Cytoscape from seeing these as canvas events.
        const c = card();
        for (const evt of ['pointerdown', 'pointerup', 'mousedown', 'mouseup', 'click', 'touchstart', 'touchend']) {
            c.addEventListener(evt, (e) => e.stopPropagation());
        }

        closeBtn().addEventListener('click', hide);

        detailsBtn().addEventListener('click', (e) => {
            e.stopPropagation();
            e.preventDefault();
            const nodeData = currentNodeData;
            const edgeData = currentEdgeData;
            hide();
            requestAnimationFrame(() => {
                if (nodeData) {
                    Sidebar.showNode(nodeData);
                } else if (edgeData) {
                    Sidebar.showEdge(edgeData);
                }
            });
        });
    }

    function setTopology(topologyData) {
        topology = topologyData;
    }

    // Keep openDetails for potential future use, but primary handler is addEventListener above.
    function openDetails() {
        detailsBtn().click();
    }

    function showForNode(nodeData, renderedPosition) {
        currentNodeData = nodeData;
        currentEdgeData = null;

        const type = nodeData.type || 'unknown';
        titleEl().innerHTML = `<span class="type-badge ${type}">${type}</span> ${esc(nodeData.label || nodeData.id)}`;

        let html = '';

        // Key info rows
        if (nodeData.mgmt_addr) html += popupRow('IP', nodeData.mgmt_addr);
        if (nodeData.chassis_id) html += popupRow('Chassis', nodeData.chassis_id);
        if (nodeData.software_version) html += popupRow('Version', nodeData.software_version);
        if (nodeData.uptime) html += popupRow('Uptime', nodeData.uptime);
        if (nodeData.system_description) html += popupRow('OS', truncate(nodeData.system_description, 40));

        // Connection count
        if (topology && topology.links) {
            const count = topology.links.filter(
                (l) => l.local_device === nodeData.id || l.remote_device === nodeData.id
            ).length;
            html += popupRow('Links', count);
        }

        // Interface health summary
        if (nodeData.interfaces_up != null && nodeData.interfaces_total != null) {
            const upColor = nodeData.interfaces_up === nodeData.interfaces_total ? '#4CAF50' : '#FF9800';
            html += `<div class="popup-row">
                <span class="popup-label">Interfaces</span>
                <span class="popup-value" style="color:${upColor}">${nodeData.interfaces_up}/${nodeData.interfaces_total} up</span>
            </div>`;
        }

        // Annotations preview
        const annotations = nodeData.annotations || {};
        const annotationKeys = Object.keys(annotations);
        if (annotationKeys.length > 0) {
            const previewCount = Math.min(annotationKeys.length, 2);
            for (let i = 0; i < previewCount; i++) {
                html += popupRow(annotationKeys[i], truncate(String(annotations[annotationKeys[i]]), 30));
            }
            if (annotationKeys.length > previewCount) {
                html += `<div class="popup-row popup-more">+${annotationKeys.length - previewCount} more</div>`;
            }
        }

        bodyEl().innerHTML = html;
        positionCard(renderedPosition);
        card().classList.remove('hidden');
    }

    function showForEdge(edgeData, renderedPosition) {
        currentNodeData = null;
        currentEdgeData = edgeData;

        titleEl().innerHTML = '🔗 Link';

        let html = '';
        html += popupRow('From', `${esc(edgeData.source)} : ${esc(edgeData.local_port || '?')}`);
        html += popupRow('To', `${esc(edgeData.target)} : ${esc(edgeData.remote_port || '?')}`);

        if (edgeData.oper_status) {
            const color = edgeData.oper_status === 'UP' ? '#4CAF50' : '#e94560';
            html += `<div class="popup-row">
                <span class="popup-label">Status</span>
                <span class="popup-value"><span class="status-dot" style="background:${color}"></span> ${esc(edgeData.oper_status)}</span>
            </div>`;
        }
        if (edgeData.speed) html += popupRow('Speed', edgeData.speed);
        if (edgeData.mtu) html += popupRow('MTU', edgeData.mtu);

        bodyEl().innerHTML = html;
        positionCard(renderedPosition);
        card().classList.remove('hidden');
    }

    function positionCard(renderedPos) {
        const el = card();
        const cyContainer = document.getElementById('cy');
        const rect = cyContainer.getBoundingClientRect();

        // Position card to the right of the node
        let x = rect.left + renderedPos.x + 30;
        let y = rect.top + renderedPos.y - 40;

        // Keep within viewport
        const cardWidth = 280;
        const cardHeight = 300;
        if (x + cardWidth > window.innerWidth) {
            x = rect.left + renderedPos.x - cardWidth - 30;
        }
        if (y + cardHeight > window.innerHeight) {
            y = window.innerHeight - cardHeight - 16;
        }
        if (y < 60) y = 60;

        el.style.left = x + 'px';
        el.style.top = y + 'px';
    }

    function hide() {
        card().classList.add('hidden');
        currentNodeData = null;
        currentEdgeData = null;
    }

    function isVisible() {
        return !card().classList.contains('hidden');
    }

    function popupRow(label, value) {
        return `<div class="popup-row">
            <span class="popup-label">${esc(String(label))}</span>
            <span class="popup-value">${esc(String(value))}</span>
        </div>`;
    }

    function truncate(str, max) {
        return str.length > max ? str.substring(0, max) + '…' : str;
    }

    function esc(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    return { init, showForNode, showForEdge, hide, isVisible, openDetails, setTopology };
})();
