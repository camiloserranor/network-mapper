// views/vm.js — VM detail: info card view

'use strict';

NM.views.renderVM = function(vmId) {
    const topology = NM.state.topology;
    const vmData = NM.data.getVMData(vmId);
    const container = document.getElementById('detail-view');
    const esc = NM.core.escapeHtml;

    if (!vmData) {
        container.innerHTML = '<div class="device-header"><div class="device-header-info"><div class="device-header-name">VM not found</div></div></div>';
        return;
    }

    const label = (vmData.ips && vmData.ips.length > 0) ? vmData.ips[0] : vmData.mac;
    const hostDev = (topology.devices || []).find(d => d.id === vmData.host_device);
    const switchDev = vmData.switch_id ? (topology.devices || []).find(d => d.id === vmData.switch_id) : null;

    let html = '';

    // Device header
    html += '<div class="device-header">';
    html += '<div class="device-header-icon vm">\u2B22</div>';
    html += '<div class="device-header-info">';
    html += '<div class="device-header-name">' + esc(label) + '</div>';
    html += '<div class="device-header-meta">';
    html += '<span>Type: <strong>Virtual Machine</strong></span>';
    html += '</div></div></div>';

    // Network info
    html += '<div class="vm-info-card">';
    html += '<div class="vm-info-card-title">Network Information</div>';
    html += '<div class="vm-info-row"><span class="vm-info-label">MAC Address</span><span class="vm-info-value">' + esc(vmData.mac) + '</span></div>';
    if (vmData.ips && vmData.ips.length > 0) {
        html += '<div class="vm-info-row"><span class="vm-info-label">IP Address(es)</span><span class="vm-info-value">' + esc(vmData.ips.join(', ')) + '</span></div>';
    }
    if (vmData.vlans && vmData.vlans.length > 0) {
        html += '<div class="vm-info-row"><span class="vm-info-label">VLAN(s)</span><span class="vm-info-value">' + esc(vmData.vlans.join(', ')) + '</span></div>';
    }
    html += '</div>';

    // Location info
    html += '<div class="vm-info-card">';
    html += '<div class="vm-info-card-title">Location</div>';
    if (hostDev) {
        html += '<div class="vm-info-row"><span class="vm-info-label">Host</span><span class="vm-info-value"><span class="link" data-view="host" data-id="' + esc(hostDev.id) + '">' + esc(hostDev.system_name || hostDev.id) + '</span></span></div>';
    }
    if (vmData.host_port) {
        html += '<div class="vm-info-row"><span class="vm-info-label">Host Port</span><span class="vm-info-value">' + esc(vmData.host_port) + '</span></div>';
    }
    if (switchDev) {
        html += '<div class="vm-info-row"><span class="vm-info-label">Switch</span><span class="vm-info-value"><span class="link" data-view="switch" data-id="' + esc(switchDev.id) + '">' + esc(switchDev.system_name || switchDev.id) + '</span></span></div>';
    }
    html += '</div>';

    container.innerHTML = html;

    // Wire link clicks
    container.querySelectorAll('.link[data-view]').forEach(link => {
        link.addEventListener('click', () => {
            NM.state.ViewManager.navigateTo(link.dataset.view, link.dataset.id);
        });
    });
};
