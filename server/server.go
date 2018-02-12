package main

import (
	"crypto/rand"
	"io"
	"log"
	"time"

	"github.com/oklog/ulid"
	"go.uber.org/zap"

	"github.com/bclermont/grpctest/common"
	grpctest "github.com/bclermont/grpctest/proto"
)

type server struct {
	log      *zap.Logger
	interval time.Duration
}

func (s *server) BiDirectionalStream(stream grpctest.GrpcTest_BiDirectionalStreamServer) error {
	var (
		ctx     = stream.Context()
		reqChan = make(chan *grpctest.Request, 1)
	)

	go func() {
		for {
			s.log.Debug("Wait request on stream")
			req, err := stream.Recv()
			switch err {
			case nil:
				reqChan <- req
			case io.EOF:
				s.log.Info("EOF received on stream, stop")
				close(reqChan)
				return
			default:
				select {
				case <-ctx.Done():
					s.log.Info("Context done, stop receive stream")
					return
				default:
					s.log.Error("Error receive stream", zap.Error(err))
					time.Sleep(common.ReconnectInterval)
				}
			}
		}
	}()

	ticker := time.NewTicker(s.interval)

	for {
		select {
		case t := <-ticker.C:
			id, err := ulid.New(ulid.Timestamp(t), rand.Reader)
			if err != nil {
				log.Fatal("Can't generate ULID", zap.Error(err))
			}
			resp := &grpctest.Response{
				Value: id.String(),
			}
			if err := stream.Send(resp); err != nil {
				s.log.Error("Can't send interval response", zap.Error(err))
			} else {
				s.log.Debug("Sent interval response", resp.ZapFields()...)
			}
		case <-ctx.Done():
			s.log.Info("Context done, leaving")
			ticker.Stop()
			close(reqChan)
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
