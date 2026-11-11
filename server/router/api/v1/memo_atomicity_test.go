package v1

import (
	"context"
	"fmt"
	storepb "github.com/usememos/memos/proto/gen/store"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testAPI(t *testing.T) (*APIV1Service, context.Context, *store.User) {
	t.Helper()
	stores := teststore.NewTestingStore(context.Background(), t)
	t.Cleanup(func() { require.NoError(t, stores.Close()) })
	user, err := stores.CreateUser(context.Background(), &store.User{Username: "owner", Role: store.RoleUser})
	require.NoError(t, err)
	return &APIV1Service{Store: stores}, context.WithValue(context.Background(), usernameContextKey, user.Username), user
}

func TestCreateMemoRollsBackWhenRelationWriteFails(t *testing.T) {
	s, ctx, user := testAPI(t)
	target, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "target", CreatorID: user.ID, Visibility: store.Private})
	require.NoError(t, err)
	_, err = s.Store.GetDriver().GetDB().ExecContext(ctx, `CREATE TRIGGER fail_relation BEFORE INSERT ON memo_relation BEGIN SELECT RAISE(ABORT, 'injected relation failure'); END`)
	require.NoError(t, err)
	_, err = s.CreateMemo(ctx, &v1pb.CreateMemoRequest{Memo: &v1pb.Memo{Content: "must roll back", Relations: []*v1pb.MemoRelation{{Type: v1pb.MemoRelation_REFERENCE, RelatedMemo: &v1pb.MemoRelation_Memo{Name: fmt.Sprintf("memos/%s", target.UID)}}}}})
	require.Error(t, err)
	memos, err := s.Store.ListMemos(ctx, &store.FindMemo{})
	require.NoError(t, err)
	require.Len(t, memos, 1, "failed creation must not leave a memo")
}

func TestCreateMemoRollsBackResourceBinding(t *testing.T) {
	s, ctx, user := testAPI(t)
	resource, err := s.Store.CreateResource(ctx, &store.Resource{UID: "upload", CreatorID: user.ID, Filename: "file.txt"})
	require.NoError(t, err)
	_, err = s.CreateMemo(ctx, &v1pb.CreateMemoRequest{Memo: &v1pb.Memo{Content: "invalid relation", Resources: []*v1pb.Resource{{Name: "resources/upload"}}, Relations: []*v1pb.MemoRelation{{RelatedMemo: &v1pb.MemoRelation_Memo{Name: "invalid"}}}}})
	require.Error(t, err)
	persisted, err := s.Store.GetResource(ctx, &store.FindResource{ID: &resource.ID})
	require.NoError(t, err)
	require.Nil(t, persisted.MemoID, "failed creation must leave upload unbound")
	memos, err := s.Store.ListMemos(ctx, &store.FindMemo{})
	require.NoError(t, err)
	require.Empty(t, memos)
}

func TestSetMemoResourcesRejectsOtherUsersResource(t *testing.T) {
	s, ctx, user := testAPI(t)
	memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "mine", CreatorID: user.ID, Visibility: store.Private})
	require.NoError(t, err)
	_, err = s.Store.CreateResource(ctx, &store.Resource{UID: "theirs", CreatorID: user.ID + 1, Filename: "private.txt"})
	require.NoError(t, err)
	_, err = s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{Name: "memos/" + memo.UID, Resources: []*v1pb.Resource{{Name: "resources/theirs"}}})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestSetMemoResourcesMissingTarget(t *testing.T) {
	s, ctx, _ := testAPI(t)
	require.NotPanics(t, func() {
		_, err := s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{Name: "memos/missing"})
		require.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestMemoSettersValidateTargetsAndOwners(t *testing.T) {
	for _, kind := range []string{"resources", "relations"} {
		t.Run(kind, func(t *testing.T) {
			s, ctx, user := testAPI(t)
			_, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "theirs", CreatorID: user.ID + 1, Visibility: store.Private})
			require.NoError(t, err)
			call := func(name string) error {
				if kind == "resources" {
					_, err := s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{Name: name})
					return err
				}
				_, err := s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{Name: name})
				return err
			}
			require.NotPanics(t, func() { require.Equal(t, codes.NotFound, status.Code(call("memos/missing"))) })
			require.Equal(t, codes.PermissionDenied, status.Code(call("memos/theirs")))
		})
	}
}

func TestSetMemoRelationsRollsBackInvalidReplacement(t *testing.T) {
	s, ctx, user := testAPI(t)
	memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "source", CreatorID: user.ID, Visibility: store.Private})
	require.NoError(t, err)
	target, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "target", CreatorID: user.ID, Visibility: store.Private})
	require.NoError(t, err)
	_, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{MemoID: memo.ID, RelatedMemoID: target.ID, Type: store.MemoRelationReference})
	require.NoError(t, err)
	require.NotPanics(t, func() {
		_, err = s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{Name: "memos/source", Relations: []*v1pb.MemoRelation{{RelatedMemo: &v1pb.MemoRelation_Memo{Name: "memos/missing"}}}})
		require.Equal(t, codes.NotFound, status.Code(err))
	})
	relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{MemoID: &memo.ID})
	require.NoError(t, err)
	require.Len(t, relations, 1)
}

func TestUpdateMemoRollsBackBeforeDeletingLocalFile(t *testing.T) {
	s, ctx, user := testAPI(t)
	memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "source", CreatorID: user.ID, Content: "original", Visibility: store.Private})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "upload.txt")
	require.NoError(t, os.WriteFile(path, []byte("must survive rollback"), 0600))
	resource, err := s.Store.CreateResource(ctx, &store.Resource{UID: "upload", CreatorID: user.ID, Filename: "upload.txt", MemoID: &memo.ID, StorageType: storepb.ResourceStorageType_LOCAL, Reference: path})
	require.NoError(t, err)
	_, err = s.UpdateMemo(ctx, &v1pb.UpdateMemoRequest{Memo: &v1pb.Memo{Name: "memos/source", Content: "changed", Relations: []*v1pb.MemoRelation{{RelatedMemo: &v1pb.MemoRelation_Memo{Name: "invalid"}}}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"resources", "relations", "content"}}})
	require.Error(t, err)
	persisted, err := s.Store.GetResource(ctx, &store.FindResource{ID: &resource.ID})
	require.NoError(t, err)
	require.NotNil(t, persisted)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "must survive rollback", string(content))
	// Successful removal commits metadata and then cleans up the local file.
	_, err = s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{Name: "memos/source"})
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))
	persisted, err = s.Store.GetResource(ctx, &store.FindResource{ID: &resource.ID})
	require.NoError(t, err)
	require.Nil(t, persisted)
}

func TestMemoSettersAllowAdministrator(t *testing.T) {
	s, ctx, user := testAPI(t)
	role := store.RoleAdmin
	_, err := s.Store.UpdateUser(ctx, &store.UpdateUser{ID: user.ID, Role: &role})
	require.NoError(t, err)
	_, err = s.Store.CreateMemo(ctx, &store.Memo{UID: "theirs", CreatorID: user.ID + 1, Visibility: store.Public})
	require.NoError(t, err)
	_, err = s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{Name: "memos/theirs"})
	require.NoError(t, err)
	_, err = s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{Name: "memos/theirs"})
	require.NoError(t, err)
}

func TestCommittedResourceRemovalDeletesS3Object(t *testing.T) {
	s, ctx, user := testAPI(t)
	deleted := make(chan string, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer remote.Close()
	memo, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "source", CreatorID: user.ID, Visibility: store.Private})
	require.NoError(t, err)
	_, err = s.Store.CreateResource(ctx, &store.Resource{UID: "s3-upload", CreatorID: user.ID, MemoID: &memo.ID, Filename: "file.txt", StorageType: storepb.ResourceStorageType_S3, Payload: &storepb.ResourcePayload{Payload: &storepb.ResourcePayload_S3Object_{S3Object: &storepb.ResourcePayload_S3Object{Key: "file.txt", S3Config: &storepb.StorageS3Config{Endpoint: remote.URL, Region: "us-east-1", Bucket: "uploads", UsePathStyle: true, AccessKeyId: "test", AccessKeySecret: "test"}}}}})
	require.NoError(t, err)
	_, err = s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{Name: "memos/source"})
	require.NoError(t, err)
	select {
	case path := <-deleted:
		require.Equal(t, "/uploads/file.txt", path)
	default:
		t.Fatal("S3 object was not deleted after commit")
	}
}

func TestCommentCreationRollsBackRelationFailure(t *testing.T) {
	s, ctx, user := testAPI(t)
	_, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "parent", CreatorID: user.ID, Visibility: store.Public})
	require.NoError(t, err)
	_, err = s.Store.GetDriver().GetDB().ExecContext(ctx, `CREATE TRIGGER fail_comment BEFORE INSERT ON memo_relation BEGIN SELECT RAISE(ABORT,'injected failure'); END`)
	require.NoError(t, err)
	_, err = s.CreateMemoComment(ctx, &v1pb.CreateMemoCommentRequest{Name: "memos/parent", Comment: &v1pb.Memo{Content: "comment"}})
	require.Error(t, err)
	memos, err := s.Store.ListMemos(ctx, &store.FindMemo{})
	require.NoError(t, err)
	require.Len(t, memos, 1)
}

func TestCommentCreationRejectsInaccessibleParent(t *testing.T) {
	s, ctx, user := testAPI(t)
	_, err := s.Store.CreateMemo(ctx, &store.Memo{UID: "private", CreatorID: user.ID + 1, Visibility: store.Private})
	require.NoError(t, err)
	_, err = s.CreateMemoComment(ctx, &v1pb.CreateMemoCommentRequest{Name: "memos/private", Comment: &v1pb.Memo{Content: "comment"}})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
