package echo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/gofu/echo/chrome"
	"github.com/gorilla/websocket"
)

var (
	ErrWSOnly = errors.New("this endpoint requires a WebSocket upgrade request")
)

type Handler struct {
	Service  *Service
	o        sync.Once
	upgrader *websocket.Upgrader
	proxy    *httputil.ReverseProxy
}

func (h *Handler) init() {
	h.upgrader = &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	h.proxy = httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:9004",
	})
}

type Result struct {
	OK    bool        `json:"ok"`
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

func (h *Handler) serveError(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	switch err.(type) {
	default:
		log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
	}
}

func (h *Handler) serveHTML(w http.ResponseWriter, r *http.Request, tpl []byte) {
	w.Header().Set("Content-Type", "text/html")
	_, err := w.Write(tpl)
	if err != nil {
		h.serveError(w, r, err)
		return
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving %s", r.URL.Path)
	h.o.Do(h.init)
	switch r.URL.Path {
	case "/":
		h.serveHTML(w, r, indexTpl)
	case "/ws":
		h.ws(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) ws(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		h.serveError(w, r, ErrWSOnly)
		return
	}

	proc, err := chrome.Start()
	if err != nil {
		h.serveError(w, r, err)
		return
	}
	defer proc.Stop()

	var list []struct {
		Description          string `json:"description"`
		DevtoolsFrontendURL  string `json:"devtoolsFrontendUrl"`
		ID                   string `json:"id"`
		Title                string `json:"title"`
		Type                 string `json:"type"`
		URL                  string `json:"url"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	res, err := http.Get("http://" + proc.Addr() + "/json/list")
	if err != nil {
		h.serveError(w, r, err)
		return
	}
	if err = json.NewDecoder(res.Body).Decode(&list); err != nil {
		res.Body.Close()
		h.serveError(w, r, err)
		return
	}
	res.Body.Close()
	if len(list) != 1 {
		h.serveError(w, r, fmt.Errorf("expected exactly one list result, got %d", len(list)))
		return
	}
	uri, err := url.Parse(list[0].WebSocketDebuggerURL)
	if err != nil {
		h.serveError(w, r, fmt.Errorf("invalid WS debug url: %s", err))
		return
	}
	(&httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = proc.Addr()
			req.URL.Path = uri.Path
		},
	}).ServeHTTP(w, r)
}
