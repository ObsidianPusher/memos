package v1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/gomark/parser"
	"github.com/usememos/gomark/parser/tokenizer"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) convertMemoFromStore(ctx context.Context, memo *store.Memo) (*v1pb.Memo, error) {
	displayTs := memo.CreatedTs
	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace memo related setting: %w", err)
	}
	if workspaceMemoRelatedSetting.DisplayWithUpdateTime {
		displayTs = memo.UpdatedTs
	}

	name := fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID)
	memoMessage := &v1pb.Memo{
		Name:        name,
		State:       convertStateFromStore(memo.RowStatus),
		Creator:     fmt.Sprintf("%s%d", UserNamePrefix, memo.CreatorID),
		CreateTime:  timestamppb.New(time.Unix(memo.CreatedTs, 0)),
		UpdateTime:  timestamppb.New(time.Unix(memo.UpdatedTs, 0)),
		DisplayTime: timestamppb.New(time.Unix(displayTs, 0)),
		Content:     memo.Content,
		Visibility:  convertVisibilityFromStore(memo.Visibility),
		Pinned:      memo.Pinned,
	}
	if memo.Payload != nil {
		memoMessage.Tags = memo.Payload.Tags
		memoMessage.Property = convertMemoPropertyFromStore(memo.Payload.Property)
		memoMessage.Emotion = convertEmotionFromStore(memo.Payload.Emotion)
	}
	if memo.ParentID != nil {
		parent, err := s.Store.GetMemo(ctx, &store.FindMemo{
			ID:             memo.ParentID,
			ExcludeContent: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get parent memo: %w", err)
		}
		if parent == nil {
			return nil, errors.New("failed to get parent memo: memo not found")
		}
		parentName := fmt.Sprintf("%s%s", MemoNamePrefix, parent.UID)
		memoMessage.Parent = &parentName
	}

	listMemoRelationsResponse, err := s.ListMemoRelations(ctx, &v1pb.ListMemoRelationsRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to list memo relations: %w", err)
	}
	memoMessage.Relations = listMemoRelationsResponse.Relations

	listMemoResourcesResponse, err := s.ListMemoResources(ctx, &v1pb.ListMemoResourcesRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to list memo resources: %w", err)
	}
	memoMessage.Resources = listMemoResourcesResponse.Resources

	listMemoReactionsResponse, err := s.ListMemoReactions(ctx, &v1pb.ListMemoReactionsRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to list memo reactions: %w", err)
	}
	memoMessage.Reactions = listMemoReactionsResponse.Reactions

	nodes, err := parser.Parse(tokenizer.Tokenize(memo.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}
	memoMessage.Nodes = convertFromASTNodes(nodes)

	snippet, err := getMemoContentSnippet(memo.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to get memo content snippet: %w", err)
	}
	memoMessage.Snippet = snippet

	return memoMessage, nil
}

func convertMemoPropertyFromStore(property *storepb.MemoPayload_Property) *v1pb.Memo_Property {
	if property == nil {
		return nil
	}
	return &v1pb.Memo_Property{
		HasLink:            property.HasLink,
		HasTaskList:        property.HasTaskList,
		HasCode:            property.HasCode,
		HasIncompleteTasks: property.HasIncompleteTasks,
	}
}

func convertVisibilityFromStore(visibility store.Visibility) v1pb.Visibility {
	switch visibility {
	case store.Private:
		return v1pb.Visibility_PRIVATE
	case store.Protected:
		return v1pb.Visibility_PROTECTED
	case store.Public:
		return v1pb.Visibility_PUBLIC
	default:
		return v1pb.Visibility_VISIBILITY_UNSPECIFIED
	}
}

func convertVisibilityToStore(visibility v1pb.Visibility) store.Visibility {
	switch visibility {
	case v1pb.Visibility_PRIVATE:
		return store.Private
	case v1pb.Visibility_PROTECTED:
		return store.Protected
	case v1pb.Visibility_PUBLIC:
		return store.Public
	default:
		return store.Private
	}
}

func convertEmotionFromStore(emotion storepb.MemoPayload_Emotion) v1pb.Emotion {
	switch emotion {
	case storepb.MemoPayload_TERRIBLE:
		return v1pb.Emotion_TERRIBLE
	case storepb.MemoPayload_BAD:
		return v1pb.Emotion_BAD
	case storepb.MemoPayload_OKAY:
		return v1pb.Emotion_OKAY
	case storepb.MemoPayload_GOOD:
		return v1pb.Emotion_GOOD
	case storepb.MemoPayload_AWESOME:
		return v1pb.Emotion_AWESOME
	default:
		return v1pb.Emotion_EMOTION_UNSPECIFIED
	}
}

func convertEmotionToStore(emotion v1pb.Emotion) storepb.MemoPayload_Emotion {
	switch emotion {
	case v1pb.Emotion_TERRIBLE:
		return storepb.MemoPayload_TERRIBLE
	case v1pb.Emotion_BAD:
		return storepb.MemoPayload_BAD
	case v1pb.Emotion_OKAY:
		return storepb.MemoPayload_OKAY
	case v1pb.Emotion_GOOD:
		return storepb.MemoPayload_GOOD
	case v1pb.Emotion_AWESOME:
		return storepb.MemoPayload_AWESOME
	default:
		return storepb.MemoPayload_EMOTION_UNSPECIFIED
	}
}
