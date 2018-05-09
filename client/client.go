package main

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/bclermont/grpctest/common"
	"github.com/bclermont/grpctest/proto"
)

func clientCommand() *cobra.Command {
	var (
		authContext func(context.Context) context.Context
		client      grpctest.GrpcTestClient
		log         *zap.Logger
		interval    time.Duration
	)
	return &cobra.Command{
		Use:   "client",
		Short: "Run client stream",
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			authContext, client, interval, log, err = preUp()
			return
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return ClientStreamTest(authContext, client, interval, log)
		},
	}
}

// ClientStreamTest connect to a server and periodically send request 10 times and close stream
func ClientStreamTest(authContext func(context.Context) context.Context, client grpctest.GrpcTestClient, interval time.Duration, log *zap.Logger) error {
	const maxSend = 10
	var (
		ctx      context.Context
		cancelFn context.CancelFunc
		sent     int
	)
	log = log.With(zap.Int("max", maxSend))

	for {
		log.Debug("Connect to gRPC server")
		ctx, cancelFn = context.WithCancel(context.Background())
		stream, err := client.ClientStream(authContext(ctx))
		if err != nil {
			log.Error("Can't open stream, try again", zap.Error(err))
			time.Sleep(common.ReconnectInterval)
			continue
		}
		log.Debug("Connected")

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
				sent++
				sLog := log.With(zap.Int("sent", sent))
				if sent <= maxSend {
					sLog.Debug("Sent interval request", req.ZapFields()...)
					continue
				}

				sLog.Debug("Maximum sent, close stream")
				resp, err := stream.CloseAndRecv()
				if err != nil && err != io.EOF {
					return err
				}
				sLog.Debug("Got response", resp.ZapFields()...)
				return nil
			case <-ctx.Done():
				log.Info("Context done, stop receive from stream", zap.Error(ctx.Err()))
				break selectLoop
			}
		}

		log.Debug("Disconnected from server, reconnect")
		time.Sleep(common.ReconnectInterval)
	}
}
