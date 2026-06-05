// views/health.js — Network health dashboard view
// Displays network health findings: CRC errors, interface errors,
// PFC congestion, MTU mismatches, native VLAN mismatches.

'use strict';

NM.views.renderHealth = function() {
    var container = document.getElementById('detail-view');
    var esc = NM.core.escapeHtml;
    var topology = NM.state.topology;
    var telemetry = NM.state.telemetry;

    var findings = NM.data.computeHealthChecks(topology, telemetry);

    // Count by severity
    var counts = { critical: 0, warning: 0, info: 0 };
    for (var i = 0; i < findings.length; i++) {
        counts[findings[i].severity] = (counts[findings[i].severity] || 0) + 1;
    }

    var html = '';

    // Header
    html += '<div class="health-header">';
    html += '<h2 class="health-title">\u2665 Network Health</h2>';
    html += '<p class="health-subtitle">Physical link health and configuration consistency checks</p>';
    html += '</div>';

    // Summary cards
    html += '<div class="health-summary">';
    html += '<div class="health-card health-card-critical">';
    html += '<div class="health-card-count">' + counts.critical + '</div>';
    html += '<div class="health-card-label">Critical</div>';
    html += '</div>';
    html += '<div class="health-card health-card-warning">';
    html += '<div class="health-card-count">' + counts.warning + '</div>';
    html += '<div class="health-card-label">Warnings</div>';
    html += '</div>';
    html += '<div class="health-card health-card-info">';
    html += '<div class="health-card-count">' + counts.info + '</div>';
    html += '<div class="health-card-label">Info</div>';
    html += '</div>';
    html += '<div class="health-card health-card-ok">';
    html += '<div class="health-card-count">' + (findings.length === 0 ? '\u2713' : findings.length) + '</div>';
    html += '<div class="health-card-label">' + (findings.length === 0 ? 'All Clear' : 'Total') + '</div>';
    html += '</div>';
    html += '</div>';

    // Checks legend
    html += '<div class="health-checks-legend">';
    html += '<h3>Checks Performed</h3>';
    html += '<div class="health-legend-grid">';
    html += legendItem('CRC / Errors', 'Detects CRC errors and interface error counters');
    html += legendItem('PFC Congestion', 'PFC pause frames, ECN marking, watchdog drops, and RDMA queue losses');
    html += legendItem('MTU Consistency', 'Detects MTU mismatches between connected ports');
    html += legendItem('Native VLAN', 'Detects native VLAN mismatches on inter-switch trunks');
    html += '</div>';
    html += '</div>';

    // Findings list
    if (findings.length === 0) {
        html += '<div class="health-all-clear">';
        html += '<div class="health-all-clear-icon">\u2713</div>';
        html += '<div class="health-all-clear-text">No issues detected. All health checks passed.</div>';
        html += '</div>';
    } else {
        html += '<div class="health-findings">';
        html += '<h3>Findings (' + findings.length + ')</h3>';

        var categories = ['errors', 'discards', 'pfc', 'mtu', 'speed', 'vlan', 'link'];
        var catLabels = {
            errors: 'CRC / Interface Errors',
            discards: 'Packet Discards',
            pfc: 'PFC Congestion',
            mtu: 'MTU Mismatches',
            speed: 'Speed Mismatches',
            vlan: 'VLAN / Port Mode',
            link: 'Link Status'
        };
        var catIcons = { errors: '\u26a0', discards: '\u2193', pfc: '\u23f8', mtu: '\u21d4', speed: '\u26a1', vlan: '\u2630', link: '\u26d4' };

        for (var c = 0; c < categories.length; c++) {
            var cat = categories[c];
            var catFindings = findings.filter(function(f) { return f.category === cat; });
            if (catFindings.length === 0) continue;

            html += '<div class="health-category">';
            html += '<h4 class="health-category-title">' + catIcons[cat] + ' ' + esc(catLabels[cat]) + ' (' + catFindings.length + ')</h4>';

            for (var f = 0; f < catFindings.length; f++) {
                var finding = catFindings[f];
                html += '<div class="health-finding health-finding-' + finding.severity + '"';
                if (finding.deviceId) {
                    html += ' data-device-id="' + esc(finding.deviceId) + '"';
                    html += ' style="cursor:pointer"';
                }
                html += '>';
                html += '<span class="health-finding-badge badge-' + finding.severity + '">' + finding.severity.toUpperCase() + '</span>';
                html += '<span class="health-finding-title">' + esc(finding.title) + '</span>';
                html += '<div class="health-finding-detail">' + esc(finding.detail) + '</div>';
                html += '</div>';
            }

            html += '</div>';
        }

        html += '</div>';
    }

    container.innerHTML = html;

    // Wire click handlers to navigate to device
    container.querySelectorAll('.health-finding[data-device-id]').forEach(function(el) {
        el.addEventListener('click', function() {
            var devId = el.dataset.deviceId;
            var dev = (topology.devices || []).find(function(d) { return d.id === devId; });
            if (dev) {
                if (dev.type === 'switch') NM.state.ViewManager.navigateTo('switch', devId);
                else if (dev.type === 'host') NM.state.ViewManager.navigateTo('host', devId);
                else if (dev.type === 'bmc') NM.state.ViewManager.navigateTo('bmc', devId);
            }
        });
    });
};

function legendItem(title, desc) {
    return '<div class="health-legend-item"><strong>' + title + '</strong>: ' + desc + '</div>';
}

