// timeline.js — Timeline slider for historical topology snapshots

'use strict';

NM.ui.Timeline = (() => {
    let snapshots = [];
    let isLive = true;
    let sliderEl, labelEl, liveBtn, bannerEl;

    function init() {
        const bar = document.getElementById('timeline-bar');
        if (!bar) return;

        sliderEl = document.getElementById('timeline-slider');
        labelEl = document.getElementById('timeline-label');
        liveBtn = document.getElementById('timeline-live-btn');
        bannerEl = document.getElementById('historical-banner');

        sliderEl.addEventListener('input', onSliderChange);
        liveBtn.addEventListener('click', returnToLive);

        var returnBtn = document.getElementById('historical-return-btn');
        if (returnBtn) {
            returnBtn.addEventListener('click', returnToLive);
        }

        // Fetch initial snapshot list
        fetchSnapshots();
    }

    async function fetchSnapshots() {
        try {
            const resp = await fetch('/api/snapshots');
            if (!resp.ok) return;
            const list = await resp.json();
            if (!Array.isArray(list)) return;
            updateSnapshots(list);
        } catch (e) {
            // Snapshots not available (static mode)
        }
    }

    function updateSnapshots(list) {
        snapshots = list.sort(function (a, b) {
            return new Date(a.timestamp) - new Date(b.timestamp);
        });

        const bar = document.getElementById('timeline-bar');
        if (!bar) return;

        if (snapshots.length < 2) {
            bar.classList.add('hidden');
            return;
        }

        bar.classList.remove('hidden');
        sliderEl.min = 0;
        sliderEl.max = snapshots.length; // extra position for "Live"
        sliderEl.value = snapshots.length; // default to live
        updateLabel(snapshots.length);
    }

    function onSliderChange() {
        var idx = parseInt(sliderEl.value, 10);
        updateLabel(idx);

        if (idx >= snapshots.length) {
            returnToLive();
        } else {
            loadSnapshot(idx);
        }
    }

    function updateLabel(idx) {
        if (idx >= snapshots.length) {
            labelEl.textContent = '● Live';
            labelEl.classList.add('live');
        } else {
            var ts = new Date(snapshots[idx].timestamp);
            labelEl.textContent = formatTimestamp(ts);
            labelEl.classList.remove('live');
        }
    }

    function formatTimestamp(date) {
        var month = String(date.getMonth() + 1).padStart(2, '0');
        var day = String(date.getDate()).padStart(2, '0');
        var hours = String(date.getHours()).padStart(2, '0');
        var minutes = String(date.getMinutes()).padStart(2, '0');
        var seconds = String(date.getSeconds()).padStart(2, '0');
        return date.getFullYear() + '-' + month + '-' + day + ' ' + hours + ':' + minutes + ':' + seconds;
    }

    async function loadSnapshot(idx) {
        var snap = snapshots[idx];
        if (!snap) return;

        isLive = false;
        showHistoricalBanner(snap.timestamp);

        try {
            var resp = await fetch('/api/snapshots/' + snap.filename.replace('topology-', '').replace('.json', ''));
            if (!resp.ok) return;
            var topo = await resp.json();
            applyTopology(topo);
        } catch (e) {
            console.error('Failed to load snapshot:', e);
        }
    }

    function returnToLive() {
        isLive = true;
        hideHistoricalBanner();
        sliderEl.value = snapshots.length;
        updateLabel(snapshots.length);

        // Re-fetch live topology
        fetch('/api/topology')
            .then(function (r) { return r.json(); })
            .then(function (topo) { applyTopology(topo); })
            .catch(function (e) { console.error('Failed to load live topology:', e); });
    }

    function applyTopology(topo) {
        var adapted = NM.data.adaptV2(topo);
        NM.state.topology = adapted;
        NM.state.rawTopology = topo;
        NM.core.showWarnings(adapted.partial_failures);
        NM.ui.Sidebar.setTopology(adapted);
        NM.ui.Popup.setTopology(adapted);
        NM.ui.Inventory.update();
        NM.graph.setTopology(adapted);
        NM.state.ViewManager.renderCurrentView();
    }

    function showHistoricalBanner(timestamp) {
        if (!bannerEl) return;
        var ts = new Date(timestamp);
        document.getElementById('historical-timestamp').textContent = formatTimestamp(ts);
        bannerEl.classList.remove('hidden');
    }

    function hideHistoricalBanner() {
        if (!bannerEl) return;
        bannerEl.classList.add('hidden');
    }

    // Called from WebSocket handler when snapshot_list updates arrive
    function onSnapshotListUpdate(list) {
        updateSnapshots(list);
        // If in live mode, keep slider at the end
        if (isLive && sliderEl) {
            sliderEl.value = snapshots.length;
            updateLabel(snapshots.length);
        }
    }

    function getIsLive() {
        return isLive;
    }

    return {
        init: init,
        onSnapshotListUpdate: onSnapshotListUpdate,
        isLive: getIsLive
    };
})();
