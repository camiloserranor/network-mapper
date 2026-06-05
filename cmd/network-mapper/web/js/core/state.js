// core/state.js — Application state and ViewManager (navigation controller)

'use strict';

// Shared topology state
NM.state.topology = null;

// ViewManager — single responsibility: navigation state + breadcrumb rendering
NM.state.ViewManager = (function() {
    let currentView = { view: 'fabric', deviceId: null };

    function getCurrentView() { return currentView; }

    function navigateTo(view, deviceId) {
        currentView = { view, deviceId: deviceId || null };
        hideAllTooltips();
        updateBreadcrumb();
        renderCurrentView();
    }

    function navigateToFabric() {
        navigateTo('fabric', null);
    }

    function hideAllTooltips() {
        var ids = ['port-hover-tooltip', 'svg-port-tooltip'];
        for (var i = 0; i < ids.length; i++) {
            var el = document.getElementById(ids[i]);
            if (el) el.style.display = 'none';
        }
    }

    function updateBreadcrumb() {
        const trail = document.getElementById('breadcrumb-trail');
        if (!trail) return;

        const esc = NM.core.escapeHtml;
        const topology = NM.state.topology;
        let html = '<span class="crumb clickable" data-index="0">Fabric Overview</span>';

        if (currentView.view === 'switch') {
            const name = getDeviceName(currentView.deviceId);
            html += ' <span class="crumb-sep">\u203A</span> ';
            html += '<span class="crumb current">Switch: ' + esc(name) + '</span>';
        } else if (currentView.view === 'host') {
            const name = getDeviceName(currentView.deviceId);
            html += ' <span class="crumb-sep">\u203A</span> ';
            html += '<span class="crumb current">Host: ' + esc(name) + '</span>';
        } else if (currentView.view === 'bmc') {
            const name = getDeviceName(currentView.deviceId);
            html += ' <span class="crumb-sep">\u203A</span> ';
            html += '<span class="crumb current">BMC: ' + esc(name) + '</span>';
        } else if (currentView.view === 'vm') {
            const vmData = NM.data.getVMData(currentView.deviceId);
            if (vmData && vmData.host_device) {
                const hostDev = (topology.devices || []).find(d => d.id === vmData.host_device);
                if (hostDev) {
                    html += ' <span class="crumb-sep">\u203A</span> ';
                    html += '<span class="crumb clickable" data-view="host" data-id="' + esc(hostDev.id) + '">Host: ' + esc(hostDev.system_name || hostDev.id) + '</span>';
                }
            }
            html += ' <span class="crumb-sep">\u203A</span> ';
            const vmLabel = vmData ? ((vmData.ips && vmData.ips.length > 0) ? vmData.ips[0] : vmData.mac) : currentView.deviceId;
            html += '<span class="crumb current">VM: ' + esc(vmLabel) + '</span>';
        } else if (currentView.view === 'health') {
            html += ' <span class="crumb-sep">\u203A</span> ';
            html += '<span class="crumb current">\u2665 Network Health</span>';
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
                    navigateTo(view, id);
                }
            });
        });
    }

    function getDeviceName(deviceId) {
        const topology = NM.state.topology;
        if (!topology) return deviceId || '';
        const dev = (topology.devices || []).find(d => d.id === deviceId);
        return dev ? (dev.system_name || dev.id) : deviceId || '';
    }

    function renderCurrentView() {
        const topology = NM.state.topology;
        if (!topology) return;
        NM.ui.Popup.hide();
        NM.ui.Sidebar.hide();

        const cyEl = document.getElementById('cy');
        const detailEl = document.getElementById('detail-view');

        if (currentView.view === 'fabric') {
            cyEl.classList.remove('hidden');
            detailEl.classList.add('hidden');
            detailEl.innerHTML = '';
            NM.views.renderFabric();
        } else {
            cyEl.classList.add('hidden');
            detailEl.classList.remove('hidden');
            switch (currentView.view) {
                case 'switch': NM.views.renderSwitch(currentView.deviceId); break;
                case 'host':   NM.views.renderHost(currentView.deviceId); break;
                case 'bmc':    NM.views.renderBMC(currentView.deviceId); break;
                case 'vm':     NM.views.renderVM(currentView.deviceId); break;
                case 'health': NM.views.renderHealth(); break;
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
