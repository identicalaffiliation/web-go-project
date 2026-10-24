//go:build integration

package integration

import (
	"context"
	"net"
	"sync"
	"testing"
	"uuid"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	balancev1 "github.com/identicalaffiliation/web-go-project/backend/payments/gen/pb/balance/v1"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/rpc"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/account"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/balance"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/workflow"
	"github.com/identicalaffiliation/web-go-project/backend/payments/pkg/logger"
)

// PaymentsSuite - сервисы payments поверх настоящего postgres.
type PaymentsSuite struct {
	suite.Suite

	db       Databases
	repo     *postgres.Repository
	accounts *account.Service
	payments *workflow.Service
	client   balancev1.PaymentsServiceClient
}

func TestPaymentsSuite(t *testing.T) {
	suite.Run(t, new(PaymentsSuite))
}

func (s *PaymentsSuite) SetupSuite() {
	s.db = startPostgres(s.T())
	s.repo = postgres.NewRepository(s.db.Master, s.db.Replica)

	log, err := logger.ProvideLogger()
	s.Require().NoError(err)

	trm := manager.Must(pgxv5.NewDefaultFactory(s.db.Master))
	s.accounts = account.NewService(s.repo, log, trm, initialBalance)
	s.payments = workflow.NewService(s.repo, log, trm)

	// баланс читается с реплики, поэтому grpc поднимаем на репозитории, где реплика = master
	s.client = s.startGRPC(balance.NewService(postgres.NewRepository(s.db.Master, s.db.Master), log))
}

func (s *PaymentsSuite) TearDownTest() {
	truncate(s.T(), s.db.Master, s.db.Replica)
}

func (s *PaymentsSuite) startGRPC(svc rpc.Service) balancev1.PaymentsServiceClient {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	s.Require().NoError(err)

	server := grpc.NewServer()
	balancev1.RegisterPaymentsServiceServer(server, rpc.NewRPChandler(svc))

	go func() {
		_ = server.Serve(listener)
	}()
	s.T().Cleanup(server.Stop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = conn.Close()
	})

	return balancev1.NewPaymentsServiceClient(conn)
}

func (s *PaymentsSuite) openAccount() uuid.UUID {
	userID := uuid.NewV7()
	s.Require().NoError(s.accounts.ProcessTask(s.T().Context(), &domain.UserCreated{EventID: uuid.NewV7(), UserID: userID}))

	return userID
}

func (s *PaymentsSuite) balance(userID uuid.UUID) int64 {
	var b int64
	s.Require().NoError(s.db.Master.QueryRow(s.T().Context(), `SELECT balance FROM accounts WHERE user_id = $1`, userID).Scan(&b))

	return b
}

func (s *PaymentsSuite) count(query string, args ...any) int {
	var c int
	s.Require().NoError(s.db.Master.QueryRow(s.T().Context(), query, args...).Scan(&c))

	return c
}

func (s *PaymentsSuite) newPayment(userID uuid.UUID, amount int64) *domain.Payment {
	return &domain.Payment{OrderID: uuid.NewV7(), UserID: userID, Amount: amount}
}

// ---------- открытие счета (консьюмер) ----------

func (s *PaymentsSuite) Test_ProcessTask_OpensAccountWithInitialBalance() {
	userID := s.openAccount()

	s.Equal(int64(initialBalance), s.balance(userID))
}

func (s *PaymentsSuite) Test_ProcessTask_SameEventTwice() {
	event := &domain.UserCreated{EventID: uuid.NewV7(), UserID: uuid.NewV7()}

	s.Require().NoError(s.accounts.ProcessTask(s.T().Context(), event))
	// за это время пользователь успел потратить деньги - повтор события не должен их вернуть
	_, err := s.db.Master.Exec(s.T().Context(), `UPDATE accounts SET balance = 10 WHERE user_id = $1`, event.UserID)
	s.Require().NoError(err)

	s.Require().NoError(s.accounts.ProcessTask(s.T().Context(), event))

	s.Equal(int64(10), s.balance(event.UserID))
	s.Equal(1, s.count(`SELECT COUNT(*) FROM processed_events`))
}

func (s *PaymentsSuite) Test_ProcessTask_DifferentEventsSameUser() {
	userID := uuid.NewV7()

	for range 3 {
		s.Require().NoError(s.accounts.ProcessTask(s.T().Context(), &domain.UserCreated{EventID: uuid.NewV7(), UserID: userID}))
	}

	s.Equal(int64(initialBalance), s.balance(userID))
	s.Equal(1, s.count(`SELECT COUNT(*) FROM accounts`))
}

func (s *PaymentsSuite) Test_ProcessTask_ConcurrentDuplicates() {
	event := &domain.UserCreated{EventID: uuid.NewV7(), UserID: uuid.NewV7()}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			s.NoError(s.accounts.ProcessTask(context.Background(), event))
		})
	}
	wg.Wait()

	s.Equal(int64(initialBalance), s.balance(event.UserID))
	s.Equal(1, s.count(`SELECT COUNT(*) FROM processed_events`))
}

// ---------- списание ----------

func (s *PaymentsSuite) Test_Charge_DebitsBalance() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)

	s.Require().NoError(s.payments.Charge(s.T().Context(), p))

	s.Equal(int64(initialBalance-300), s.balance(userID))
	s.Equal(1, s.count(`SELECT COUNT(*) FROM payment_operations WHERE order_id = $1 AND operation = 'charge'`, p.OrderID))
}

// ретрай активити: повторное списание по тому же заказу ничего не меняет
func (s *PaymentsSuite) Test_Charge_Idempotent() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)

	for range 3 {
		s.Require().NoError(s.payments.Charge(s.T().Context(), p))
	}

	s.Equal(int64(initialBalance-300), s.balance(userID))
}

func (s *PaymentsSuite) Test_Charge_ConcurrentSameOrder() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			s.NoError(s.payments.Charge(context.Background(), p))
		})
	}
	wg.Wait()

	s.Equal(int64(initialBalance-300), s.balance(userID))
}

// конкурентные заказы одного пользователя не уводят баланс в минус
func (s *PaymentsSuite) Test_Charge_ConcurrentOrdersNeverOverdraw() {
	userID := s.openAccount()
	const amount = initialBalance / 4

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		paid   int
		failed int
	)

	for range 10 {
		wg.Go(func() {
			err := s.payments.Charge(context.Background(), s.newPayment(userID, amount))

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				paid++
			default:
				s.ErrorIs(err, domain.ErrInsufficientFunds)
				failed++
			}
		})
	}
	wg.Wait()

	s.Equal(4, paid)
	s.Equal(6, failed)
	s.Equal(int64(0), s.balance(userID))
	s.Equal(4, s.count(`SELECT COUNT(*) FROM payment_operations WHERE operation = 'charge'`))
}

func (s *PaymentsSuite) Test_Charge_InsufficientFunds() {
	userID := s.openAccount()
	p := s.newPayment(userID, initialBalance+1)

	s.Require().ErrorIs(s.payments.Charge(s.T().Context(), p), domain.ErrInsufficientFunds)

	s.Equal(int64(initialBalance), s.balance(userID))
	s.Equal(0, s.count(`SELECT COUNT(*) FROM payment_operations`))
}

func (s *PaymentsSuite) Test_Charge_AccountNotFound() {
	p := s.newPayment(uuid.NewV7(), 100)

	s.Require().ErrorIs(s.payments.Charge(s.T().Context(), p), domain.ErrAccountNotFound)
	s.Equal(0, s.count(`SELECT COUNT(*) FROM payment_operations`))
}

// ---------- возврат ----------

func (s *PaymentsSuite) Test_Refund_ReturnsChargedAmount() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)
	s.Require().NoError(s.payments.Charge(s.T().Context(), p))

	// сумма во входе возврата не важна - вернется списанная
	refund := *p
	refund.Amount = 1
	for range 3 {
		s.Require().NoError(s.payments.Refund(s.T().Context(), &refund))
	}

	s.Equal(int64(initialBalance), s.balance(userID))
}

func (s *PaymentsSuite) Test_Refund_WithoutCharge() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)

	s.Require().NoError(s.payments.Refund(s.T().Context(), p))

	s.Equal(int64(initialBalance), s.balance(userID))
	s.Equal(1, s.count(`SELECT COUNT(*) FROM payment_operations WHERE order_id = $1 AND operation = 'refund' AND amount = 0`, p.OrderID))
}

// запоздавшее списание после компенсации отклоняется
func (s *PaymentsSuite) Test_Charge_AfterRefundRejected() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)

	s.Require().NoError(s.payments.Refund(s.T().Context(), p))
	s.Require().ErrorIs(s.payments.Charge(s.T().Context(), p), domain.ErrOrderRefunded)

	s.Equal(int64(initialBalance), s.balance(userID))
}

func (s *PaymentsSuite) Test_ChargeAndRefund_Concurrent() {
	userID := s.openAccount()
	p := s.newPayment(userID, 300)

	var wg sync.WaitGroup
	wg.Go(func() {
		err := s.payments.Charge(context.Background(), p)
		if err != nil {
			s.ErrorIs(err, domain.ErrOrderRefunded)
		}
	})
	wg.Go(func() {
		s.NoError(s.payments.Refund(context.Background(), p))
	})
	wg.Wait()

	// в любом порядке деньги в итоге на месте
	s.Equal(int64(initialBalance), s.balance(userID))
}

// ---------- master / replica ----------

func (s *PaymentsSuite) Test_GetAccount_ReadsReplicaOutsideTx() {
	userID := s.openAccount()

	_, err := s.repo.GetAccount(s.T().Context(), userID)
	s.Require().ErrorIs(err, domain.ErrAccountNotFound, "счет есть только в master, вне транзакции читаем реплику")

	_, err = s.db.Replica.Exec(s.T().Context(), `INSERT INTO accounts (user_id, balance) VALUES ($1, 42)`, userID)
	s.Require().NoError(err)

	acc, err := s.repo.GetAccount(s.T().Context(), userID)
	s.Require().NoError(err)
	s.Equal(int64(42), acc.Balance)
}

func (s *PaymentsSuite) Test_GetAccount_ReadsMasterInsideTx() {
	userID := s.openAccount()
	trm := manager.Must(pgxv5.NewDefaultFactory(s.db.Master))

	var got *domain.Account
	err := trm.Do(s.T().Context(), func(ctx context.Context) error {
		var err error
		got, err = s.repo.GetAccount(ctx, userID)
		return err
	})

	s.Require().NoError(err)
	s.Equal(int64(initialBalance), got.Balance)
}

// ---------- grpc ----------

func (s *PaymentsSuite) Test_GRPC_GetBalanceByUser() {
	userID := s.openAccount()
	s.Require().NoError(s.payments.Charge(s.T().Context(), s.newPayment(userID, 300)))

	resp, err := s.client.GetBalanceByUser(s.T().Context(), &balancev1.GetBalanceRequest{UserId: userID.String()})
	s.Require().NoError(err)
	s.Equal(userID.String(), resp.GetUserId())
	s.Equal(int64(initialBalance-300), resp.GetAmount())
}

func (s *PaymentsSuite) Test_GRPC_Errors() {
	_, err := s.client.GetBalanceByUser(s.T().Context(), &balancev1.GetBalanceRequest{UserId: uuid.NewV7().String()})
	s.Equal(codes.NotFound, status.Code(err))

	_, err = s.client.GetBalanceByUser(s.T().Context(), &balancev1.GetBalanceRequest{UserId: "not-uuid"})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

var _ ports.Repository = (*postgres.Repository)(nil)
