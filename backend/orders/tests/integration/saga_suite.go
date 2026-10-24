//go:build integration

package integration

import (
	"context"
	"sync"
	"time"
	"uuid"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"google.golang.org/grpc"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/temporal"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
	"github.com/identicalaffiliation/web-go-project/backend/orders/pkg/logger"
)

const (
	sagaTimeout   = 30 * time.Second
	ordersQueue   = "orders"
	temporalSpace = "default"
)

type fakeInventory struct {
	mu         sync.Mutex
	outOfStock map[uuid.UUID]struct{}
	reserved   map[uuid.UUID]int
	released   map[uuid.UUID]int
}

func newFakeInventory() *fakeInventory {
	return &fakeInventory{
		outOfStock: make(map[uuid.UUID]struct{}),
		reserved:   make(map[uuid.UUID]int),
		released:   make(map[uuid.UUID]int),
	}
}

func (f *fakeInventory) ReserveStock(_ context.Context, in temporal.SagaInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.outOfStock[in.ProductID]; ok {
		return sdktemporal.NewNonRetryableApplicationError("insufficient stock", "InsufficientStock", nil)
	}

	f.reserved[in.OrderID]++

	return nil
}

func (f *fakeInventory) ReleaseStock(_ context.Context, in temporal.SagaInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.released[in.OrderID]++

	return nil
}

func (f *fakeInventory) setOutOfStock(productID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.outOfStock[productID] = struct{}{}
}

func (f *fakeInventory) calls(orderID uuid.UUID) (reserved, released int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.reserved[orderID], f.released[orderID]
}

type fakePayments struct {
	mu        sync.Mutex
	noFunds   map[uuid.UUID]struct{}
	charged   map[uuid.UUID]int64
	refunded  map[uuid.UUID]int
	lastInput map[uuid.UUID]temporal.PaymentInput
}

func newFakePayments() *fakePayments {
	return &fakePayments{
		noFunds:   make(map[uuid.UUID]struct{}),
		charged:   make(map[uuid.UUID]int64),
		refunded:  make(map[uuid.UUID]int),
		lastInput: make(map[uuid.UUID]temporal.PaymentInput),
	}
}

func (f *fakePayments) Charge(_ context.Context, in temporal.PaymentInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastInput[in.OrderID] = in

	if _, ok := f.noFunds[in.UserID]; ok {
		return sdktemporal.NewNonRetryableApplicationError("insufficient funds", "InsufficientFunds", nil)
	}

	f.charged[in.OrderID] += in.Amount

	return nil
}

func (f *fakePayments) Refund(_ context.Context, in temporal.PaymentInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.refunded[in.OrderID]++

	return nil
}

func (f *fakePayments) setNoFunds(userID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.noFunds[userID] = struct{}{}
}

func (f *fakePayments) calls(orderID uuid.UUID) (charged int64, refunded int, in temporal.PaymentInput) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.charged[orderID], f.refunded[orderID], f.lastInput[orderID]
}

type SagaSuite struct {
	suite.Suite

	pool      *pgxpool.Pool
	repo      *postgres.Repository
	service   *orders.Service
	catalog   *fakeCatalog
	inventory *fakeInventory
	payments  *fakePayments
	temporal  client.Client

	stopOrders      chan struct{}
	ordersDone      chan error
	inventoryWorker worker.Worker
	paymentsWorker  worker.Worker
}

func (s *SagaSuite) SetupSuite() {
	s.pool = startPostgres(s.T())
	s.repo = postgres.NewRepository(s.pool, s.pool)

	conn := s.startCatalog()
	hostPort := startTemporal(s.T())

	log, err := logger.ProvideLogger()
	s.Require().NoError(err)

	ordersWorker, err := s.dialOrdersWorker(hostPort)
	s.Require().NoError(err)

	s.stopOrders = make(chan struct{})
	s.ordersDone = make(chan error, 1)
	go func() {
		s.ordersDone <- ordersWorker.Run(s.stopOrders)
	}()

	s.temporal, err = client.Dial(client.Options{HostPort: hostPort, Namespace: temporalSpace})
	s.Require().NoError(err)

	s.inventory = newFakeInventory()
	s.inventoryWorker = worker.New(s.temporal, temporal.InventoryTaskQueue, worker.Options{})
	s.inventoryWorker.RegisterActivityWithOptions(s.inventory.ReserveStock, activity.RegisterOptions{Name: temporal.ReserveStockActivity})
	s.inventoryWorker.RegisterActivityWithOptions(s.inventory.ReleaseStock, activity.RegisterOptions{Name: temporal.ReleaseStockActivity})
	s.Require().NoError(s.inventoryWorker.Start())

	s.payments = newFakePayments()
	s.paymentsWorker = worker.New(s.temporal, temporal.PaymentsTaskQueue, worker.Options{})
	s.paymentsWorker.RegisterActivityWithOptions(s.payments.Charge, activity.RegisterOptions{Name: temporal.ChargeActivity})
	s.paymentsWorker.RegisterActivityWithOptions(s.payments.Refund, activity.RegisterOptions{Name: temporal.RefundActivity})
	s.Require().NoError(s.paymentsWorker.Start())

	s.service = orders.NewService(
		s.repo,
		log,
		manager.Must(pgxv5.NewDefaultFactory(s.pool)),
		catalog.NewClient(conn),
		ordersWorker,
	)
}

func (s *SagaSuite) TearDownSuite() {
	s.inventoryWorker.Stop()
	s.paymentsWorker.Stop()
	close(s.stopOrders)
	s.NoError(<-s.ordersDone)
	s.temporal.Close()
}

func (s *SagaSuite) TearDownTest() {
	_, err := s.pool.Exec(s.T().Context(), `TRUNCATE TABLE orders`)
	s.Require().NoError(err)
}

func (s *SagaSuite) startCatalog() *grpc.ClientConn {
	fake, conn := startFakeCatalog(s.T())
	s.catalog = fake

	return conn
}

// dialOrdersWorker - temporal dev server принимает tcp раньше, чем готов namespace, поэтому ретраим dial.
func (s *SagaSuite) dialOrdersWorker(hostPort string) (*temporal.Worker, error) {
	cfg := &config.TemporalConfig{
		HostPort:                hostPort,
		Namespace:               temporalSpace,
		TaskQueue:               ordersQueue,
		MaxConcurrentActivities: 10,
	}

	var (
		w   *temporal.Worker
		err error
	)

	s.Require().Eventually(func() bool {
		w, err = temporal.NewWorker(cfg, temporal.NewActivities(s.repo))
		return err == nil
	}, waitTimeout, pollTick, "dial temporal: %v", err)

	return w, err
}

// waitStatus - ждет, пока сага переведет заказ в нужный статус.
func (s *SagaSuite) waitStatus(orderID uuid.UUID, want domain.Status) {
	var got domain.Status

	ok := s.Eventually(func() bool {
		err := s.pool.QueryRow(s.T().Context(), `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&got)
		return err == nil && got == want
	}, sagaTimeout, pollTick)

	s.Require().True(ok, "order %s status %q, want %q", orderID, got, want)
}

// waitSagaCompleted - ждет завершения workflow заказа и возвращает его ошибку.
func (s *SagaSuite) waitSagaCompleted(orderID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(s.T().Context(), sagaTimeout)
	defer cancel()

	return s.temporal.GetWorkflow(ctx, "order-"+orderID.String(), "").Get(ctx, nil)
}

func (s *SagaSuite) ordersCount() int {
	var count int
	s.Require().NoError(s.pool.QueryRow(s.T().Context(), `SELECT COUNT(*) FROM orders`).Scan(&count))

	return count
}
