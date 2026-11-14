package v1

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func TestResourceFileThumbnailBounds(t *testing.T) {
	for _, storageType := range []storepb.ResourceStorageType{storepb.ResourceStorageType_RESOURCE_STORAGE_TYPE_UNSPECIFIED, storepb.ResourceStorageType_LOCAL} {
		for _, size := range []image.Point{{X: 2400, Y: 1200}, {X: 1200, Y: 2400}, {X: 100, Y: 50}} {
			t.Run(storageType.String()+"/"+size.String(), func(t *testing.T) {
				s, ctx, user := testAPI(t)
				s.Profile = &profile.Profile{Data: t.TempDir()}
				memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "photo", CreatorID: user.ID, Visibility: store.Public})
				require.NoError(t, err)
				img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
				for y := 0; y < size.Y; y++ {
					for x := 0; x < size.X; x++ {
						img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x * y), A: 255})
					}
				}
				var encoded bytes.Buffer
				require.NoError(t, jpeg.Encode(&encoded, img, nil))
				resource := &store.Resource{UID: "photo", CreatorID: user.ID, MemoID: &memo.ID, Filename: "photo.jpg", Type: "image/jpeg", StorageType: storageType, Blob: encoded.Bytes()}
				if storageType == storepb.ResourceStorageType_LOCAL {
					resource.Reference = "photo.jpg"
					require.NoError(t, os.WriteFile(filepath.Join(s.Profile.Data, resource.Reference), encoded.Bytes(), 0600))
					resource.Blob = nil
				}
				resource, err = s.Store.CreateResource(ctx, resource)
				require.NoError(t, err)
				// Existing oversized cache entries must not bypass the new thumbnail bounds.
				require.NoError(t, os.MkdirAll(filepath.Join(s.Profile.Data, ThumbnailCacheFolder), 0700))
				require.NoError(t, os.WriteFile(filepath.Join(s.Profile.Data, ThumbnailCacheFolder, fmt.Sprintf("%d.jpg", resource.ID)), encoded.Bytes(), 0600))
				request := httptest.NewRequest(http.MethodGet, "/file/resources/photo/photo.jpg?thumbnail=true", nil)
				response := httptest.NewRecorder()
				c := echo.New().NewContext(request, response)
				c.SetParamNames("uid", "filename")
				c.SetParamValues("photo", "photo.jpg")
				require.NoError(t, s.handleResourceFile(c))
				require.Equal(t, http.StatusOK, response.Code)
				decoded, err := jpeg.Decode(bytes.NewReader(response.Body.Bytes()))
				require.NoError(t, err)
				require.LessOrEqual(t, decoded.Bounds().Dx(), 512)
				require.LessOrEqual(t, decoded.Bounds().Dy(), 512)
				if size.X == 100 {
					require.Equal(t, size, image.Pt(decoded.Bounds().Dx(), decoded.Bounds().Dy()))
				} else {
					require.Less(t, response.Body.Len(), encoded.Len()/2, "list thumbnail should not transfer the original photo")
				}
				t.Logf("image %s: original=%d bytes thumbnail=%d bytes", size, encoded.Len(), response.Body.Len())
				original := httptest.NewRecorder()
				originalContext := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/file/resources/photo/photo.jpg", nil), original)
				originalContext.SetParamNames("uid", "filename")
				originalContext.SetParamValues("photo", "photo.jpg")
				require.NoError(t, s.handleResourceFile(originalContext))
				require.Equal(t, encoded.Bytes(), original.Body.Bytes(), "opening the image must still return the original")
				if storageType == storepb.ResourceStorageType_LOCAL {
					require.NoError(t, os.Remove(filepath.Join(s.Profile.Data, resource.Reference)))
					cached := httptest.NewRecorder()
					cachedContext := echo.New().NewContext(request, cached)
					cachedContext.SetParamNames("uid", "filename")
					cachedContext.SetParamValues("photo", "photo.jpg")
					require.NoError(t, s.handleResourceFile(cachedContext))
					require.Equal(t, response.Body.Bytes(), cached.Body.Bytes(), "cache hits must not regenerate the image")
				}
			})
		}
	}
}

func TestDatabaseResourceFileRange(t *testing.T) {
	s, ctx, user := testAPI(t)
	memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "video", CreatorID: user.ID, Visibility: store.Public})
	require.NoError(t, err)
	_, err = s.Store.CreateResource(ctx, &store.Resource{UID: "video", CreatorID: user.ID, MemoID: &memo.ID, Filename: "video.mp4", Type: "video/mp4", Blob: []byte("0123456789")})
	require.NoError(t, err)
	for _, tc := range []struct {
		method, byteRange  string
		status             int
		body, contentRange string
	}{
		{http.MethodGet, "bytes=2-5", http.StatusPartialContent, "2345", "bytes 2-5/10"},
		{http.MethodGet, "bytes=-3", http.StatusPartialContent, "789", "bytes 7-9/10"},
		{http.MethodGet, "bytes=20-", http.StatusRequestedRangeNotSatisfiable, "", "bytes */10"},
		{http.MethodHead, "", http.StatusOK, "", ""},
		{http.MethodGet, "", http.StatusOK, "0123456789", ""},
	} {
		t.Run(tc.method+tc.byteRange, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "/file/resources/video/video.mp4", nil)
			if tc.byteRange != "" {
				request.Header.Set("Range", tc.byteRange)
			}
			response := httptest.NewRecorder()
			c := echo.New().NewContext(request, response)
			c.SetParamNames("uid", "filename")
			c.SetParamValues("video", "video.mp4")
			require.NoError(t, s.handleResourceFile(c))
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.contentRange, response.Header().Get("Content-Range"))
			if tc.status != http.StatusRequestedRangeNotSatisfiable {
				require.Equal(t, tc.body, response.Body.String())
			}
		})
	}
}
