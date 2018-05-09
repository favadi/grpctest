package common

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	ReconnectInterval time.Duration = time.Second //time.Millisecond * 500
	IdlePing          time.Duration = time.Second * 10
	IdlePingTimeout   time.Duration = time.Second * 5

	keyPort          = "port"
	keyKey           = "key"
	keyInterval      = "interval"
	errMissingFormat = "Missing %q"
)

func init() {
	viper.SetDefault(keyInterval, time.Second*15)
	viper.SetDefault(keyPort, 8841)
}

func Init() (port int, apiKey string, interval time.Duration, log *zap.Logger) {
	viper.AutomaticEnv()
	port = viper.GetInt(keyPort)
	apiKey = viper.GetString(keyKey)
	interval = viper.GetDuration(keyInterval)

	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	if len(apiKey) == 0 {
		log.Fatal(fmt.Sprintf(errMissingFormat, keyKey))
	}
	if port == 0 {
		log.Fatal(fmt.Sprintf(errMissingFormat, keyPort))
	}
	return
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
