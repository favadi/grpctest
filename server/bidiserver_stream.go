package main

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid"
	"github.com/prometheus/common/log"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpctest "github.com/bclermont/grpctest/proto"
)

// BiDirectionalStream send response at some interval and log received request
func (s *server) BiDirectionalStream(stream grpctest.GrpcTest_BiDirectionalStreamServer) error {
	reqChan := make(chan *grpctest.Request, 1)
	recvErrorChan := make(chan error, 1)
	ctx := stream.Context()

	go func() {
		defer close(reqChan)
		defer close(recvErrorChan)
		for {
			s.log.Debug("Wait request on stream")
			req, err := stream.Recv()
			switch err {
			case nil:
				reqChan <- req
			case io.EOF:
				log.Info("EOF received on stream, stop")
				return
			default:
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Canceled {
					log.Debug("Context cancelled, stop receive stream")
					return
				}
				recvErrorChan <- err
				return
			}
		}
	}()

	ticker := time.NewTicker(s.interval)

	for {
		select {
		case t := <-ticker.C:
			// send some dummy response
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
		case recvErr, isOpen := <-recvErrorChan:
			if !isOpen {
				s.log.Debug("Error channel closed")
				continue
			}
			s.log.Debug("Couldn't receive message, stop", zap.Error(recvErr))
			return recvErr
		case <-ctx.Done():
			s.log.Info("Context done, leaving", zap.Error(ctx.Err()))
			ticker.Stop()
			return ctx.Err()
		case req, isOpen := <-reqChan:
			if !isOpen {
				s.log.Debug("Channel closed, leaving")
				ticker.Stop()
				return nil
			}
			// process request
			s.log.Debug("Request received", req.ZapFields()...)
		}
	}
}
