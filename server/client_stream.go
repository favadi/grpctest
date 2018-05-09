package main

import (
	"io"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpctest "github.com/bclermont/grpctest/proto"
	"github.com/prometheus/common/log"
)

func (s *server) ClientStream(stream grpctest.GrpcTest_ClientStreamServer) error {
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
				log.Error("Error receive stream", zap.Error(err))
				recvErrorChan <- err
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Context done, leaving", zap.Error(ctx.Err()))
			return ctx.Err()
		case recvErr, isOpen := <-recvErrorChan:
			if !isOpen {
				s.log.Debug("Error channel closed")
				continue
			}
			s.log.Debug("Couldn't receive message, stop", zap.Error(recvErr))
			return recvErr
		case req, isOpen := <-reqChan:
			if !isOpen {
				s.log.Debug("Channel closed, leaving")
				return nil
			}
			// process request
			s.log.Debug("Request received", req.ZapFields()...)
		}
	}
}
