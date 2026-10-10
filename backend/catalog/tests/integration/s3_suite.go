package integration

import (
	"context"
	"time"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/s3"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
)

type MinioSuite struct {
	suite.Suite
	container testcontainers.Container
	client    *s3.Client
	endpoint  string
}

func (s *MinioSuite) SetupSuite() {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "netvark/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd: []string{"server", "/data"},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err)
	s.container = c

	host, err := c.Host(ctx)
	s.Require().NoError(err)
	port, err := c.MappedPort(ctx, "9000")
	s.Require().NoError(err)

	s.endpoint = host
	s.Require().NoError(err)

	cfg := &config.MinioConfig{
		User:         "minioadmin",
		Password:     "minioadmin",
		ExternalHost: host,
		InternalHost: host,
		Port:         int(port.Num()),
		Bucket:       "test-bucket",
		Expiration:   15 * time.Minute,
	}

	s.client, err = s3.NewClient(cfg)
	s.Require().NoError(err)
}

func (s *MinioSuite) TearDownSuite() {
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}
