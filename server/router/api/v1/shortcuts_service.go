package v1

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/filter"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) ListShortcuts(ctx context.Context, request *v1pb.ListShortcutsRequest) (*v1pb.ListShortcutsResponse, error) {
	_, userID, err := s.getCurrentUserByParent(ctx, request.Parent)
	if err != nil {
		return nil, err
	}

	userSetting, err := s.Store.GetUserSetting(ctx, &store.FindUserSetting{
		UserID: &userID,
		Key:    storepb.UserSettingKey_SHORTCUTS,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user setting", err)
	}
	if userSetting == nil {
		return &v1pb.ListShortcutsResponse{
			Shortcuts: []*v1pb.Shortcut{},
		}, nil
	}

	shortcutsUserSetting := userSetting.GetShortcuts()
	shortcuts := []*v1pb.Shortcut{}
	for _, shortcut := range shortcutsUserSetting.GetShortcuts() {
		shortcuts = append(shortcuts, &v1pb.Shortcut{
			Id:     shortcut.GetId(),
			Title:  shortcut.GetTitle(),
			Filter: shortcut.GetFilter(),
		})
	}

	return &v1pb.ListShortcutsResponse{
		Shortcuts: shortcuts,
	}, nil
}

func (s *APIV1Service) CreateShortcut(ctx context.Context, request *v1pb.CreateShortcutRequest) (*v1pb.Shortcut, error) {
	_, userID, err := s.getCurrentUserByParent(ctx, request.Parent)
	if err != nil {
		return nil, err
	}
	if request.Shortcut == nil {
		return nil, invalidArgErrf("shortcut is required")
	}

	newShortcut := &storepb.ShortcutsUserSetting_Shortcut{
		Id:     util.GenUUID(),
		Title:  request.Shortcut.GetTitle(),
		Filter: request.Shortcut.GetFilter(),
	}
	if newShortcut.Title == "" {
		return nil, invalidArgErrf("title is required")
	}
	if err := s.validateFilter(ctx, newShortcut.Filter); err != nil {
		return nil, invalidArgErrf("invalid filter: %v", err)
	}
	if request.ValidateOnly {
		return &v1pb.Shortcut{
			Id:     newShortcut.GetId(),
			Title:  newShortcut.GetTitle(),
			Filter: newShortcut.GetFilter(),
		}, nil
	}

	userSetting, err := s.Store.GetUserSetting(ctx, &store.FindUserSetting{
		UserID: &userID,
		Key:    storepb.UserSettingKey_SHORTCUTS,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user setting", err)
	}
	if userSetting == nil {
		userSetting = &storepb.UserSetting{
			UserId: userID,
			Key:    storepb.UserSettingKey_SHORTCUTS,
			Value: &storepb.UserSetting_Shortcuts{
				Shortcuts: &storepb.ShortcutsUserSetting{
					Shortcuts: []*storepb.ShortcutsUserSetting_Shortcut{},
				},
			},
		}
	}
	shortcutsUserSetting := userSetting.GetShortcuts()
	shortcuts := shortcutsUserSetting.GetShortcuts()
	shortcuts = append(shortcuts, newShortcut)
	shortcutsUserSetting.Shortcuts = shortcuts

	userSetting.Value = &storepb.UserSetting_Shortcuts{
		Shortcuts: shortcutsUserSetting,
	}

	_, err = s.Store.UpsertUserSetting(ctx, userSetting)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
	}

	return &v1pb.Shortcut{
		Id:     newShortcut.GetId(),
		Title:  newShortcut.GetTitle(),
		Filter: newShortcut.GetFilter(),
	}, nil
}

func (s *APIV1Service) UpdateShortcut(ctx context.Context, request *v1pb.UpdateShortcutRequest) (*v1pb.Shortcut, error) {
	_, userID, err := s.getCurrentUserByParent(ctx, request.Parent)
	if err != nil {
		return nil, err
	}
	if err := requireFieldMaskPaths(request.UpdateMask, "update mask"); err != nil {
		return nil, err
	}
	if request.Shortcut == nil {
		return nil, invalidArgErrf("shortcut is required")
	}

	userSetting, err := s.Store.GetUserSetting(ctx, &store.FindUserSetting{
		UserID: &userID,
		Key:    storepb.UserSettingKey_SHORTCUTS,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user setting", err)
	}
	if userSetting == nil {
		return nil, grpcErr(codes.NotFound, "shortcut not found", nil)
	}

	shortcutsUserSetting := userSetting.GetShortcuts()
	shortcuts := shortcutsUserSetting.GetShortcuts()
	newShortcuts := make([]*storepb.ShortcutsUserSetting_Shortcut, 0, len(shortcuts))
	found := false
	for _, shortcut := range shortcuts {
		if shortcut.GetId() == request.Shortcut.GetId() {
			found = true
			for _, field := range request.UpdateMask.Paths {
				if field == "title" {
					if request.Shortcut.GetTitle() == "" {
						return nil, invalidArgErrf("title is required")
					}
					shortcut.Title = request.Shortcut.GetTitle()
				} else if field == "filter" {
					if err := s.validateFilter(ctx, request.Shortcut.GetFilter()); err != nil {
						return nil, invalidArgErrf("invalid filter: %v", err)
					}
					shortcut.Filter = request.Shortcut.GetFilter()
				} else {
					return nil, invalidArgErrf("invalid update path: %s", field)
				}
			}
		}
		newShortcuts = append(newShortcuts, shortcut)
	}
	if !found {
		return nil, grpcErr(codes.NotFound, "shortcut not found", nil)
	}
	shortcutsUserSetting.Shortcuts = newShortcuts
	userSetting.Value = &storepb.UserSetting_Shortcuts{
		Shortcuts: shortcutsUserSetting,
	}
	_, err = s.Store.UpsertUserSetting(ctx, userSetting)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
	}

	return &v1pb.Shortcut{
		Id:     request.Shortcut.GetId(),
		Title:  request.Shortcut.GetTitle(),
		Filter: request.Shortcut.GetFilter(),
	}, nil
}

func (s *APIV1Service) DeleteShortcut(ctx context.Context, request *v1pb.DeleteShortcutRequest) (*emptypb.Empty, error) {
	_, userID, err := s.getCurrentUserByParent(ctx, request.Parent)
	if err != nil {
		return nil, err
	}

	userSetting, err := s.Store.GetUserSetting(ctx, &store.FindUserSetting{
		UserID: &userID,
		Key:    storepb.UserSettingKey_SHORTCUTS,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user setting", err)
	}
	if userSetting == nil {
		return &emptypb.Empty{}, nil
	}

	shortcutsUserSetting := userSetting.GetShortcuts()
	shortcuts := shortcutsUserSetting.GetShortcuts()
	newShortcuts := make([]*storepb.ShortcutsUserSetting_Shortcut, 0, len(shortcuts))
	for _, shortcut := range shortcuts {
		if shortcut.GetId() != request.Id {
			newShortcuts = append(newShortcuts, shortcut)
		}
	}
	shortcutsUserSetting.Shortcuts = newShortcuts
	userSetting.Value = &storepb.UserSetting_Shortcuts{
		Shortcuts: shortcutsUserSetting,
	}
	_, err = s.Store.UpsertUserSetting(ctx, userSetting)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) validateFilter(_ context.Context, filterStr string) error {
	if filterStr == "" {
		return errors.New("filter cannot be empty")
	}
	// Validate the filter.
	parsedExpr, err := filter.Parse(filterStr, filter.MemoFilterCELAttributes...)
	if err != nil {
		return fmt.Errorf("failed to parse filter: %w", err)
	}
	convertCtx := filter.NewConvertContext()
	err = s.Store.GetDriver().ConvertExprToSQL(convertCtx, parsedExpr.GetExpr())
	if err != nil {
		return fmt.Errorf("failed to convert filter to SQL: %w", err)
	}
	return nil
}
