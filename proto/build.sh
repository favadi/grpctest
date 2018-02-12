#!/bin/sh

protoc --go_out=plugins=grpc:$HOME/go/src grpctest.proto
