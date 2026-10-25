package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xinghai-osc/xinghai-router/internal/app"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	stop, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	svc, err := app.New(stop, cfg)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		svc.Close()
		return fmt.Errorf("listen: %w", err)
	}
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           svc.Handler(),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
	served := make(chan error, 1)
	go func() {
		log.Printf("xinghai-router listening on %s", listener.Addr())
		served <- server.Serve(listener)
	}()
	var serveErr error
	select {
	case <-stop.Done():
	case serveErr = <-served:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
	}
	ctx := context.Background()
	if cfg.ShutdownTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.ShutdownTimeout)
		defer cancel()
	}
	if err := svc.Shutdown(ctx, server); err != nil {
		return fmt.Errorf("drain incomplete; process exiting with active requests: %w", err)
	}
	svc.Close()
	return serveErr
}
