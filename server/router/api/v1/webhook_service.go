package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) CreateWebhook(ctx context.Context, request *v1pb.CreateWebhookRequest) (*v1pb.Webhook, error) {
	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if currentUser == nil {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}
	name := strings.TrimSpace(request.Name)
	url := strings.TrimSpace(request.Url)
	if name == "" {
		return nil, invalidArgErrf("name is required")
	}
	if url == "" {
		return nil, invalidArgErrf("url is required")
	}

	webhook, err := s.Store.CreateWebhook(ctx, &store.Webhook{
		CreatorID: currentUser.ID,
		Name:      name,
		URL:       url,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create webhook", err)
	}
	return convertWebhookFromStore(webhook), nil
}

func (s *APIV1Service) ListWebhooks(ctx context.Context, request *v1pb.ListWebhooksRequest) (*v1pb.ListWebhooksResponse, error) {
	creatorID, err := ExtractUserIDFromName(request.Creator)
	if err != nil {
		return nil, invalidArgErrf("invalid creator name: %v", err)
	}

	webhooks, err := s.Store.ListWebhooks(ctx, &store.FindWebhook{
		CreatorID: &creatorID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list webhooks", err)
	}

	response := &v1pb.ListWebhooksResponse{
		Webhooks: []*v1pb.Webhook{},
	}
	for _, webhook := range webhooks {
		response.Webhooks = append(response.Webhooks, convertWebhookFromStore(webhook))
	}
	return response, nil
}

func (s *APIV1Service) GetWebhook(ctx context.Context, request *v1pb.GetWebhookRequest) (*v1pb.Webhook, error) {
	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if currentUser == nil {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	webhook, err := s.Store.GetWebhook(ctx, &store.FindWebhook{
		ID:        &request.Id,
		CreatorID: &currentUser.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get webhook", err)
	}
	if webhook == nil {
		return nil, grpcErr(codes.NotFound, "webhook not found", nil)
	}
	return convertWebhookFromStore(webhook), nil
}

func (s *APIV1Service) UpdateWebhook(ctx context.Context, request *v1pb.UpdateWebhookRequest) (*v1pb.Webhook, error) {
	if request.UpdateMask == nil || len(request.UpdateMask.Paths) == 0 {
		return nil, invalidArgErrf("update_mask is required")
	}
	if request.Webhook == nil {
		return nil, invalidArgErrf("webhook is required")
	}

	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if currentUser == nil {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	webhook, err := s.Store.GetWebhook(ctx, &store.FindWebhook{
		ID:        &request.Webhook.Id,
		CreatorID: &currentUser.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get webhook", err)
	}
	if webhook == nil {
		return nil, grpcErr(codes.NotFound, "webhook not found", nil)
	}

	update := &store.UpdateWebhook{
		ID: request.Webhook.Id,
	}
	for _, field := range request.UpdateMask.Paths {
		switch field {
		case "name":
			name := strings.TrimSpace(request.Webhook.Name)
			if name == "" {
				return nil, invalidArgErrf("name is required")
			}
			update.Name = &name
		case "url":
			url := strings.TrimSpace(request.Webhook.Url)
			if url == "" {
				return nil, invalidArgErrf("url is required")
			}
			update.URL = &url
		default:
			return nil, invalidArgErrf("invalid update path: %s", field)
		}
	}

	updatedWebhook, err := s.Store.UpdateWebhook(ctx, update)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to update webhook", err)
	}
	return convertWebhookFromStore(updatedWebhook), nil
}

func (s *APIV1Service) DeleteWebhook(ctx context.Context, request *v1pb.DeleteWebhookRequest) (*emptypb.Empty, error) {
	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if currentUser == nil {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	webhook, err := s.Store.GetWebhook(ctx, &store.FindWebhook{
		ID: &request.Id,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get webhook", err)
	}
	if webhook == nil {
		return nil, grpcErr(codes.NotFound, "webhook not found", nil)
	}
	if webhook.CreatorID != currentUser.ID {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	err = s.Store.DeleteWebhook(ctx, &store.DeleteWebhook{
		ID: request.Id,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to delete webhook", err)
	}
	return &emptypb.Empty{}, nil
}

func convertWebhookFromStore(webhook *store.Webhook) *v1pb.Webhook {
	return &v1pb.Webhook{
		Id:         webhook.ID,
		CreateTime: timestamppb.New(time.Unix(webhook.CreatedTs, 0)),
		UpdateTime: timestamppb.New(time.Unix(webhook.UpdatedTs, 0)),
		Creator:    fmt.Sprintf("%s%d", UserNamePrefix, webhook.CreatorID),
		Name:       webhook.Name,
		Url:        webhook.URL,
	}
}
