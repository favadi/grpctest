package main

import (
	"fmt"
	"net"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"

	"github.com/bclermont/grpctest/common"
	"github.com/bclermont/grpctest/proto"
)

func main() {
	port, apiKey, interval, log := common.Init()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal("Can't bind port", zap.Error(err), zap.Int("port", port))
	}

	myAuthFunction := func(ctx context.Context) (context.Context, error) {
		val, err := grpc_auth.AuthFromMD(ctx, grpctest.Scheme)
		if err != nil {
			return ctx, err
		}
		if val != apiKey {
			return ctx, grpc.Errorf(codes.Unauthenticated, "Invalid API key")
		}
		return ctx, nil
	}

	grpc_zap.ReplaceGrpcLogger(log)
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_zap.StreamServerInterceptor(log),
				grpc_auth.StreamServerInterceptor(myAuthFunction),
				grpc_recovery.StreamServerInterceptor(),
			),
		),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				grpc_zap.UnaryServerInterceptor(log),
				grpc_auth.UnaryServerInterceptor(myAuthFunction),
				grpc_recovery.UnaryServerInterceptor(),
			),
		),
		grpc.KeepaliveParams(
			keepalive.ServerParameters{
				Time:    common.IdlePing,
				Timeout: common.IdlePingTimeout,
			},
		),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             common.IdlePing - time.Second,
				PermitWithoutStream: true,
			},
		),
	)

	grpctest.RegisterGrpcTestServer(grpcServer, &server{
		log:      log,
		interval: interval,
	})
	log.Info("Listen gRPC Server", zap.String("address", lis.Addr().String()))
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatal("Can't grpc serve", zap.Error(err))
	}
}
