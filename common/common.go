package common

import (
	"flag"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	ReconnectInterval time.Duration = time.Second //time.Millisecond * 500
	IdlePing          time.Duration = time.Second * 10
	IdlePingTimeout   time.Duration = time.Second * 5
)

func Init() (int, string, time.Duration, *zap.Logger) {
	port := flag.Int("port", 8841, "Port")
	apiKey := flag.String("key", "", "API Key")
	sendInterval := flag.Duration("interval", time.Second*15, "Interval between sending messages")
	flag.Parse()

	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	if len(*apiKey) == 0 {
		log.Fatal("Missing '-key'")
	}
	if *port == 0 {
		log.Fatal("Missing '-port'")
	}
	return *port, *apiKey, *sendInterval, log
}

func GrpcErrorFields(err error) []zapcore.Field {
	code := grpc.Code(err)
	return []zapcore.Field{
		GrpcCodeField(code),
		zap.Error(err),
	}
}

func GrpcCodeField(code codes.Code) zapcore.Field {
	return zap.String("code", code.String())
}
