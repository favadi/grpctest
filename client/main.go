package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/oklog/ulid"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/bclermont/grpctest/common"
	"github.com/bclermont/grpctest/proto"
)

func main() {
	server := flag.String("server", "localhost", "Hostname or IP of server")
	port, apiKey, interval, log := common.Init()
	if len(*server) == 0 {
		log.Fatal("missing '-server'")
	}

	grpc_zap.ReplaceGrpcLogger(log)

	retryOption := grpc_retry.WithPerRetryTimeout(time.Minute * 5)
	retryUnary := grpc_retry.UnaryClientInterceptor(retryOption)
	retryStream := grpc_retry.StreamClientInterceptor(retryOption)

	clientConn, err := grpc.Dial(
		net.JoinHostPort(*server, strconv.Itoa(port)),
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
		log.Fatal("Can't gRPC dial", zap.Error(err))
	}

	contextWithToken := func(ctx context.Context) context.Context {
		md := metadata.Pairs("authorization", fmt.Sprintf("%s %v", grpctest.Scheme, apiKey))
		return metautils.NiceMD(md).ToOutgoing(ctx)
	}

	var (
		ctx      context.Context
		cancel   context.CancelFunc
		client   = grpctest.NewGrpcTestClient(clientConn)
		respChan = make(chan *grpctest.Response, 1)
	)

	for {
		log.Debug("Connect to gRPC server")
		ctx, cancel = context.WithCancel(context.Background())
		stream, err := client.BiDirectionalStream(contextWithToken(ctx))
		if err != nil {
			log.Error("Can't open stream, try again", zap.Error(err))
			time.Sleep(common.ReconnectInterval)
			continue
		}
		log.Debug("Connected")

		go func() {
			defer cancel()
			for {
				log.Debug("Wait response on stream")
				resp, err := stream.Recv()
				switch err {
				case nil:
					respChan <- resp
					continue
				case io.EOF:
					log.Debug("Stream closed, reconnect")
					return
				default:
					switch code := grpc.Code(err); code {
					case codes.Unauthenticated:
						log.Error("Invalid API key")
					case codes.Canceled:
						log.Debug("Cancel, stop reading")
					default:
						if closeErr := stream.CloseSend(); err != nil {
							log.With(zap.NamedError("close_err", closeErr)).Error("Error receive stream message, closed stream", common.GrpcErrorFields(err)...)
						} else {
							log.Error("Error receive stream message, closed stream", common.GrpcErrorFields(err)...)
						}
					}
					return
				}
			}
		}()

		ticker := time.NewTicker(interval)

	selectLoop:
		for {
			select {
			case <-ctx.Done():
				log.Info("Context done, stop receive from stream")
				break selectLoop
			case t := <-ticker.C:
				id, err := ulid.New(ulid.Timestamp(t), rand.Reader)
				if err != nil {
					log.Error("Can't generate ULID", zap.Error(err))
					cancel()
					break selectLoop
				}
				req := &grpctest.Request{
					Value: id.String(),
				}
				if err := stream.Send(req); err != nil {
					log.Error("Can't send interval request", zap.Error(err))
					cancel()
					break selectLoop
				} else {
					log.Debug("Sent interval request", req.ZapFields()...)
				}
			case resp := <-respChan:
				log.Debug("Received response", resp.ZapFields()...)
			}
		}

		time.Sleep(common.ReconnectInterval)
	}
}
