//go:build unit

package dto

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

func TestFeedbackCreateRequest_Validate(t *testing.T) {
	valid := func() *FeedbackCreateRequest {
		return &FeedbackCreateRequest{
			ProductID: uuid.NewV7().String(),
			UserID:    uuid.NewV7().String(),
			Score:     3,
		}
	}

	t.Run("valid without description", func(t *testing.T) {
		require.NoError(t, valid().Validate())
	})

	t.Run("valid with description", func(t *testing.T) {
		r := valid()
		r.Description = new("ok")
		require.NoError(t, r.Validate())
	})

	t.Run("score bounds", func(t *testing.T) {
		for _, score := range []int8{1, 5} {
			r := valid()
			r.Score = score
			require.NoError(t, r.Validate(), score)
		}

		for _, score := range []int8{0, -1, 6} {
			r := valid()
			r.Score = score
			require.ErrorIs(t, r.Validate(), ErrInvalidJSON, score)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		r := valid()
		r.UserID = ""
		require.ErrorIs(t, r.Validate(), ErrInvalidJSON)
	})

	t.Run("product id not uuid", func(t *testing.T) {
		r := valid()
		r.ProductID = "bad"
		require.ErrorIs(t, r.Validate(), ErrInvalidJSON)
	})

	t.Run("blank description", func(t *testing.T) {
		r := valid()
		r.Description = new(" \t ")
		require.ErrorIs(t, r.Validate(), ErrInvalidData)
	})
}

func TestFeedbackCreateRequest_ToDomain(t *testing.T) {
	productID, userID := uuid.NewV7(), uuid.NewV7()

	t.Run("ok", func(t *testing.T) {
		r := &FeedbackCreateRequest{ProductID: productID.String(), UserID: userID.String(), Score: 5, Description: new("d")}

		f, err := r.ToDomain()
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil(), f.ID)
		require.Equal(t, productID, f.ProductID)
		require.Equal(t, userID, f.UserID)
		require.Equal(t, int8(5), f.Score)
		require.Equal(t, "d", *f.Description)
	})

	t.Run("bad product id", func(t *testing.T) {
		_, err := (&FeedbackCreateRequest{ProductID: "bad", UserID: userID.String()}).ToDomain()
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("bad user id", func(t *testing.T) {
		_, err := (&FeedbackCreateRequest{ProductID: productID.String(), UserID: "bad"}).ToDomain()
		require.ErrorIs(t, err, ErrInvalidData)
	})
}

func TestLikeAction_Valid(t *testing.T) {
	require.True(t, LikeAdd.Valid())
	require.True(t, LikeRemove.Valid())
	require.False(t, LikeAction("toggle").Valid())
	require.False(t, LikeAction("").Valid())
}

func TestImageFormat_Ext(t *testing.T) {
	ext, ok := JPEG.Ext()
	require.True(t, ok)
	require.Equal(t, ".jpg", ext)

	ext, ok = PNG.Ext()
	require.True(t, ok)
	require.Equal(t, ".png", ext)

	_, ok = ImageFormat("gif").Ext()
	require.False(t, ok)
}
