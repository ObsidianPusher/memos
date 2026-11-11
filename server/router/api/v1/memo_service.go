package v1

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/lithammer/shortuuid/v4"
	"github.com/usememos/gomark/ast"
	"github.com/usememos/gomark/parser"
	"github.com/usememos/gomark/parser/tokenizer"
	"github.com/usememos/gomark/renderer"
	"github.com/usememos/gomark/restore"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/plugin/webhook"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/server/runner/memopayload"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) CreateMemo(ctx context.Context, request *v1pb.CreateMemoRequest) (*v1pb.Memo, error) {
	var memo *v1pb.Memo
	err := s.Store.WithTransaction(ctx, func(stores *store.Store) error {
		scoped := *s
		scoped.Store = stores
		var err error
		memo, err = scoped.createMemo(ctx, request)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.DispatchMemoCreatedWebhook(ctx, memo); err != nil {
		slog.Warn("Failed to dispatch memo webhook", slog.Any("err", err))
	}
	return memo, nil
}

func (s *APIV1Service) createMemo(ctx context.Context, request *v1pb.CreateMemoRequest) (*v1pb.Memo, error) {
	if request == nil || request.Memo == nil {
		return nil, invalidArgErrf("memo is required")
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	create := &store.Memo{
		UID:        shortuuid.New(),
		CreatorID:  user.ID,
		Content:    request.Memo.Content,
		Visibility: convertVisibilityToStore(request.Memo.Visibility),
		Payload: &storepb.MemoPayload{
			Emotion: convertEmotionToStore(request.Memo.Emotion),
		},
	}
	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get workspace memo related setting", err)
	}
	if workspaceMemoRelatedSetting.DisallowPublicVisibility && create.Visibility == store.Public {
		return nil, grpcErr(codes.PermissionDenied, "disable public memos system setting is enabled", nil)
	}
	contentLengthLimit, err := s.getContentLengthLimit(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get content length limit", err)
	}
	if len(create.Content) > contentLengthLimit {
		return nil, invalidArgErrf("content too long (max %d characters)", contentLengthLimit)
	}
	if err := memopayload.RebuildMemoPayload(create); err != nil {
		return nil, grpcErr(codes.Internal, "failed to rebuild memo payload", err)
	}

	memo, err := s.Store.CreateMemo(ctx, create)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create memo", err)
	}
	if len(request.Memo.Resources) > 0 {
		_, err := s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{
			Name:      fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
			Resources: request.Memo.Resources,
		})
		if err != nil {
			return nil, err
		}
	}
	if len(request.Memo.Relations) > 0 {
		_, err := s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{
			Name:      fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
			Relations: request.Memo.Relations,
		})
		if err != nil {
			return nil, err
		}
	}

	memoMessage, err := s.convertMemoFromStore(ctx, memo)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to convert memo", err)
	}

	return memoMessage, nil
}

func (s *APIV1Service) ListMemos(ctx context.Context, request *v1pb.ListMemosRequest) (*v1pb.ListMemosResponse, error) {
	if request == nil {
		request = &v1pb.ListMemosRequest{}
	}
	memoFind := &store.FindMemo{
		// Exclude comments by default.
		ExcludeComments: true,
	}
	if err := s.buildMemoFindWithFilter(ctx, memoFind, request.OldFilter); err != nil {
		return nil, invalidArgErrf("failed to build find memos with filter: %v", err)
	}
	if request.Parent != "" && request.Parent != "users/-" {
		userID, err := ExtractUserIDFromName(request.Parent)
		if err != nil {
			return nil, invalidArgErrf("invalid parent: %v", err)
		}
		memoFind.CreatorID = &userID
		memoFind.OrderByPinned = true
	}
	if request.State == v1pb.State_ARCHIVED {
		state := store.Archived
		memoFind.RowStatus = &state
	} else {
		state := store.Normal
		memoFind.RowStatus = &state
	}
	if request.Direction == v1pb.Direction_ASC {
		memoFind.OrderByTimeAsc = true
	}
	if request.Filter != "" {
		if err := s.validateFilter(ctx, request.Filter); err != nil {
			return nil, invalidArgErrf("invalid filter: %v", err)
		}
		memoFind.Filter = &request.Filter
	}

	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if currentUser == nil {
		memoFind.VisibilityList = []store.Visibility{store.Public}
	} else {
		if memoFind.CreatorID == nil {
			internalFilter := fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
			if memoFind.Filter != nil {
				filter := fmt.Sprintf("(%s) && (%s)", *memoFind.Filter, internalFilter)
				memoFind.Filter = &filter
			} else {
				memoFind.Filter = &internalFilter
			}
		} else if *memoFind.CreatorID != currentUser.ID {
			memoFind.VisibilityList = []store.Visibility{store.Public, store.Protected}
		}
	}

	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get workspace memo related setting", err)
	}
	if workspaceMemoRelatedSetting.DisplayWithUpdateTime {
		memoFind.OrderByUpdatedTs = true
	}

	var limit, offset int
	if request.PageToken != "" {
		var pageToken v1pb.PageToken
		if err := unmarshalPageToken(request.PageToken, &pageToken); err != nil {
			return nil, invalidArgErrf("invalid page token: %v", err)
		}
		limit = int(pageToken.Limit)
		offset = int(pageToken.Offset)
	} else {
		limit = int(request.PageSize)
	}
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limitPlusOne := limit + 1
	memoFind.Limit = &limitPlusOne
	memoFind.Offset = &offset
	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memos", err)
	}

	memoMessages := []*v1pb.Memo{}
	nextPageToken := ""
	if len(memos) == limitPlusOne {
		memos = memos[:limit]
		nextPageToken, err = getPageToken(limit, offset+limit)
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get next page token", err)
		}
	}
	for _, memo := range memos {
		memoMessage, err := s.convertMemoFromStore(ctx, memo)
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to convert memo", err)
		}
		memoMessages = append(memoMessages, memoMessage)
	}

	response := &v1pb.ListMemosResponse{
		Memos:         memoMessages,
		NextPageToken: nextPageToken,
	}
	return response, nil
}

func (s *APIV1Service) GetMemo(ctx context.Context, request *v1pb.GetMemoRequest) (*v1pb.Memo, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
		UID: &memoUID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if memo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}
	if memo.Visibility != store.Public {
		user, err := s.GetCurrentUser(ctx)
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get user", err)
		}
		if user == nil {
			return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
		}
		if memo.Visibility == store.Private && memo.CreatorID != user.ID {
			return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
		}
	}

	memoMessage, err := s.convertMemoFromStore(ctx, memo)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to convert memo", err)
	}
	return memoMessage, nil
}

func (s *APIV1Service) UpdateMemo(ctx context.Context, request *v1pb.UpdateMemoRequest) (*v1pb.Memo, error) {
	var memo *v1pb.Memo
	err := s.Store.WithTransaction(ctx, func(stores *store.Store) error {
		scoped := *s
		scoped.Store = stores
		var err error
		memo, err = scoped.updateMemo(ctx, request)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.DispatchMemoUpdatedWebhook(ctx, memo); err != nil {
		slog.Warn("Failed to dispatch memo webhook", slog.Any("err", err))
	}
	return memo, nil
}

func (s *APIV1Service) updateMemo(ctx context.Context, request *v1pb.UpdateMemoRequest) (*v1pb.Memo, error) {
	if request == nil || request.Memo == nil {
		return nil, invalidArgErrf("memo is required")
	}
	memoUID, err := ExtractMemoUIDFromName(request.Memo.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	if request.UpdateMask == nil || len(request.UpdateMask.Paths) == 0 {
		return nil, invalidArgErrf("update mask is required")
	}

	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if memo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}

	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	// Only the creator or admin can update the memo.
	if memo.CreatorID != user.ID && !isSuperUser(user) {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	update := &store.UpdateMemo{
		ID: memo.ID,
	}
	for _, path := range request.UpdateMask.Paths {
		if path == "content" {
			contentLengthLimit, err := s.getContentLengthLimit(ctx)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to get content length limit", err)
			}
			if len(request.Memo.Content) > contentLengthLimit {
				return nil, invalidArgErrf("content too long (max %d characters)", contentLengthLimit)
			}
			memo.Content = request.Memo.Content
			if err := memopayload.RebuildMemoPayload(memo); err != nil {
				return nil, grpcErr(codes.Internal, "failed to rebuild memo payload", err)
			}
			update.Content = &memo.Content
			update.Payload = memo.Payload
		} else if path == "visibility" {
			workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to get workspace memo related setting", err)
			}
			visibility := convertVisibilityToStore(request.Memo.Visibility)
			if workspaceMemoRelatedSetting.DisallowPublicVisibility && visibility == store.Public {
				return nil, grpcErr(codes.PermissionDenied, "disable public memos system setting is enabled", nil)
			}
			update.Visibility = &visibility
		} else if path == "pinned" {
			update.Pinned = &request.Memo.Pinned
		} else if path == "emotion" {
			if memo.Payload == nil {
				memo.Payload = &storepb.MemoPayload{}
			}
			memo.Payload.Emotion = convertEmotionToStore(request.Memo.Emotion)
			update.Payload = memo.Payload
		} else if path == "state" {
			rowStatus := convertStateToStore(request.Memo.State)
			update.RowStatus = &rowStatus
		} else if path == "create_time" {
			createdTs := request.Memo.CreateTime.AsTime().Unix()
			update.CreatedTs = &createdTs
		} else if path == "update_time" {
			updatedTs := time.Now().Unix()
			if request.Memo.UpdateTime != nil {
				updatedTs = request.Memo.UpdateTime.AsTime().Unix()
			}
			update.UpdatedTs = &updatedTs
		} else if path == "display_time" {
			displayTs := request.Memo.DisplayTime.AsTime().Unix()
			memoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to get workspace memo related setting", err)
			}
			if memoRelatedSetting.DisplayWithUpdateTime {
				update.UpdatedTs = &displayTs
			} else {
				update.CreatedTs = &displayTs
			}
		} else if path == "resources" {
			_, err := s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{
				Name:      request.Memo.Name,
				Resources: request.Memo.Resources,
			})
			if err != nil {
				return nil, err
			}
		} else if path == "relations" {
			_, err := s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{
				Name:      request.Memo.Name,
				Relations: request.Memo.Relations,
			})
			if err != nil {
				return nil, err
			}
		}
	}

	if err = s.Store.UpdateMemo(ctx, update); err != nil {
		return nil, grpcErr(codes.Internal, "failed to update memo", err)
	}

	memo, err = s.Store.GetMemo(ctx, &store.FindMemo{
		ID: &memo.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	memoMessage, err := s.convertMemoFromStore(ctx, memo)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to convert memo", err)
	}

	return memoMessage, nil
}

func (s *APIV1Service) DeleteMemo(ctx context.Context, request *v1pb.DeleteMemoRequest) (*emptypb.Empty, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
		UID: &memoUID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if memo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}

	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	// Only the creator or admin can update the memo.
	if memo.CreatorID != user.ID && !isSuperUser(user) {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	if memoMessage, err := s.convertMemoFromStore(ctx, memo); err == nil {
		// Try to dispatch webhook when memo is deleted.
		if err := s.DispatchMemoDeletedWebhook(ctx, memoMessage); err != nil {
			slog.Warn("Failed to dispatch memo deleted webhook", slog.Any("err", err))
		}
	}

	if err = s.Store.DeleteMemo(ctx, &store.DeleteMemo{ID: memo.ID}); err != nil {
		return nil, grpcErr(codes.Internal, "failed to delete memo", err)
	}

	// Delete memo relation
	if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{MemoID: &memo.ID}); err != nil {
		return nil, grpcErr(codes.Internal, "failed to delete memo relations", err)
	}

	// Delete related resources.
	resources, err := s.Store.ListResources(ctx, &store.FindResource{MemoID: &memo.ID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list resources", err)
	}
	for _, resource := range resources {
		if err := s.Store.DeleteResource(ctx, &store.DeleteResource{ID: resource.ID}); err != nil {
			return nil, grpcErr(codes.Internal, "failed to delete resource", err)
		}
	}

	// Delete memo comments
	commentType := store.MemoRelationComment
	relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{RelatedMemoID: &memo.ID, Type: &commentType})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memo comments", err)
	}
	for _, relation := range relations {
		if err := s.Store.DeleteMemo(ctx, &store.DeleteMemo{ID: relation.MemoID}); err != nil {
			return nil, grpcErr(codes.Internal, "failed to delete memo comment", err)
		}
	}

	// Delete memo references
	referenceType := store.MemoRelationReference
	if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{RelatedMemoID: &memo.ID, Type: &referenceType}); err != nil {
		return nil, grpcErr(codes.Internal, "failed to delete memo references", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) CreateMemoComment(ctx context.Context, request *v1pb.CreateMemoCommentRequest) (*v1pb.Memo, error) {
	var memo *v1pb.Memo
	err := s.Store.WithTransaction(ctx, func(stores *store.Store) error {
		scoped := *s
		scoped.Store = stores
		var err error
		memo, err = scoped.createMemoComment(ctx, request)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.DispatchMemoCreatedWebhook(ctx, memo); err != nil {
		slog.Warn("Failed to dispatch memo webhook", slog.Any("err", err))
	}
	return memo, nil
}

func (s *APIV1Service) createMemoComment(ctx context.Context, request *v1pb.CreateMemoCommentRequest) (*v1pb.Memo, error) {
	if request == nil || request.Comment == nil {
		return nil, invalidArgErrf("comment is required")
	}
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if relatedMemo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}

	// Check parent visibility before creating any comment data.
	if _, err := s.GetMemo(ctx, &v1pb.GetMemoRequest{Name: request.Name}); err != nil {
		return nil, err
	}
	// Create the memo comment in the same transaction as its relation and inbox.
	memoComment, err := s.createMemo(ctx, &v1pb.CreateMemoRequest{Memo: request.Comment})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create memo", err)
	}
	memoUID, err = ExtractMemoUIDFromName(memoComment.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}

	// Build the relation between the comment memo and the original memo.
	_, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        memo.ID,
		RelatedMemoID: relatedMemo.ID,
		Type:          store.MemoRelationComment,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create memo relation", err)
	}
	creatorID, err := ExtractUserIDFromName(memoComment.Creator)
	if err != nil {
		return nil, invalidArgErrf("invalid memo creator")
	}
	if memoComment.Visibility != v1pb.Visibility_PRIVATE && creatorID != relatedMemo.CreatorID {
		activity, err := s.Store.CreateActivity(ctx, &store.Activity{
			CreatorID: creatorID,
			Type:      store.ActivityTypeMemoComment,
			Level:     store.ActivityLevelInfo,
			Payload: &storepb.ActivityPayload{
				MemoComment: &storepb.ActivityMemoCommentPayload{
					MemoId:        memo.ID,
					RelatedMemoId: relatedMemo.ID,
				},
			},
		})
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to create activity", err)
		}
		if _, err := s.Store.CreateInbox(ctx, &store.Inbox{
			SenderID:   creatorID,
			ReceiverID: relatedMemo.CreatorID,
			Status:     store.UNREAD,
			Message: &storepb.InboxMessage{
				Type:       storepb.InboxMessage_MEMO_COMMENT,
				ActivityId: &activity.ID,
			},
		}); err != nil {
			return nil, grpcErr(codes.Internal, "failed to create inbox", err)
		}
	}

	return memoComment, nil
}

func (s *APIV1Service) ListMemoComments(ctx context.Context, request *v1pb.ListMemoCommentsRequest) (*v1pb.ListMemoCommentsResponse, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if memo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}

	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	var memoFilter string
	if currentUser == nil {
		memoFilter = `visibility == "PUBLIC"`
	} else {
		memoFilter = fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
	}
	memoRelationComment := store.MemoRelationComment
	memoRelations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		RelatedMemoID: &memo.ID,
		Type:          &memoRelationComment,
		MemoFilter:    &memoFilter,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memo relations", err)
	}

	var memos []*v1pb.Memo
	for _, memoRelation := range memoRelations {
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
			ID: &memoRelation.MemoID,
		})
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get memo", err)
		}
		if memo != nil {
			memoMessage, err := s.convertMemoFromStore(ctx, memo)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to convert memo", err)
			}
			memos = append(memos, memoMessage)
		}
	}

	response := &v1pb.ListMemoCommentsResponse{
		Memos: memos,
	}
	return response, nil
}

func (s *APIV1Service) RenameMemoTag(ctx context.Context, request *v1pb.RenameMemoTagRequest) (*emptypb.Empty, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	memoFind := &store.FindMemo{
		CreatorID:       &user.ID,
		PayloadFind:     &store.FindMemoPayload{TagSearch: []string{request.OldTag}},
		ExcludeComments: true,
	}
	if (request.Parent) != "memos/-" {
		memoUID, err := ExtractMemoUIDFromName(request.Parent)
		if err != nil {
			return nil, invalidArgErrf("invalid memo name: %v", err)
		}
		memoFind.UID = &memoUID
	}

	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memos", err)
	}

	for _, memo := range memos {
		nodes, err := parser.Parse(tokenizer.Tokenize(memo.Content))
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to parse memo", err)
		}
		memopayload.TraverseASTNodes(nodes, func(node ast.Node) {
			if tag, ok := node.(*ast.Tag); ok && tag.Content == request.OldTag {
				tag.Content = request.NewTag
			}
		})
		memo.Content = restore.Restore(nodes)
		if err := memopayload.RebuildMemoPayload(memo); err != nil {
			return nil, grpcErr(codes.Internal, "failed to rebuild memo payload", err)
		}
		if err := s.Store.UpdateMemo(ctx, &store.UpdateMemo{
			ID:      memo.ID,
			Content: &memo.Content,
			Payload: memo.Payload,
		}); err != nil {
			return nil, grpcErr(codes.Internal, "failed to update memo", err)
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) DeleteMemoTag(ctx context.Context, request *v1pb.DeleteMemoTagRequest) (*emptypb.Empty, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	memoFind := &store.FindMemo{
		CreatorID:       &user.ID,
		PayloadFind:     &store.FindMemoPayload{TagSearch: []string{request.Tag}},
		ExcludeContent:  true,
		ExcludeComments: true,
	}
	if request.Parent != "memos/-" {
		memoUID, err := ExtractMemoUIDFromName(request.Parent)
		if err != nil {
			return nil, invalidArgErrf("invalid memo name: %v", err)
		}
		memoFind.UID = &memoUID
	}

	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memos", err)
	}

	for _, memo := range memos {
		if request.DeleteRelatedMemos {
			err := s.Store.DeleteMemo(ctx, &store.DeleteMemo{ID: memo.ID})
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to delete memo", err)
			}
		} else {
			archived := store.Archived
			err := s.Store.UpdateMemo(ctx, &store.UpdateMemo{
				ID:        memo.ID,
				RowStatus: &archived,
			})
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to update memo", err)
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) getContentLengthLimit(ctx context.Context) (int, error) {
	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return 0, grpcErr(codes.Internal, "failed to get workspace memo related setting", err)
	}
	return int(workspaceMemoRelatedSetting.ContentLengthLimit), nil
}

// DispatchMemoCreatedWebhook dispatches webhook when memo is created.
func (s *APIV1Service) DispatchMemoCreatedWebhook(ctx context.Context, memo *v1pb.Memo) error {
	return s.dispatchMemoRelatedWebhook(ctx, memo, "memos.memo.created")
}

// DispatchMemoUpdatedWebhook dispatches webhook when memo is updated.
func (s *APIV1Service) DispatchMemoUpdatedWebhook(ctx context.Context, memo *v1pb.Memo) error {
	return s.dispatchMemoRelatedWebhook(ctx, memo, "memos.memo.updated")
}

// DispatchMemoDeletedWebhook dispatches webhook when memo is deleted.
func (s *APIV1Service) DispatchMemoDeletedWebhook(ctx context.Context, memo *v1pb.Memo) error {
	return s.dispatchMemoRelatedWebhook(ctx, memo, "memos.memo.deleted")
}

func (s *APIV1Service) dispatchMemoRelatedWebhook(ctx context.Context, memo *v1pb.Memo, activityType string) error {
	creatorID, err := ExtractUserIDFromName(memo.Creator)
	if err != nil {
		return invalidArgErrf("invalid memo creator")
	}
	webhooks, err := s.Store.ListWebhooks(ctx, &store.FindWebhook{
		CreatorID: &creatorID,
	})
	if err != nil {
		return grpcErr(codes.Internal, "failed to list webhooks", err)
	}
	for _, hook := range webhooks {
		payload, err := convertMemoToWebhookPayload(memo)
		if err != nil {
			return fmt.Errorf("failed to convert memo to webhook payload: %w", err)
		}
		payload.ActivityType = activityType
		payload.Url = hook.URL

		// Use asynchronous webhook dispatch
		webhook.PostAsync(payload)
	}
	return nil
}

func convertMemoToWebhookPayload(memo *v1pb.Memo) (*v1pb.WebhookRequestPayload, error) {
	creatorID, err := ExtractUserIDFromName(memo.Creator)
	if err != nil {
		return nil, fmt.Errorf("invalid memo creator: %w", err)
	}
	return &v1pb.WebhookRequestPayload{
		Creator:    fmt.Sprintf("%s%d", UserNamePrefix, creatorID),
		CreateTime: timestamppb.New(time.Now()),
		Memo:       memo,
	}, nil
}

func getMemoContentSnippet(content string) (string, error) {
	nodes, err := parser.Parse(tokenizer.Tokenize(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse content: %w", err)
	}

	plainText := renderer.NewStringRenderer().Render(nodes)
	if len(plainText) > 64 {
		return substring(plainText, 64) + "...", nil
	}
	return plainText, nil
}

func substring(s string, length int) string {
	if length <= 0 {
		return ""
	}

	runeCount := 0
	byteIndex := 0
	for byteIndex < len(s) {
		_, size := utf8.DecodeRuneInString(s[byteIndex:])
		byteIndex += size
		runeCount++
		if runeCount == length {
			break
		}
	}

	return s[:byteIndex]
}
