// core/utils.js — Shared utility functions

'use strict';

NM.core.escapeHtml = function(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
};

NM.core.showError = function(message) {
    const overlay = document.getElementById('error-overlay');
    const msgEl = document.getElementById('error-message');
    if (overlay) overlay.classList.remove('hidden');
    if (msgEl) msgEl.textContent = message;
};

// Warning banner with dismiss + expand/collapse
NM.core.showWarnings = (function() {
    let dismissedSignature = null;

    return function(failures) {
        const banner = document.getElementById('warning-banner');
        const summary = document.getElementById('warning-summary');
        const details = document.getElementById('warning-details');
        const toggle = document.getElementById('warning-toggle');

        if (!failures || failures.length === 0) {
            if (banner) banner.classList.add('hidden');
            return;
        }

        const sig = failures.map(f => f.switch + '|' + f.phase + '|' + f.message).sort().join('\n');
        if (sig === dismissedSignature) return;

        const groups = {};
        for (const f of failures) {
            const key = f.switch || 'unknown';
            if (!groups[key]) groups[key] = [];
            groups[key].push(f);
        }

        const switchNames = Object.keys(groups);
        const unreachable = switchNames.filter(s => groups[s].some(f => f.phase === 'connect'));
        const degraded = switchNames.filter(s => !groups[s].some(f => f.phase === 'connect'));

        const parts = [];
        if (unreachable.length > 0) parts.push(unreachable.length + ' switch' + (unreachable.length > 1 ? 'es' : '') + ' unreachable');
        if (degraded.length > 0) parts.push(degraded.length + ' switch' + (degraded.length > 1 ? 'es' : '') + ' with partial data');
        summary.textContent = parts.join(', ') + ' \u2014 topology may be incomplete';

        const sortedNames = [...unreachable, ...degraded];
        const esc = NM.core.escapeHtml;
        let html = '';
        for (const sw of sortedNames) {
            const isUnreachable = unreachable.includes(sw);
            const sevClass = isUnreachable ? 'unreachable' : 'degraded';
            const sevLabel = isUnreachable ? 'Unreachable' : 'Partial data';
            html += '<div class="warning-switch-group">';
            html += '<div class="warning-switch-name"><span class="severity-dot ' + sevClass + '" title="' + sevLabel + '"></span>' + esc(sw) + '</div>';
            for (const f of groups[sw]) {
                html += '<div class="warning-phase"><span class="phase-label">' + esc(f.phase) + ':</span> ' + esc(f.message) + '</div>';
            }
            html += '</div>';
        }
        details.innerHTML = html;

        toggle.onclick = () => {
            details.classList.toggle('hidden');
            toggle.textContent = details.classList.contains('hidden') ? '\u25BE' : '\u25B4';
        };
        document.getElementById('warning-dismiss').onclick = () => {
            dismissedSignature = sig;
            banner.classList.add('hidden');
            details.classList.add('hidden');
            toggle.textContent = '\u25BE';
        };
        banner.classList.remove('hidden');
    };
})();
