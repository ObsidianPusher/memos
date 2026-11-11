package memopayload

import (
	"context"
	"log/slog"

	"github.com/pkg/errors"
	"github.com/usememos/gomark/ast"
	"github.com/usememos/gomark/parser"
	"github.com/usememos/gomark/parser/tokenizer"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

type Runner struct {
	Store *store.Store
}

func NewRunner(store *store.Store) *Runner {
	return &Runner{
		Store: store,
	}
}

// RunOnce rebuilds the payload of all memos.
func (r *Runner) RunOnce(ctx context.Context) {
	// Process memos in batches to avoid loading all memos into memory at once
	const batchSize = 100
	offset := 0
	processed := 0
	successCount := 0
	failedCount := 0

	for {
		limit := batchSize
		memos, err := r.Store.ListMemos(ctx, &store.FindMemo{
			Limit:  &limit,
			Offset: &offset,
		})
		if err != nil {
			slog.Error("failed to list memos", "err", err)
			return
		}

		// Break if no more memos
		if len(memos) == 0 {
			break
		}

		// Process batch
		for _, memo := range memos {
			if err := RebuildMemoPayload(memo); err != nil {
				slog.Error("failed to rebuild memo payload", "err", err, "memoID", memo.ID)
				failedCount++
				continue
			}
			if err := r.Store.UpdateMemo(ctx, &store.UpdateMemo{
				ID:      memo.ID,
				Payload: memo.Payload,
			}); err != nil {
				slog.Error("failed to update memo", "err", err, "memoID", memo.ID)
				failedCount++
				continue
			}
			successCount++
		}

		processed += len(memos)

		// Move to next batch
		offset += len(memos)
	}

	if processed > 0 {
		slog.Info("memo payload rebuild finished", "processed", processed, "successCount", successCount, "failedCount", failedCount)
	}
}

func RebuildMemoPayload(memo *store.Memo) error {
	nodes, err := parser.Parse(tokenizer.Tokenize(memo.Content))
	if err != nil {
		return errors.Wrap(err, "failed to parse content")
	}

	if memo.Payload == nil {
		memo.Payload = &storepb.MemoPayload{}
	}
	tags := []string{}
	tagSet := map[string]struct{}{}
	property := &storepb.MemoPayload_Property{}
	emotion := memo.Payload.GetEmotion()
	TraverseASTNodes(nodes, func(node ast.Node) {
		switch n := node.(type) {
		case *ast.Tag:
			tag := n.Content
			if _, exists := tagSet[tag]; !exists {
				tagSet[tag] = struct{}{}
				tags = append(tags, tag)
			}
		case *ast.Link, *ast.AutoLink:
			property.HasLink = true
		case *ast.TaskListItem:
			property.HasTaskList = true
			if !n.Complete {
				property.HasIncompleteTasks = true
			}
		case *ast.Code, *ast.CodeBlock:
			property.HasCode = true
		case *ast.EmbeddedContent:
			// TODO: validate references.
			property.References = append(property.References, n.ResourceName)
		}
	})
	memo.Payload.Tags = tags
	memo.Payload.Property = property
	memo.Payload.Emotion = emotion
	return nil
}

func TraverseASTNodes(nodes []ast.Node, fn func(ast.Node)) {
	for _, node := range nodes {
		fn(node)
		switch n := node.(type) {
		case *ast.Paragraph:
			TraverseASTNodes(n.Children, fn)
		case *ast.Heading:
			TraverseASTNodes(n.Children, fn)
		case *ast.Blockquote:
			TraverseASTNodes(n.Children, fn)
		case *ast.List:
			TraverseASTNodes(n.Children, fn)
		case *ast.OrderedListItem:
			TraverseASTNodes(n.Children, fn)
		case *ast.UnorderedListItem:
			TraverseASTNodes(n.Children, fn)
		case *ast.TaskListItem:
			TraverseASTNodes(n.Children, fn)
		case *ast.Bold:
			TraverseASTNodes(n.Children, fn)
		}
	}
}
