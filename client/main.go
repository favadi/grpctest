package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/bclermont/grpctest/common"
	"github.com/bclermont/grpctest/proto"
)

const keyServer = "server"

func init() {
	viper.SetDefault(keyServer, "localhost")
}

func preUp() (fn func(context.Context) context.Context, client grpctest.GrpcTestClient, interval time.Duration, log *zap.Logger, err error) {
	var (
		port   int
		apiKey string
		server = viper.GetString(keyServer)
	)
	port, apiKey, interval, log = common.Init()
	if len(server) == 0 {
		err = errors.Errorf("Missing %q", keyServer)
		return
	}

	grpc_zap.ReplaceGrpcLogger(log)

	retryOption := grpc_retry.WithPerRetryTimeout(time.Minute * 5)
	retryUnary := grpc_retry.UnaryClientInterceptor(retryOption)
	retryStream := grpc_retry.StreamClientInterceptor(retryOption)

	clientConn, err := grpc.Dial(
		net.JoinHostPort(server, strconv.Itoa(port)),
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                common.IdlePing,
			Timeout:             common.IdlePingTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithUnaryInterceptor(retryUnary),
		grpc.WithStreamInterceptor(retryStream),
	)
	if err != nil {
		return
	}

	fn = func(ctx context.Context) context.Context {
		md := metadata.Pairs("authorization", fmt.Sprintf("%s %v", grpctest.Scheme, apiKey))
		return metautils.NiceMD(md).ToOutgoing(ctx)
	}

	client = grpctest.NewGrpcTestClient(clientConn)
	return
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "client",
		Short: "gRPC test client",
	}
	rootCmd.Long = rootCmd.Short
	rootCmd.AddCommand(bidiCommand())
	rootCmd.AddCommand(clientCommand())
	rootCmd.AddCommand(serverCommand())
	rootCmd.AddCommand(unaryCommand())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
