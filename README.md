# gRPC Stream tests

```
export KEY=xxx
```

# Server

```
go run github.com/bclermont/grpctest/server
```

# Client

```
go build -o client github.com/bclermont/grpctest/client
```

## Bidirectional

```
./client bidi
```

## Server side stream

```
./client server
```

## Client side stream

```
./client client
```