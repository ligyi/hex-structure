# hex-structure 

This repo holds a rough around the edges structure of an (opinionated) hexagonal architectured Golang service.

## Layout

| Layer | Package |
| --- | --- |
| Composition roots | `cmd/api` (HTTP **and** gRPC), `cmd/web` (HTTP only) |
| Primary / driving adapters | `internal/adapters/primary/web` (Fiber/REST), `internal/adapters/primary/grpc` (gRPC) |
| API contract | `proto/user/v1/user.proto` → generated into `gen/user/v1` |
| Inbound port | `internal/core/services/users` (`users.API`) |
| Core | `internal/core/domain/user`, `internal/core/services/users` |
| Outbound port | `internal/ports` (`ports.UserRepo`) |
| Secondary / driven adapter | `internal/adapters/secondary/postgres` |

Both primary adapters are handed the *same* `*users.Service`, so the business logic is written once and
stays transport agnostic — the refactor from REST to gRPC never touched `internal/core`.

## Running

```sh
make run     # go run ./cmd/api — REST on :8000 and gRPC on :50051
make build   # go build ./...
make test    # go test ./...
```

REST:

```sh
curl -X POST localhost:8000/users -H 'Content-Type: application/json' -d '{"username":"alice"}'
# {"userId":"..."}
```

gRPC (service `user.v1.UserService`, method `CreateAccount`). The server speaks plaintext h2c, so client
tools need `-plaintext` — without it you get `tls: first record does not look like a TLS handshake`:

```sh
grpcurl -plaintext -d '{"username":"alice"}' localhost:50051 user.v1.UserService/CreateAccount
grpcui -plaintext localhost:50051        # browser UI
```

`cmd/api` enables gRPC server reflection (`WithReflection()` → `reflection.Register`), so grpcui and
grpcurl discover the API on their own. Drop that option (or gate it behind config) if the API should not
be introspectable, in which case those tools need `-proto proto/user/v1/user.proto`.

Domain validation failures come back as `codes.InvalidArgument`; anything unexpected becomes
`codes.Internal`.

## Regenerating the gRPC code

```sh
brew install protobuf   # protoc, or the equivalent for your OS
make tools              # protoc-gen-go + protoc-gen-go-grpc into $(go env GOPATH)/bin
make proto              # proto/user/v1/user.proto -> gen/user/v1/*.pb.go
```

