package main

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/bclermont/grpctest/common"
	"github.com/bclermont/grpctest/proto"
)

func serverCommand() *cobra.Command {
	var (
		authContext func(context.Context) context.Context
		client      grpctest.GrpcTestClient
		log         *zap.Logger
		interval    time.Duration
	)
	return &cobra.Command{
		Use:   "server",
		Short: "Run server client",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			authContext, client, interval, log, err = preUp()
			return
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return ServerClientTest(authContext, client, interval, log)
		},
	}
}

// ServerClientTest connect to a server and log response when it receive one. stop when it got 10 response
func ServerClientTest(authContext func(context.Context) context.Context, client grpctest.GrpcTestClient, interval time.Duration, log *zap.Logger) error {
	const maxReceived = 10
	var (
		ctx      context.Context
		cancelFn context.CancelFunc
		respChan = make(chan *grpctest.Response, 1)
		received int
	)
	log = log.With(zap.Int("max", maxReceived))

	for {
		log.Debug("Connect to gRPC server")
		ctx, cancelFn = context.WithCancel(context.Background())

		id, err := ulid.New(ulid.Now(), rand.Reader)
		if err != nil {
			return err
		}
		stream, err := client.ServerStream(authContext(ctx), &grpctest.Request{Value: id.String()})
		if err != nil {
			log.Error("Can't open stream, try again", zap.Error(err))
			time.Sleep(common.ReconnectInterval)
			continue
		}
		log.Debug("Connected")

		go func() {
			defer cancelFn()
			for {
				log.Debug("Wait response on stream")
				resp, err := stream.Recv()
				switch err {
				case nil:
					respChan <- resp
				case io.EOF:
					log.Debug("Stream closed, reconnect")
					return
				default:
					st, ok := status.FromError(err)
					if ok && st.Code() == codes.Canceled {
						log.Debug("Context cancelled, stop receive stream")
						return
					}
					log.Error("Error receive stream", zap.Error(err))
					return
				}
			}
		}()

	selectLoop:
		for {
			select {
			case <-ctx.Done():
				log.Info("Context done, stop receive from stream", zap.Error(ctx.Err()))
				break selectLoop
			case resp := <-respChan:
				// process response
				received++
				sLog := log.With(zap.Int("received", received))
				sLog.Debug("Received response", resp.ZapFields()...)
				if received >= maxReceived {
					return stream.CloseSend()
				}
			}
		}

		log.Debug("Disconnected from server, reconnect")
		time.Sleep(common.ReconnectInterval)
	}
}
