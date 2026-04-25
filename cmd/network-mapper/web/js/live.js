// live.js — WebSocket client for real-time topology updates

'use strict';

const LiveConnection = (() => {
    let ws = null;
    let onTopologyUpdate = null;
    let reconnectTimer = null;
    let reconnectDelay = 1000;
    const MAX_RECONNECT_DELAY = 30000;

    function init(callback) {
        onTopologyUpdate = callback;
        connect();
    }

    function connect() {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = `${protocol}//${location.host}/api/ws`;

        try {
            ws = new WebSocket(url);
        } catch (err) {
            scheduleReconnect();
            return;
        }

        ws.onopen = () => {
            reconnectDelay = 1000;
            updateStatus('connected');
        };

        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);

                // Status message from server
                if (data.type === 'status') {
                    if (data.live_mode) {
                        updateStatus('live');
                    }
                    return;
                }

                // Topology update (has devices/links)
                if (data.devices || data.links) {
                    updateStatus('live');
                    if (onTopologyUpdate) {
                        onTopologyUpdate(data);
                    }
                }
            } catch (err) {
                console.warn('[live] Failed to parse message:', err);
            }
        };

        ws.onclose = () => {
            updateStatus('disconnected');
            scheduleReconnect();
        };

        ws.onerror = () => {
            updateStatus('disconnected');
        };
    }

    function scheduleReconnect() {
        if (reconnectTimer) return;
        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY);
            connect();
        }, reconnectDelay);
    }

    function updateStatus(state) {
        const indicator = document.getElementById('live-indicator');
        if (!indicator) return;

        indicator.className = 'live-indicator ' + state;

        switch (state) {
            case 'live':
                indicator.innerHTML = '<span class="live-dot"></span> Live';
                indicator.title = 'Receiving real-time topology updates';
                break;
            case 'connected':
                indicator.innerHTML = '<span class="live-dot"></span> Connected';
                indicator.title = 'WebSocket connected, waiting for updates';
                break;
            case 'disconnected':
                indicator.innerHTML = '<span class="live-dot"></span> Offline';
                indicator.title = 'Reconnecting...';
                break;
        }
    }

    return { init };
})();
