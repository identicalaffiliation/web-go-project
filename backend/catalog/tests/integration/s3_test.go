//go:build integration

package integration

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/stretchr/testify/suite"
)

func TestMinioSuite(t *testing.T) {
	suite.Run(t, new(MinioSuite))
}

func (s *MinioSuite) Test_GetPresignedURL_EmptyKey_ReturnsEmpty() {
	p := &domain.Product{ImageKey: ""}
	url, err := s.client.GetPresignedURL(context.Background(), p)
	s.Require().NoError(err)
	s.Empty(url)
}

func (s *MinioSuite) Test_GetPresignedURL_ValidKey_ReturnsURL() {
	p := &domain.Product{ImageKey: "products/2026/October/test.png"}
	url, err := s.client.GetPresignedURL(context.Background(), p)
	s.Require().NoError(err)
	s.NotEmpty(url)
	s.Contains(url, "test-bucket")
	s.Contains(url, "products/2026/October/test.png")
	s.Contains(url, "X-Amz-Signature")
}

func (s *MinioSuite) Test_PresignedURL_ActuallyUploads() {
	key := "products/test-upload.png"
	p := &domain.Product{ImageKey: key}

	url, err := s.client.GetPresignedURL(context.Background(), p)
	s.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader([]byte("hello")))
	s.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer func() {
		_ = resp.Body.Close()
	}()

	s.Equal(http.StatusOK, resp.StatusCode)
}
