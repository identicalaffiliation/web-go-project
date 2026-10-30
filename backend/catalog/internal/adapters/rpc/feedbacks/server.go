package feedbacks

import (
	"context"
	"errors"

	feedbacksv1 "github.com/identicalaffiliation/web-go-project/backend/catalog/gen/pb/feedbacks/v1"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/feedbacks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GRPCServer struct {
	feedbacksv1.UnimplementedFeedbackServiceServer
	app ports.FeedbackService
}

func NewGRPCServer(app ports.FeedbackService) *grpc.Server {
	server := grpc.NewServer()
	feedbacksv1.RegisterFeedbackServiceServer(server, &GRPCServer{app: app})
	return server
}

func (s *GRPCServer) GetMyCreatedFeedbacks(ctx context.Context, req *feedbacksv1.GetMyFeedbacksRequest) (*feedbacksv1.GetMyFeedbacksResponse, error) {
	response, err := s.app.GetMyCreatedFeedbacks(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, dto.ErrInvalidData) {
			return nil, status.Error(codes.InvalidArgument, "invalid user id")
		}

		return nil, status.Error(codes.Internal, "failed to get feedbacks")
	}

	result := make([]*feedbacksv1.Feedback, 0, len(response))
	for _, v := range response {
		result = append(result, &feedbacksv1.Feedback{
			Id:          v.ID.String(),
			ProductId:   v.ProductID.String(),
			UserId:      v.UserID.String(),
			Score:       int32(v.Score),
			Description: v.Description,
			CreatedAt:   timestamppb.New(v.CreatedAt),
		})
	}

	return &feedbacksv1.GetMyFeedbacksResponse{Feedbacks: result}, nil
}

func (s *GRPCServer) DeleteFeedback(ctx context.Context, req *feedbacksv1.DeleteFeedbackRequest) (*feedbacksv1.DeleteFeedbackResponse, error) {
	if err := s.app.DeleteFeedback(ctx, req.GetId()); err != nil {
		switch {
		case errors.Is(err, dto.ErrInvalidData):
			return nil, status.Error(codes.InvalidArgument, "invalid feedback id")
		case errors.Is(err, feedbacks.ErrNotFound):
			return nil, status.Error(codes.NotFound, "feedback not found")
		default:
			return nil, status.Error(codes.Internal, "failed to delete feedback")
		}
	}

	return &feedbacksv1.DeleteFeedbackResponse{}, nil
}
