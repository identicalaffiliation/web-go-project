//go:build unit

package dto

import (
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewFile(t *testing.T) {
	f := NewFile(nil, nil)
	require.NotNil(t, f)
}

func TestReq_BuildingImageKey(t *testing.T) {

	t.Run("nil file", func(t *testing.T) {
		r := &CreateProductRequest{
			File: nil,
		}

		require.Equal(t, "", r.buildImageKeyFromImageName())
	})

	t.Run("not nil file", func(t *testing.T) {
		mime := textproto.MIMEHeader{}
		mime.Set("Content-Type", "some mime")
		mimeHeader := multipart.FileHeader{
			Header:   mime,
			Filename: "test",
		}

		f := &File{
			header: &mimeHeader,
		}

		r := &CreateProductRequest{
			File: f,
		}

		n := time.Now().UTC()

		y := n.Year()
		m := n.Month()

		key := r.buildImageKeyFromImageName()
		suffix := strings.TrimPrefix(key, "image/"+strconv.Itoa(y)+"/"+m.String()+"/")
		id := strings.TrimSuffix(suffix, ".some mime")

		require.Equal(t, "image/2026/"+m.String()+"/"+id+".some mime", key)
	})
}

func TestToDomain(t *testing.T) {

	t.Run("file - nil", func(t *testing.T) {
		r := &CreateProductRequest{
			Title:       "aa",
			Description: new("bb"),
			Price:       123,
		}

		d := r.ToDomain()
		require.NotNil(t, d)
		require.Equal(t, r.Title, d.Title)
		require.Equal(t, *r.Description, d.Description)
		require.Equal(t, r.Price, d.Price)
		require.Equal(t, "", d.ImageKey)
		require.Empty(t, d.CreatedAt)
		require.Empty(t, d.UpdatedAt)
		require.NotEmpty(t, d.ID)
	})

	t.Run("file - not nil", func(t *testing.T) {
		mime := textproto.MIMEHeader{}
		mime.Set("Content-Type", "some mime")
		mimeHeader := multipart.FileHeader{
			Header:   mime,
			Filename: "test",
		}

		f := &File{
			header: &mimeHeader,
		}

		r := &CreateProductRequest{
			File:  f,
			Title: "123",
			Price: 123,
		}

		n := time.Now().UTC()
		y := strconv.Itoa(n.Year())
		m := n.Month().String()

		d := r.ToDomain()
		require.NotNil(t, d)
		require.Empty(t, d.CreatedAt)
		require.Empty(t, d.UpdatedAt)
		require.Equal(t, r.Price, d.Price)
		require.Equal(t, r.Title, d.Title)
		require.Equal(t, "", d.Description)
		require.True(t, strings.HasPrefix(d.ImageKey, "image/"+y+"/"+m))
		require.True(t, strings.HasSuffix(d.ImageKey, ".some mime"))
	})
}
