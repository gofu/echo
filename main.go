package echo

import (
	"context"
	"log"
	"net"
	"net/http"
)

func Main(ctx context.Context, addr string) error {
	svc := &Service{}
	defer svc.StopAll()
	handler := &Handler{
		Service: svc,
	}
	srv := &http.Server{
		Handler: handler,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	serveCh := make(chan error, 1)
	log.Printf("Listening on http://%s", ln.Addr())
	go func() { serveCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		if err := srv.Close(); err != nil {
			return err
		}
		if err := <-serveCh; err != http.ErrServerClosed {
			return err
		}
		return ctx.Err()
	case err := <-serveCh:
		return err
	}
}
