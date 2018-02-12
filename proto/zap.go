package grpctest

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (r *Request) ZapFields() []zapcore.Field {
	return []zapcore.Field{
		zap.String("value", r.Value),
	}
}

func (r *Response) ZapFields() []zapcore.Field {
	return []zapcore.Field{
		zap.String("value", r.Value),
	}
}
