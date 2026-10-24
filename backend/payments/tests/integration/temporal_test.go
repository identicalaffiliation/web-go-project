//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/temporal"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/account"
	paymentsworkflow "github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/workflow"
	"github.com/identicalaffiliation/web-go-project/backend/payments/pkg/logger"
)

const (
	paymentsQueue = "payments"
	callerQueue   = "payments-test-caller"
	temporalSpace = "default"
	sagaTimeout   = 30 * time.Second
)

// paymentInput - то, что шлет сага orders (orders/internal/adapters/temporal.PaymentInput).
type paymentInput struct {
	OrderID uuid.UUID `json:"orderId"`
	UserID  uuid.UUID `json:"userId"`
	Amount  int64     `json:"amount"`
}

// chargeThenMaybeRefund - тестовый workflow, вызывает активити payments по имени и очереди, как сага orders.
func chargeThenMaybeRefund(ctx workflow.Context, in paymentInput, refund bool) error {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           paymentsQueue,
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         &sdktemporal.RetryPolicy{MaximumAttempts: 3},
	})

	chargeErr := workflow.ExecuteActivity(actx, temporal.ChargeActivity, in).Get(ctx, nil)
	if chargeErr == nil && !refund {
		return nil
	}

	if err := workflow.ExecuteActivity(actx, temporal.RefundActivity, in).Get(ctx, nil); err != nil {
		return err
	}

	return chargeErr
}

// TemporalSuite - воркер payments на настоящем temporal dev server.
type TemporalSuite struct {
	suite.Suite

	db       Databases
	accounts *account.Service
	temporal client.Client

	stopWorker   chan struct{}
	workerDone   chan error
	callerWorker worker.Worker
}

func TestTemporalSuite(t *testing.T) {
	suite.Run(t, new(TemporalSuite))
}

func (s *TemporalSuite) SetupSuite() {
	s.db = startPostgres(s.T())
	hostPort := startTemporal(s.T())

	log, err := logger.ProvideLogger()
	s.Require().NoError(err)

	repo := postgres.NewRepository(s.db.Master, s.db.Replica)
	trm := manager.Must(pgxv5.NewDefaultFactory(s.db.Master))
	s.accounts = account.NewService(repo, log, trm, initialBalance)
	activities := temporal.NewActivities(paymentsworkflow.NewService(repo, log, trm))

	cfg := &config.TemporalConfig{
		HostPort:                hostPort,
		Namespace:               temporalSpace,
		TaskQueue:               paymentsQueue,
		MaxConcurrentActivities: 10,
	}

	// temporal dev server принимает tcp раньше, чем готов namespace, поэтому ретраим dial
	var paymentsWorker *temporal.Worker
	s.Require().Eventually(func() bool {
		paymentsWorker, err = temporal.NewWorker(cfg, activities)
		return err == nil
	}, waitTimeout, pollTick, "dial temporal: %v", err)

	s.stopWorker = make(chan struct{})
	s.workerDone = make(chan error, 1)
	go func() {
		s.workerDone <- paymentsWorker.Run(s.stopWorker)
	}()

	s.temporal, err = client.Dial(client.Options{HostPort: hostPort, Namespace: temporalSpace})
	s.Require().NoError(err)

	s.callerWorker = worker.New(s.temporal, callerQueue, worker.Options{})
	s.callerWorker.RegisterWorkflow(chargeThenMaybeRefund)
	s.Require().NoError(s.callerWorker.Start())
}

func (s *TemporalSuite) TearDownSuite() {
	s.callerWorker.Stop()
	close(s.stopWorker)
	s.NoError(<-s.workerDone)
	s.temporal.Close()
}

func (s *TemporalSuite) TearDownTest() {
	truncate(s.T(), s.db.Master)
}

func (s *TemporalSuite) run(in paymentInput, refund bool) error {
	ctx, cancel := context.WithTimeout(s.T().Context(), sagaTimeout)
	defer cancel()

	run, err := s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "payments-test-" + in.OrderID.String(),
		TaskQueue: callerQueue,
	}, chargeThenMaybeRefund, in, refund)
	s.Require().NoError(err)

	return run.Get(ctx, nil)
}

func (s *TemporalSuite) openAccount() uuid.UUID {
	userID := uuid.NewV7()
	s.Require().NoError(s.accounts.ProcessTask(s.T().Context(), &domain.UserCreated{EventID: uuid.NewV7(), UserID: userID}))

	return userID
}

func (s *TemporalSuite) balance(userID uuid.UUID) int64 {
	var b int64
	s.Require().NoError(s.db.Master.QueryRow(s.T().Context(), `SELECT balance FROM accounts WHERE user_id = $1`, userID).Scan(&b))

	return b
}

func (s *TemporalSuite) Test_Charge() {
	userID := s.openAccount()

	s.Require().NoError(s.run(paymentInput{OrderID: uuid.NewV7(), UserID: userID, Amount: 300}, false))

	s.Equal(int64(initialBalance-300), s.balance(userID))
}

func (s *TemporalSuite) Test_ChargeThenRefund() {
	userID := s.openAccount()

	s.Require().NoError(s.run(paymentInput{OrderID: uuid.NewV7(), UserID: userID, Amount: 300}, true))

	s.Equal(int64(initialBalance), s.balance(userID))
}

// недостаточно средств - non retryable ошибка с типом InsufficientFunds, баланс не тронут
func (s *TemporalSuite) Test_InsufficientFunds() {
	userID := s.openAccount()

	err := s.run(paymentInput{OrderID: uuid.NewV7(), UserID: userID, Amount: initialBalance + 1}, false)

	appErr, ok := errors.AsType[*sdktemporal.ApplicationError](err)
	s.Require().True(ok, "error: %v", err)
	s.Equal(temporal.ErrTypeInsufficientFunds, appErr.Type())
	s.Equal(int64(initialBalance), s.balance(userID))
}

func (s *TemporalSuite) Test_AccountNotFound() {
	err := s.run(paymentInput{OrderID: uuid.NewV7(), UserID: uuid.NewV7(), Amount: 100}, false)

	appErr, ok := errors.AsType[*sdktemporal.ApplicationError](err)
	s.Require().True(ok, "error: %v", err)
	s.Equal(temporal.ErrTypeAccountNotFound, appErr.Type())
}
