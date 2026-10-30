//go:build unit

package box

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/logger"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestServeAndStopGRPC(t *testing.T) {
	listener, err := grpcListener(&config.GRPCConfig{Host: "127.0.0.1", Port: 0})
	require.NoError(t, err)

	server := grpc.NewServer()
	done := make(chan error, 1)
	go func() { done <- serveGRPC(server, listener, "test", logger.NewNop())() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stopGRPC(ctx, server)

	select {
	case err := <-done:
		require.NoError(t, err, "graceful stop is not an error")
	case <-time.After(time.Second):
		t.Fatal("serve did not return after stop")
	}
}

func TestStopGRPC_DeadlineExceeded(t *testing.T) {
	server := grpc.NewServer()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// дедлайн уже истёк - должен отработать Stop и не зависнуть
	finished := make(chan struct{})
	go func() {
		stopGRPC(ctx, server)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("stopGRPC hung")
	}
}

func TestGRPCListener_PortBusy(t *testing.T) {
	first, err := grpcListener(&config.GRPCConfig{Host: "127.0.0.1", Port: 0})
	require.NoError(t, err)
	defer first.Close()

	// второй сервер на занятом порту - ошибка на старте, а не тихий фейл
	busy := first.Addr().(*net.TCPAddr).Port
	_, err = grpcListener(&config.GRPCConfig{Host: "127.0.0.1", Port: busy})
	require.Error(t, err)
}
