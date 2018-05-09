package main

import (
	"time"

	"go.uber.org/zap"
)

type server struct {
	log      *zap.Logger
	interval time.Duration
}
