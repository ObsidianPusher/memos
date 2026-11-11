package v1

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) SetMemoRelations(ctx context.Context, request *v1pb.SetMemoRelationsRequest) (*emptypb.Empty, error) {
	err := s.Store.WithTransaction(ctx, func(stores *store.Store) error {
		scoped := *s
		scoped.Store = stores
		return scoped.setMemoRelations(ctx, request)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) setMemoRelations(ctx context.Context, request *v1pb.SetMemoRelationsRequest) error {
	if request == nil {
		return invalidArgErrf("request is required")
	}
	memo, err := s.getOwnedMemo(ctx, request.Name)
	if err != nil {
		return err
	}
	referenceType := store.MemoRelationReference
	// Delete all reference relations first.
	if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{
		MemoID: &memo.ID,
		Type:   &referenceType,
	}); err != nil {
		return grpcErr(codes.Internal, "failed to delete memo relation", err)
	}

	for _, relation := range request.Relations {
		if relation == nil || relation.RelatedMemo == nil {
			return invalidArgErrf("related memo is required")
		}
		// Ignore reflexive relations.
		if request.Name == relation.RelatedMemo.Name {
			continue
		}
		// Ignore comment relations as there's no need to update a comment's relation.
		// Inserting/Deleting a comment is handled elsewhere.
		if relation.Type == v1pb.MemoRelation_COMMENT {
			continue
		}
		relatedMemoUID, err := ExtractMemoUIDFromName(relation.RelatedMemo.Name)
		if err != nil {
			return invalidArgErrf("invalid related memo name: %v", err)
		}
		relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &relatedMemoUID})
		if err != nil {
			return grpcErr(codes.Internal, "failed to get related memo", err)
		}
		if relatedMemo == nil {
			return grpcErr(codes.NotFound, "related memo not found", nil)
		}
		if _, err := s.GetMemo(ctx, &v1pb.GetMemoRequest{Name: relation.RelatedMemo.Name}); err != nil {
			return err
		}
		if _, err := s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
			MemoID:        memo.ID,
			RelatedMemoID: relatedMemo.ID,
			Type:          convertMemoRelationTypeToStore(relation.Type),
		}); err != nil {
			return grpcErr(codes.Internal, "failed to upsert memo relation", err)
		}
	}

	return nil
}

func (s *APIV1Service) ListMemoRelations(ctx context.Context, request *v1pb.ListMemoRelationsRequest) (*v1pb.ListMemoRelationsResponse, error) {
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
	relationList := []*v1pb.MemoRelation{}
	tempList, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		MemoID:     &memo.ID,
		MemoFilter: &memoFilter,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memo relations", err)
	}
	for _, raw := range tempList {
		relation, err := s.convertMemoRelationFromStore(ctx, raw)
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to convert memo relation", err)
		}
		relationList = append(relationList, relation)
	}
	tempList, err = s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		RelatedMemoID: &memo.ID,
		MemoFilter:    &memoFilter,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list memo relations", err)
	}
	for _, raw := range tempList {
		relation, err := s.convertMemoRelationFromStore(ctx, raw)
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to convert memo relation", err)
		}
		relationList = append(relationList, relation)
	}

	response := &v1pb.ListMemoRelationsResponse{
		Relations: relationList,
	}
	return response, nil
}

func (s *APIV1Service) convertMemoRelationFromStore(ctx context.Context, memoRelation *store.MemoRelation) (*v1pb.MemoRelation, error) {
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &memoRelation.MemoID})
	if err != nil {
		return nil, err
	}
	memoSnippet, err := getMemoContentSnippet(memo.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to get memo content snippet: %w", err)
	}
	relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &memoRelation.RelatedMemoID})
	if err != nil {
		return nil, err
	}
	relatedMemoSnippet, err := getMemoContentSnippet(relatedMemo.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to get related memo content snippet: %w", err)
	}
	return &v1pb.MemoRelation{
		Memo: &v1pb.MemoRelation_Memo{
			Name:    fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
			Uid:     memo.UID,
			Snippet: memoSnippet,
		},
		RelatedMemo: &v1pb.MemoRelation_Memo{
			Name:    fmt.Sprintf("%s%s", MemoNamePrefix, relatedMemo.UID),
			Uid:     relatedMemo.UID,
			Snippet: relatedMemoSnippet,
		},
		Type: convertMemoRelationTypeFromStore(memoRelation.Type),
	}, nil
}

func convertMemoRelationTypeFromStore(relationType store.MemoRelationType) v1pb.MemoRelation_Type {
	switch relationType {
	case store.MemoRelationReference:
		return v1pb.MemoRelation_REFERENCE
	case store.MemoRelationComment:
		return v1pb.MemoRelation_COMMENT
	default:
		return v1pb.MemoRelation_TYPE_UNSPECIFIED
	}
}

func convertMemoRelationTypeToStore(relationType v1pb.MemoRelation_Type) store.MemoRelationType {
	switch relationType {
	case v1pb.MemoRelation_REFERENCE:
		return store.MemoRelationReference
	case v1pb.MemoRelation_COMMENT:
		return store.MemoRelationComment
	default:
		return store.MemoRelationReference
	}
}
