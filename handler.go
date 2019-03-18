package echo

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/websocket"
)

var (
	ErrWSOnly = errors.New("this endpoint requires a WebSocket upgrade request")
)

type Handler struct {
	Service *Service
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

	var err error
	var uri *url.URL
	if uris := h.Service.List(); len(uris) != 0 {
		uri = uris[0]
	} else {
		uri, err = h.Service.Start()
		if err != nil {
			h.serveError(w, r, err)
			return
		}
	}
	(&httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = uri.Host
			req.URL.Path = uri.Path
		},
	}).ServeHTTP(w, r)
}
