// Package reload provides a process-stable SSE hub for browser auto-reload
// during Air rebuilds. The app server POSTs /notify after listen succeeds;
// open browser tabs receive a reload event without depending on the app process.
package reload

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// DefaultAddr is the loopback address for the reload hub.
const DefaultAddr = "127.0.0.1:8199"

const clientJS = `(function () {
  var src = document.currentScript && document.currentScript.src;
  var base = src ? new URL(src).origin : "http://127.0.0.1:8199";
  var es = new EventSource(base + "/events");
  es.addEventListener("reload", function () {
    location.reload();
  });
})();
`

// Hub serves the reload client script, SSE events, and notify endpoint.
type Hub struct {
	addr string
	ln   net.Listener
	srv  *http.Server

	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

// Start listens on addr and serves the reload hub until Close.
func Start(addr string) (*Hub, error) {
	if addr == "" {
		addr = DefaultAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen reload hub on %s: %w", addr, err)
	}
	h := &Hub{
		addr:    ln.Addr().String(),
		ln:      ln,
		clients: make(map[chan []byte]struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dev-reload.js", h.serveScript)
	mux.HandleFunc("GET /events", h.serveEvents)
	mux.HandleFunc("POST /notify", h.serveNotify)
	h.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = h.srv.Serve(ln)
	}()
	return h, nil
}

// Close shuts down the hub listener and drops SSE clients.
func (h *Hub) Close() error {
	if h == nil || h.srv == nil {
		return nil
	}
	h.mu.Lock()
	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}
	h.mu.Unlock()
	return h.srv.Close()
}

// NotifyURL is the URL the app server should POST after listen succeeds.
func (h *Hub) NotifyURL() string {
	return "http://" + h.addr + "/notify"
}

// ScriptURL is the script src to inject into HTML during development.
func (h *Hub) ScriptURL() string {
	return "http://" + h.addr + "/dev-reload.js"
}

func (h *Hub) serveScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(clientJS))
}

func (h *Hub) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan []byte, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()

	_, _ = fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(msg); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Hub) serveNotify(w http.ResponseWriter, r *http.Request) {
	h.broadcast([]byte("event: reload\ndata: ok\n\n"))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Hub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		// Latest-wins: drop a queued event so a slow tab still gets the newest reload.
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- msg:
		default:
		}
	}
}
