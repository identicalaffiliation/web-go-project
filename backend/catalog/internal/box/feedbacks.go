package box

import (
	feedbacksRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/feedbacks"
	feedbacksRPC "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rpc/feedbacks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/feedbacks"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

func setupFeedbacks(master, replica *pgxpool.Pool, logger ports.Logger) (*feedbacks.FeedbackService, *grpc.Server) {
	repo := feedbacksRepository.NewRepository(master, replica)
	service := feedbacks.NewService(repo, logger)

	return service, feedbacksRPC.NewGRPCServer(service)
}
