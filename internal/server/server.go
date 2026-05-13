package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/camiloserranor/network-mapper/internal/storage"
)

// Server serves the embedded web UI and the topology API.
type Server struct {
	topologyPath string
	port         int
	webFS        fs.FS
	live         bool // true when using streaming collector
	enablePprof  bool // enable /debug/pprof/* endpoints
	startTime    time.Time
	snapshots    *storage.SnapshotStore

	// Live mode state
	mu        sync.RWMutex
	liveTopo  interface{} // *topology.TopologyV2
	clients   map[*wsClient]struct{}
	clientsMu sync.Mutex
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// New creates a new Server. webFS should be the web/ directory filesystem.
func New(topologyPath string, port int, webFS fs.FS) *Server {
	return &Server{
		topologyPath: topologyPath,
		port:         port,
		webFS:        webFS,
		clients:      make(map[*wsClient]struct{}),
		startTime:    time.Now(),
	}
}

// SetPprof enables /debug/pprof/* endpoints.
func (s *Server) SetPprof(enable bool) {
	s.enablePprof = enable
}

// SetLiveMode puts the server into live mode with initial topology data.
func (s *Server) SetLiveMode(initial interface{}) {
	s.live = true
	s.liveTopo = initial
}

// SetSnapshots enables the snapshot history API.
func (s *Server) SetSnapshots(store *storage.SnapshotStore) {
	s.snapshots = store
}

// UpdateTopology updates the in-memory topology and broadcasts to all WebSocket clients.
func (s *Server) UpdateTopology(topo interface{}) {
	s.mu.Lock()
	s.liveTopo = topo
	s.mu.Unlock()

	data, err := json.Marshal(topo)
	if err != nil {
		log.Printf("[ws] Failed to marshal topology: %v", err)
		return
	}

	s.broadcast(data)
}

// Start begins listening and serving HTTP requests.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.FS(s.webFS)))

	mux.HandleFunc("/api/topology", s.handleTopology)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/ws", s.handleWebSocket)
	mux.HandleFunc("/api/snapshots", s.handleSnapshotList)
	mux.HandleFunc("/api/snapshots/", s.handleSnapshotLoad)

	if s.enablePprof {
		log.Println("[pprof] Profiling endpoints enabled at /debug/pprof/")
		mux.Handle("/debug/pprof/", http.DefaultServeMux)
	}

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if s.live {
		s.mu.RLock()
		topo := s.liveTopo
		s.mu.RUnlock()

		if topo == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "topology not yet collected",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(topo)
		return
	}

	// Static file mode
	data, err := os.ReadFile(s.topologyPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("topology file not found: %s", s.topologyPath),
		})
		return
	}

	if !json.Valid(data) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "topology file contains invalid JSON",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"status":    "ok",
		"live_mode": s.live,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if !s.live {
		resp["topology_file"] = s.topologyPath
	}

	s.clientsMu.Lock()
	resp["ws_clients"] = len(s.clients)
	s.clientsMu.Unlock()

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := map[string]interface{}{
		"uptime_seconds":    time.Since(s.startTime).Seconds(),
		"goroutines":        runtime.NumGoroutine(),
		"heap_alloc_bytes":  m.HeapAlloc,
		"heap_sys_bytes":    m.HeapSys,
		"heap_objects":      m.HeapObjects,
		"gc_cycles":         m.NumGC,
		"gc_pause_total_ns": m.PauseTotalNs,
		"num_cpu":           runtime.NumCPU(),
		"go_version":        runtime.Version(),
	}

	s.clientsMu.Lock()
	metrics["ws_clients"] = len(s.clients)
	s.clientsMu.Unlock()

	json.NewEncoder(w).Encode(metrics)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow any origin for local dev
	})
	if err != nil {
		log.Printf("[ws] Accept error: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 8),
	}

	s.clientsMu.Lock()
	s.clients[client] = struct{}{}
	clientCount := len(s.clients)
	s.clientsMu.Unlock()
	log.Printf("[ws] Client connected (%d total)", clientCount)

	// Send initial status message
	ctx := r.Context()
	wsjson.Write(ctx, conn, map[string]interface{}{
		"type":      "status",
		"live_mode": s.live,
		"message":   "connected",
	})

	// Send current topology immediately if available
	if s.live {
		s.mu.RLock()
		topo := s.liveTopo
		s.mu.RUnlock()
		if topo != nil {
			data, _ := json.Marshal(topo)
			client.send <- data
		}
	}

	// Writer goroutine
	go func() {
		defer conn.CloseNow()
		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					return
				}
				writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := conn.Write(writeCtx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					return
				}
			}
		}
	}()

	// Reader goroutine (keep connection alive, handle pings)
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	// Cleanup
	s.clientsMu.Lock()
	delete(s.clients, client)
	clientCount = len(s.clients)
	s.clientsMu.Unlock()
	close(client.send)
	log.Printf("[ws] Client disconnected (%d remaining)", clientCount)
}

func (s *Server) broadcast(data []byte) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for client := range s.clients {
		select {
		case client.send <- data:
		default:
			// Client too slow, disconnect
			log.Println("[ws] Dropping slow client")
			close(client.send)
			delete(s.clients, client)
		}
	}
}

func (s *Server) handleSnapshotList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.snapshots == nil {
		json.NewEncoder(w).Encode([]storage.SnapshotInfo{})
		return
	}

	list, err := s.snapshots.List()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleSnapshotLoad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.snapshots == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "snapshots not enabled"})
		return
	}

	// Extract timestamp from path: /api/snapshots/{timestamp}
	tsStr := strings.TrimPrefix(r.URL.Path, "/api/snapshots/")
	tsStr = strings.TrimSpace(tsStr)
	if tsStr == "" {
		// Redirect to list endpoint
		s.handleSnapshotList(w, r)
		return
	}

	ts, err := time.Parse(storage.TimestampFormat, tsStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid timestamp format, expected " + storage.TimestampFormat})
		return
	}

	topo, err := s.snapshots.Load(ts)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(topo)
}

// NotifySnapshotSaved broadcasts a snapshot list update to all WebSocket clients.
func (s *Server) NotifySnapshotSaved() {
	if s.snapshots == nil {
		return
	}

	list, err := s.snapshots.List()
	if err != nil {
		log.Printf("[ws] Failed to list snapshots for notification: %v", err)
		return
	}

	msg := map[string]interface{}{
		"type":      "snapshot_list",
		"snapshots": list,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.broadcast(data)
}

// OpenBrowser opens the default browser to the given URL.
func OpenBrowser(url string) {
	// Small delay to let the server start
	time.Sleep(500 * time.Millisecond)

	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		log.Printf("Could not open browser: %v", err)
	}
}
