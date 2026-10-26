//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
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

func (s *MinioSuite) Test_FetchSignedURL_EmptyKey_ReturnsEmpty() {
	url, err := s.client.FetchSignedURL(context.Background(), "")
	s.Require().NoError(err)
	s.Empty(url)
}

func (s *MinioSuite) Test_FetchSignedURL_ValidKey_ReturnsURL() {
	url, err := s.client.FetchSignedURL(context.Background(), "products/2026/October/test.png")
	s.Require().NoError(err)
	s.NotEmpty(url)
	s.Contains(url, "test-bucket")
	s.Contains(url, "products/2026/October/test.png")
	s.Contains(url, "X-Amz-Signature")
}

func (s *MinioSuite) Test_FetchSignedURL_ActuallyDownloads() {
	key := "products/test-download.png"
	content := []byte("hello download")

	putURL, err := s.client.GetPresignedURL(context.Background(), &domain.Product{ImageKey: key})
	s.Require().NoError(err)

	putReq, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(content))
	s.Require().NoError(err)

	putResp, err := http.DefaultClient.Do(putReq)
	s.Require().NoError(err)
	_ = putResp.Body.Close()
	s.Require().Equal(http.StatusOK, putResp.StatusCode)

	getURL, err := s.client.FetchSignedURL(context.Background(), key)
	s.Require().NoError(err)

	resp, err := http.Get(getURL)
	s.Require().NoError(err)
	defer func() {
		_ = resp.Body.Close()
	}()

	s.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Equal(content, body)
}

func (s *MinioSuite) Test_FetchSignedURL_MissingObject_NotFound() {
	url, err := s.client.FetchSignedURL(context.Background(), "products/not-exists.png")
	s.Require().NoError(err)

	resp, err := http.Get(url)
	s.Require().NoError(err)
	defer func() {
		_ = resp.Body.Close()
	}()

	s.Equal(http.StatusNotFound, resp.StatusCode)
}
