package v1

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/usememos/memos/internal/profile"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

type APIV1Service struct {
	grpc_health_v1.UnimplementedHealthServer

	v1pb.UnimplementedWorkspaceServiceServer
	v1pb.UnimplementedWorkspaceSettingServiceServer
	v1pb.UnimplementedAuthServiceServer
	v1pb.UnimplementedUserServiceServer
	v1pb.UnimplementedMemoServiceServer
	v1pb.UnimplementedResourceServiceServer
	v1pb.UnimplementedShortcutServiceServer
	v1pb.UnimplementedInboxServiceServer
	v1pb.UnimplementedActivityServiceServer
	v1pb.UnimplementedWebhookServiceServer
	v1pb.UnimplementedMarkdownServiceServer
	v1pb.UnimplementedIdentityProviderServiceServer

	Secret  string
	Profile *profile.Profile
	Store   *store.Store
	// clashSubscriptionStore persists clash verge subscription import rules.
	clashSubscriptionStore *clashVergeSubscriptionStore
	// runtimeConfigStore persists runtime related configurations such as memory limit.
	runtimeConfigStore *runtimeConfigStore

	grpcServer *grpc.Server
}

func NewAPIV1Service(secret string, profile *profile.Profile, store *store.Store, grpcServer *grpc.Server) *APIV1Service {
	grpc.EnableTracing = true
	apiv1Service := &APIV1Service{
		Secret:                 secret,
		Profile:                profile,
		Store:                  store,
		clashSubscriptionStore: newClashVergeSubscriptionStore(filepath.Join(profile.Data, "clash_verge_subscriptions.json")),
		runtimeConfigStore:     newRuntimeConfigStore(filepath.Join(profile.Data, "runtime_config.json")),
		grpcServer:             grpcServer,
	}
	if cfg, err := apiv1Service.runtimeConfigStore.Get(); err == nil {
		apiv1Service.applyMemoryLimit(cfg.MemoryLimitBytes)
	}
	grpc_health_v1.RegisterHealthServer(grpcServer, apiv1Service)
	v1pb.RegisterWorkspaceServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterWorkspaceSettingServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterAuthServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterUserServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterMemoServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterResourceServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterShortcutServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterInboxServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterActivityServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterWebhookServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterMarkdownServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterIdentityProviderServiceServer(grpcServer, apiv1Service)
	reflection.Register(grpcServer)
	return apiv1Service
}

// RegisterGateway registers the gRPC-Gateway with the given Echo instance.
// It returns the underlying runtime.ServeMux so callers can reuse it for special
// routes such as /file.
func (s *APIV1Service) RegisterGateway(ctx context.Context, echoServer *echo.Echo, dialOptions ...grpc.DialOption) (*runtime.ServeMux, error) {
	var target string
	if len(s.Profile.UNIXSock) == 0 {
		target = fmt.Sprintf("%s:%d", s.Profile.Addr, s.Profile.Port)
	} else {
		target = fmt.Sprintf("unix:%s", s.Profile.UNIXSock)
	}
	if len(dialOptions) == 0 {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
		// Use passthrough to avoid resolver errors with bufconn.
		target = "passthrough:///bufnet"
	}
	dialOptions = append(dialOptions, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)))
	conn, err := grpc.NewClient(target, dialOptions...)
	if err != nil {
		return nil, err
	}

	gwMux := runtime.NewServeMux()
	if err := v1pb.RegisterWorkspaceServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterWorkspaceSettingServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterAuthServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterUserServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterMemoServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterResourceServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterShortcutServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterInboxServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterActivityServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterWebhookServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterMarkdownServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	if err := v1pb.RegisterIdentityProviderServiceHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}
	gwGroup := echoServer.Group("")
	gwGroup.Use(middleware.CORS())
	handler := echo.WrapHandler(gwMux)

	gwGroup.GET("/api/v1/clash-verge/subscription", s.handleFilteredClashVergeSubscription)
	gwGroup.GET("/api/v1/clash-verge/subscriptions", s.listClashVergeSubscriptions)
	gwGroup.POST("/api/v1/clash-verge/subscriptions", s.createClashVergeSubscription)
	gwGroup.PATCH("/api/v1/clash-verge/subscriptions/:id", s.updateClashVergeSubscription)
	gwGroup.DELETE("/api/v1/clash-verge/subscriptions/:id", s.deleteClashVergeSubscription)
	gwGroup.GET("/api/v1/clash-verge/subscriptions/:id/config", s.getClashVergeSubscriptionConfig)
	gwGroup.GET("/api/v1/emotions/report", s.getEmotionReport)
	gwGroup.GET("/api/v1/runtime/memory-limit", s.getRuntimeMemoryLimit)
	gwGroup.PUT("/api/v1/runtime/memory-limit", s.upsertRuntimeMemoryLimit)
	gwGroup.Any("/api/v1/*", handler)
	gwGroup.GET("/file/resources/:uid/:filename", s.handleResourceFile)
	gwGroup.Any("/file/*", handler)

	// GRPC web proxy.
	options := []grpcweb.Option{
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(func(_ string) bool {
			return true
		}),
	}
	wrappedGrpc := grpcweb.WrapServer(s.grpcServer, options...)
	echoServer.Any("/memos.api.v1.*", echo.WrapHandler(wrappedGrpc))

	return gwMux, nil
}
