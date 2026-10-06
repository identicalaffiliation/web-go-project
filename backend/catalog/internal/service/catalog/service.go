package catalog

import "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"

type Service struct {
	outbox      ports.OutboxRepository
	catalog     ports.CatalogRepository
	logger      ports.Logger
	manager     ports.TxManager
	minioClient ports.S3Client
	cache       ports.Cache
}

func NewService(
	outboxRepository ports.OutboxRepository,
	catalogRepository ports.CatalogRepository,
	logger ports.Logger,
	manager ports.TxManager,
	client ports.S3Client,
	cache ports.Cache,
) *Service {
	return &Service{
		outbox:      outboxRepository,
		catalog:     catalogRepository,
		logger:      logger,
		manager:     manager,
		minioClient: client,
		cache:       cache,
	}
}
