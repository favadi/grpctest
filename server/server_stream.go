package main

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid"
	"go.uber.org/zap"

	grpctest "github.com/bclermont/grpctest/proto"
)

// ServerStream send response at some interval, until client close stream
func (s *server) ServerStream(req *grpctest.Request, stream grpctest.GrpcTest_ServerStreamServer) error {
	// process request
	s.log.Debug("Request received", req.ZapFields()...)

	ticker := time.NewTicker(s.interval)
	ctx := stream.Context()

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
		case <-ctx.Done():
			s.log.Info("Context done, leaving", zap.Error(ctx.Err()))
			ticker.Stop()
			return ctx.Err()
		}
	}
}
