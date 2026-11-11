package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func TestResourceDownloadAccess(t *testing.T) {
	for _, tc := range []struct {
		name, visibility, viewer string
		orphan                   bool
		want                     int
	}{
		{name: "unbound anonymous", want: 401},
		{name: "unbound stranger", viewer: "stranger", want: 403},
		{name: "unbound owner", viewer: "owner", want: 200},
		{name: "orphan anonymous", orphan: true, want: 401},
		{name: "orphan stranger", orphan: true, viewer: "stranger", want: 403},
		{name: "orphan owner", orphan: true, viewer: "owner", want: 200},
		{name: "public anonymous", visibility: "PUBLIC", want: 200},
		{name: "protected anonymous", visibility: "PROTECTED", want: 401},
		{name: "protected member", visibility: "PROTECTED", viewer: "stranger", want: 200},
		{name: "private stranger", visibility: "PRIVATE", viewer: "stranger", want: 403},
		{name: "private owner", visibility: "PRIVATE", viewer: "owner", want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx, owner := testAPI(t)
			resource := &store.Resource{UID: "download", CreatorID: owner.ID, Filename: "note.txt", Type: "text/plain", Blob: []byte("private content")}
			if tc.visibility != "" {
				memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "parent", CreatorID: owner.ID, Visibility: store.Visibility(tc.visibility)})
				require.NoError(t, err)
				resource.MemoID = &memo.ID
			} else if tc.orphan {
				id := int32(999)
				resource.MemoID = &id
			}
			_, err := s.Store.CreateResource(ctx, resource)
			require.NoError(t, err)
			viewerCtx := context.Background()
			request := httptest.NewRequest(http.MethodGet, "/file/resources/download/note.txt", nil)
			if tc.viewer != "" {
				viewer := owner
				if tc.viewer == "stranger" {
					viewer, err = s.Store.CreateUser(ctx, &store.User{Username: "stranger", Role: store.RoleUser})
					require.NoError(t, err)
				}
				token, err := GenerateAccessToken(viewer.Username, viewer.ID, time.Time{}, []byte(s.Secret))
				require.NoError(t, err)
				require.NoError(t, s.UpsertAccessTokenToStore(ctx, viewer, token, "test"))
				viewerCtx = context.WithValue(viewerCtx, usernameContextKey, viewer.Username)
				request.Header.Set("Authorization", "Bearer "+token)
			}
			_, err = s.GetResourceBinary(viewerCtx, &v1pb.GetResourceBinaryRequest{Name: "resources/download"})
			if tc.want == 200 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			response := httptest.NewRecorder()
			c := echo.New().NewContext(request, response)
			c.SetParamNames("uid", "filename")
			c.SetParamValues("download", "note.txt")
			require.NoError(t, s.handleResourceFile(c))
			require.Equal(t, tc.want, response.Code)
			if tc.want == 200 {
				require.Equal(t, "private content", response.Body.String())
			}
		})
	}
}

func TestResourceDownloadRejectsInvalidCredentials(t *testing.T) {
	s, ctx, owner := testAPI(t)
	_, err := s.Store.CreateResource(ctx, &store.Resource{UID: "upload", CreatorID: owner.ID, Filename: "file.txt"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/file/resources/upload/file.txt", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response := httptest.NewRecorder()
	c := echo.New().NewContext(request, response)
	c.SetParamNames("uid", "filename")
	c.SetParamValues("upload", "file.txt")
	require.NoError(t, s.handleResourceFile(c))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}
