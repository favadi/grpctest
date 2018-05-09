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

func bidiCommand() *cobra.Command {
	var (
		authContext func(context.Context) context.Context
		client      grpctest.GrpcTestClient
		log         *zap.Logger
		interval    time.Duration
	)
	return &cobra.Command{
		Use:   "bidi",
		Short: "Run bidirectional client",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			authContext, client, interval, log, err = preUp()
			return
		},
		Run: func(_ *cobra.Command, _ []string) {
			BidirectionalClientTest(authContext, client, interval, log)
		},
	}
}

// BidirectionalClientTest connect to a server and periodically send request, log response when it receive one. try forever
func BidirectionalClientTest(authContext func(context.Context) context.Context, client grpctest.GrpcTestClient, interval time.Duration, log *zap.Logger) {

	var (
		ctx      context.Context
		cancelFn context.CancelFunc
		respChan = make(chan *grpctest.Response, 1)
	)

	for {
		log.Debug("Connect to gRPC server")
		ctx, cancelFn = context.WithCancel(context.Background())
		stream, err := client.BiDirectionalStream(authContext(ctx))
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

		ticker := time.NewTicker(interval)

	selectLoop:
		for {
			select {
			case t := <-ticker.C:
				// send some dummy request
				id, err := ulid.New(ulid.Timestamp(t), rand.Reader)
				if err != nil {
					log.Error("Can't generate ULID", zap.Error(err))
					cancelFn()
					break selectLoop
				}
				req := &grpctest.Request{
					Value: id.String(),
				}
				if err := stream.Send(req); err != nil {
					log.Error("Can't send interval request", zap.Error(err))
					cancelFn()
					break selectLoop
				}
				log.Debug("Sent interval request", req.ZapFields()...)
			case <-ctx.Done():
				log.Info("Context done, stop receive from stream", zap.Error(ctx.Err()))
				break selectLoop
			case resp := <-respChan:
				// process response
				log.Debug("Received response", resp.ZapFields()...)
			}
		}

		log.Debug("Disconnected from server, reconnect")
		time.Sleep(common.ReconnectInterval)
	}
}
