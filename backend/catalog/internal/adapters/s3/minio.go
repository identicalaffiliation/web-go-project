package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	external *minio.Client
	internal *minio.Client
	bucket   string
	exp      time.Duration
}

func (c *Client) GetPresignedURL(ctx context.Context, product *domain.Product) (string, error) {
	if product.ImageKey == "" {
		return "", nil
	}

	url, err := c.external.PresignedPutObject(ctx, c.bucket, product.ImageKey, c.exp)
	if err != nil {
		return "", fmt.Errorf("error GetPresignedURL: %w", err)
	}

	return url.String(), nil
}

func NewClient(cfg *config.MinioConfig) (*Client, error) {
	ops := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.User, cfg.Password, ""),
		Region: "us-east-1",
	}

	externalClient, err := minio.New(
		fmt.Sprintf("%s:%d", cfg.ExternalHost, cfg.Port),
		ops,
	)
	if err != nil {
		return nil, fmt.Errorf("error NewExternalMinioClient: %w", err)
	}

	internalClient, err := minio.New(
		fmt.Sprintf("%s:%d", cfg.InternalHost, cfg.Port),
		ops,
	)
	if err != nil {
		return nil, fmt.Errorf("error NewInternalMinioClient: %w", err)
	}

	c := &Client{external: externalClient, internal: internalClient, bucket: cfg.Bucket, exp: cfg.Expiration}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	if err := c.createBucket(ctx, cfg.Bucket); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) createBucket(ctx context.Context, bucketName string) error {
	ok, err := c.internal.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("error createBucket: %w", err)
	}

	if !ok {
		if err := c.internal.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("error createBucket: %w", err)
		}
	}

	return nil
}
