package echo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gofu/echo/chrome"
)

type serviceHandle struct {
	process   *chrome.Process
	uri       *url.URL
	stoppedAt time.Time
}

type Service struct {
	mu      sync.RWMutex
	handles []*serviceHandle
}

// StopAll stops all Chrome handles and blocks until they return.
// Call Cleanup to remove the references.
func (s *Service) StopAll() {
	s.mu.RLock()
	for _, h := range s.handles {
		if err := h.process.Stop(); err != nil {
			log.Printf("Process stop error: %s", err)
		}
	}
	for _, h := range s.handles {
		<-h.process.Done()
	}
	s.mu.RUnlock()
}

// Cleanup removes references to stopped Chrome handles.
func (s *Service) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
rewind:
	for i, h := range s.handles {
		if !h.stoppedAt.IsZero() {
			s.handles = append(s.handles[:i], s.handles[i+1:]...)
			goto rewind
		}
	}
}

func (s *Service) Start() (*url.URL, error) {
	proc, err := chrome.Start()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if err := proc.Stop(); err != nil {
				log.Printf("stop Chrome: %s", err)
			}
		}
	}()

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
		return nil, err
	}
	buf, err := ioutil.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read DevTools response: %s", err)
	}
	if err = json.Unmarshal(buf, &list); err != nil {
		return nil, err
	}
	if len(list) != 1 {
		return nil, fmt.Errorf("expected exactly one list result, got %s", string(buf))
	}
	uri, err := url.Parse(list[0].WebSocketDebuggerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid WS debug url: %s", err)
	}
	go func() {
		s.mu.Lock()
		handle := &serviceHandle{process: proc, uri: uri}
		s.handles = append(s.handles, handle)
		s.mu.Unlock()

		<-proc.Done()

		s.mu.Lock()
		handle.stoppedAt = time.Now()
		s.mu.Unlock()
	}()
	return uri, nil
}

func (s *Service) List() []*url.URL {
	s.mu.Lock()
	defer s.mu.Unlock()
	uris := make([]*url.URL, len(s.handles))
	for i, h := range s.handles {
		uris[i] = h.uri
	}
	return uris
}
