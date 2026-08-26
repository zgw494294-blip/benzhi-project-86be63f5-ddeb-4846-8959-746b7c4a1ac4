package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/httpui"
	"oral-history-release-studio/internal/persistence"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	addressFlag := flags.String("addr", "", "回环监听地址")
	dataFlag := flags.String("data", "data", "持久化数据目录")
	selfcheck := flags.Bool("selfcheck", false, "运行完整回环 HTTP 自检后退出")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	address, err := resolveAddress(*addressFlag)
	if err != nil {
		return err
	}
	dataDir := *dataFlag
	cleanup := func() {}
	if *selfcheck {
		dataDir, err = os.MkdirTemp("", "oral-history-selfcheck-")
		if err != nil {
			return err
		}
		cleanup = func() { _ = os.RemoveAll(dataDir) }
	}
	defer cleanup()
	store, err := persistence.Open(dataDir)
	if err != nil {
		return fmt.Errorf("打开持久化存储失败: %w", err)
	}
	service := application.NewService(store)
	ui := httpui.New(service)
	server := &http.Server{Handler: ui.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", address, err)
	}
	if *selfcheck {
		if err := runSelfcheck(context.Background(), listener, server); err != nil {
			return fmt.Errorf("selfcheck 失败: %w", err)
		}
		log.Printf("selfcheck 通过：完整公开授权流程与凭据校验成功")
		return nil
	}
	log.Printf("口述史公开授权工作台已监听 http://%s", listener.Addr())
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signalCtx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
