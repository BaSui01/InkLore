package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"InkLore/internal/app"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("InkLore 启动失败: %v", err)
	}
}

func run() error {
	application, err := app.New()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := application.Close(); closeErr != nil {
			log.Printf("释放资源失败: %v", closeErr)
		}
	}()

	address := application.Address()
	timeouts := application.ServerTimeouts()
	// WriteTimeout 有意留空：章节生成使用 SSE 长连接，设置写超时会在流式中途切断响应。
	server := &http.Server{
		Addr:              address,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       timeouts.Read,
		IdleTimeout:       timeouts.Idle,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("InkLore listening on %s (env=%s)", address, application.Config().App.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	select {
	case err := <-serveErr:
		return err
	case sig := <-stop:
		log.Printf("received signal: %v, initiating graceful shutdown...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
	log.Println("server cleanly shut down")
	return nil
}
