package main

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	grpctest "github.com/bclermont/grpctest/proto"
)

type server struct {
	log      *zap.Logger
	interval time.Duration
}

func (s *server) BiDirectionalStream(stream grpctest.GrpcTest_BiDirectionalStreamServer) error {
	reqChan := make(chan *grpctest.Request, 1)
	ctx := stream.Context()

	go func() {
		defer close(reqChan)
		for {
			s.log.Debug("Wait request on stream")
			req, err := stream.Recv()
			switch err {
			case nil:
				reqChan <- req
			case io.EOF:
				s.log.Info("EOF received on stream, stop")
				return
			default:
				if grpc.Code(err) == codes.Canceled {
					s.log.Debug("Context cancelled, stop receive stream")
					return
				}
				s.log.Error("Error receive stream", zap.Error(err))
				return
			}
		}
	}()

	ticker := time.NewTicker(s.interval)

	for {
		select {
		case t := <-ticker.C:
			id, err := ulid.New(ulid.Timestamp(t), rand.Reader)
			if err != nil {
				return err
			}
			resp := &grpctest.Response{
				Value: id.String(),
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			s.log.Debug("Sent interval response", resp.ZapFields()...)
		case <-ctx.Done():
			s.log.Info("Context done, leaving")
			ticker.Stop()
			return nil
		case req, isOpen := <-reqChan:
			if !isOpen {
				s.log.Debug("Channel closed, leaving")
				ticker.Stop()
				return nil
			}
			s.log.Debug("Request received", req.ZapFields()...)
		}
	}
}
