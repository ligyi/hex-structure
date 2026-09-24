# hex-structure — Project Flow

Mermaid diagrams generated from the current source tree.

- Module: `github.com/samverrall/hex-structure` (`go 1.25.0` — raised from 1.21.0 by `google.golang.org/grpc` v1.84)
- Stack: Go, `github.com/gofiber/fiber/v2` (HTTP/REST), `google.golang.org/grpc` + `google.golang.org/protobuf`
  (gRPC), `github.com/google/uuid` (domain IDs)
- Pattern: Ports & Adapters (Hexagonal) — `internal/core` depends on nothing outside the stdlib,
  adapters depend on `internal/ports`, and `cmd/api/main.go` is the composition root that wires **two**
  primary adapters (HTTP + gRPC) onto the same `users.Service`.

| Layer | Package | Responsibility |
| --- | --- | --- |
| Composition root | `cmd/api` | Builds the repo + service, runs the HTTP and gRPC adapters side by side |
| Composition root (HTTP only) | `cmd/web` | Builds the repo + service, starts only the Fiber server |
| Primary / driving adapter | `internal/adapters/primary/web` | HTTP transport (Fiber), routes, handlers, views |
| Primary / driving adapter | `internal/adapters/primary/grpc` | gRPC transport, generated service registration, status mapping |
| API contract | `proto/user/v1` → `gen/user/v1` | `.proto` definition and the generated Go stubs |
| Ports | `internal/ports` | Interfaces + sentinel errors owned by the application |
| Core / domain | `internal/core/domain/user` | Entities and value objects (`User`, `Username`) |
| Core / services | `internal/core/services/users` | Use cases (`API`, `Service`, `CreateAccount`) |
| Secondary / driven adapter | `internal/adapters/secondary/postgres` | `ports.UserRepo` implementation (stub) |

## 1. Composition roots — startup wiring (`cmd/api`, `cmd/web`)

```mermaid
flowchart LR
    subgraph ROOT["cmd/web/main.go"]
        A["postgres.NewUserRepo()"] --> B["users.NewService(userRepo)"]
        B --> C["web.NewApp(usersService, web.WithPort(8000))"]
        C --> D["srv.Run()"]
    end

    A -.->|"returns *postgres.UserRepo"| P1["satisfies ports.UserRepo<br/>Add(ctx, user.User) error"]
    B -.->|"returns *users.Service"| P2["satisfies users.API<br/>CreateAccount(ctx, req)"]
    C -.->|"initAppRoutes()"| R["fiber.Group('/users', CreateAccount)"]
    D -->|"a.fiber.Listen(':8000')"| L["HTTP server blocking on port 8000"]

    subgraph OPT["Functional option (options.go)"]
        O["WithPort(port int) AppOption"]
    end
    O -->|"a.port = 8000"| C
```

Notes:
- `NewApp` defaults `port` to `8000` and applies every `AppOption` before `initAppRoutes()`.
- `Run()` calls `fiber.Listen` which blocks until shutdown, so `main` never exits early.
- Any `error` from `NewUserRepo()` or `Run()` is fatal (`log.Fatalf`).

### Both transports in one binary — `cmd/api/main.go`

```mermaid
flowchart LR
    subgraph ROOT["cmd/api/main.go"]
        A["postgres.NewUserRepo()"] --> B["users.NewService(userRepo)"]
        B --> C["web.NewApp(usersService, web.WithPort(8000))"]
        B --> E["grpc.NewApp(usersService, grpc.WithPort(50051))"]
        C --> F["go httpSrv.Run()<br/>Fiber on :8000"]
        E --> G["go grpcSrv.Run()<br/>grpc.Server on :50051"]
        F --> H{"select on ctx.Done() / errCh"}
        G --> H
        H --> I["grpcSrv.Stop() + httpSrv.Shutdown()"]
    end
```

Notes:
- Because `Run()` blocks on both adapters, each is started in its own goroutine and the process waits on
  a buffered `errCh` (capacity 2) plus `signal.NotifyContext` for `SIGINT`/`SIGTERM`.
- The same `*users.Service` pointer is injected into both adapters — no duplicated business logic.
- `web.App.Shutdown()` and `grpc.App.Stop()` drain in-flight requests before the process exits.
- `grpc.NewApp` defaults `port` to `50051`; `WithServerOptions` lets callers add interceptors/credentials
  (the server itself is constructed *after* the options are applied).

## 2. Inbound request flow — `POST /users` (happy path)

```mermaid
flowchart TD
    C["Client / Browser"] -->|"HTTP request, JSON body with a username field"| FIBER["fiber.App<br/>server.go"]

    subgraph PRIMARY["Primary adapter — internal/adapters/primary/web"]
        FIBER --> GRP["routes.go: initAppRoutes()<br/>fiber.Group('/users', handler)"]
        GRP --> H["users/createaccount.go<br/>CreateAccount(usersAPI) fiber.Handler"]
        H --> BP["c.BodyParser(&input)<br/>input.Username string"]
        BP -->|"parse error"| S400["return c.SendStatus(400)"]
    end

    BP -->|"ok"| CALL["usersAPI.CreateAccount(ctx,<br/>users.CreateAccountReq with the username)"]

    subgraph CORE["Core — internal/core/services/users"]
        CALL --> SVC["Service.CreateAccount<br/>createapplication.go"]
        SVC --> UN["user.NewUsername(req.Username)"]
        UN --> TRIM["strings.TrimSpace"]
        TRIM --> CHK{"empty?"}
        CHK -->|"yes"| ERREMPTY["domain.ErrEmptyUsername<br/>wrapped: 'invalid username supplied: %w'"]
        CHK -->|"no"| NEWU["user.New(userName)<br/>uuid.New() for ID"]
    end

    ERREMPTY --> S500A["return c.SendStatus(500)"]

    NEWU --> PORT["port: ports.UserRepo<br/>Add(ctx, u user.User) error"]
    PORT --> REPO["Secondary adapter<br/>postgres.UserRepo.Add"]
    REPO --> DB[("Postgres database<br/>currently a stub — returns nil")]

    REPO -->|"error"| ERRSV["wrapped: 'failed to add a user: %w'"]
    ERRSV --> S500B["return c.SendStatus(500)"]
    REPO -->|"nil"| RESP["CreateAccountResp with<br/>UserID = user.ID.String()"]
    RESP --> JSON["c.JSON response with a userId field"]
    JSON --> C
    S400 --> C
    S500A --> C
    S500B --> C
```

### Registration detail (current behaviour)

`internal/adapters/primary/web/routes.go` registers the handler through `fiber.Group("/users", ...)`
with no method specified, so Fiber mounts it as middleware on the `/users` prefix (`app.Use`).
The handler writes a response and never calls `c.Next()`, so it terminates every request whose path
matches `/users` (any method), instead of a specific `POST /users` route.

## 3. Sequence diagram — `CreateAccount`

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant Fiber as fiber.App<br/>primary web adapter
    participant Handler as web/users.CreateAccount
    participant API as users.API interface
    participant Svc as users.Service
    participant Dom as domain/user
    participant Repo as postgres.UserRepo
    participant DB as Postgres

    Client->>Fiber: POST /users with JSON body containing username
    Fiber->>Handler: invoke matched handler
    Handler->>Handler: c.BodyParser into anonymous input struct
    alt body does not parse
        Handler-->>Client: 400 Bad Request
    else body parses
        Handler->>API: CreateAccount(ctx, CreateAccountReq)
        API->>Svc: Service.CreateAccount
        Svc->>Dom: NewUsername(req.Username)
        alt username empty after TrimSpace
            Dom-->>Svc: ErrEmptyUsername
            Svc-->>Handler: error wrapped with invalid username supplied
            Handler-->>Client: 500 Internal Server Error
        else valid username
            Dom-->>Svc: Username value object
            Svc->>Dom: New(username) builds User with uuid.New()
            Svc->>Repo: Add(ctx, user)
            Repo->>DB: INSERT user - stub, no query yet
            DB-->>Repo: result
            Repo-->>Svc: nil
            Svc-->>Handler: CreateAccountResp with UserID
            Handler-->>Client: 200 OK JSON userId
        end
    end
```

## 4. Dependency graph — who is allowed to import whom

```mermaid
flowchart BT
    subgraph DRIVING["Driving / primary side"]
        MAIN["cmd/api/main.go<br/>composition root (both transports)"]
        WEB["adapters/primary/web"]
        WEBUSERS["adapters/primary/web/users"]
        GRPC["adapters/primary/grpc"]
        GRPCUSERS["adapters/primary/grpc/users"]
    end

    subgraph CONTRACT["Transport contract (generated)"]
        GEN["gen/user/v1<br/>userv1.UserServiceServer / Client"]
    end

    subgraph APP["Application core"]
        PORTS["ports<br/>UserRepo, ErrUserNotFound"]
        SVC["core/services/users<br/>API, Service, CreateAccount"]
        DOM["core/domain/user<br/>User, Username, New, NewUsername"]
        SERR["core/services<br/>ErrBadRequest, ErrInternalFailure"]
    end

    subgraph DRIVEN["Driven / secondary side"]
        PG["adapters/secondary/postgres<br/>UserRepo"]
    end

    WEB --> WEBUSERS
    WEB --> SVC
    WEBUSERS --> SVC
    GRPC --> GRPCUSERS
    GRPC --> SVC
    GRPC --> GEN
    GRPCUSERS --> SVC
    GRPCUSERS --> DOM
    GRPCUSERS --> GEN
    SVC --> PORTS
    SVC --> DOM
    PORTS --> DOM
    PG --> PORTS
    PG --> DOM
    MAIN --> WEB
    MAIN --> GRPC
    MAIN --> SVC
    MAIN --> PG

    classDef stdlib fill:#eef,stroke:#88a
    class SERR stdlib
```

Key rules visible in this graph:
- `internal/core/...` imports **only** the standard library plus `internal/ports` / its own domain —
  no Fiber, no `database/sql`, so business logic stays transport- and storage-agnostic.
- The dependency on persistence is inverted: `core/services` declares `ports.UserRepo`,
  and `adapters/secondary/postgres` implements it; the interface lives with the consumer.
- `internal/ports` owns the error vocabulary (`ErrUserNotFound`) and the domain import direction is
  `adapters -> ports -> domain`.
- `users.API` (in `core/services/users`) is the inbound port consumed by *both* primary adapters: the
  Fiber handler in `adapters/primary/web/users` and the gRPC service in `adapters/primary/grpc/users`.
  Any further driving adapter (CLI, queue consumer, ...) only needs to satisfy that interface.
- `gen/user/v1` (generated protobuf/gRPC stubs) is imported **only** by `adapters/primary/grpc` — the core
  and the HTTP adapter stay free of transport types.

## 5. File map

```mermaid
flowchart LR
    root["hex-structure/"]
    root --> c1["cmd/web/main.go<br/>HTTP-only composition root"]
    root --> c1b["cmd/api/main.go<br/>HTTP + gRPC composition root"]
    root --> c2["go.mod / go.sum<br/>module + deps"]
    root --> c3["Makefile<br/>tools / proto / build / run / test"]
    root --> c4["README.md"]
    root --> c5["proto/user/v1/user.proto<br/>API contract"]
    root --> c6["gen/user/v1/<br/>user.pb.go, user_grpc.pb.go"]

    c1 --> i1["internal/"]

    i1 --> a1["adapters/primary/web/"]
    a1 --> a11["server.go — App, NewApp, Run, Shutdown"]
    a1 --> a12["options.go — AppOption, WithPort"]
    a1 --> a13["routes.go — initAppRoutes"]
    a1 --> a14["users/createaccount.go — handler"]
    a1 --> a15["views/ — layouts, pages, partials<br/>all empty files"]

    i1 --> a1g["adapters/primary/grpc/"]
    a1g --> a1g1["server.go — App, NewApp, Run, Serve, Stop"]
    a1g --> a1g2["options.go — AppOption, WithPort, WithServerOptions"]
    a1g --> a1g3["routes.go — initAppRoutes<br/>RegisterUserServiceServer"]
    a1g --> a1g4["users/createaccount.go — UserService"]
    a1g --> a1g5["users/errors.go — toStatusError"]
    a1g --> a1g6["users/createaccount_test.go — TestCreateAccount"]
    a1g --> a1g7["server_test.go — bufconn end-to-end tests"]

    i1 --> a2["adapters/secondary/postgres/"]
    a2 --> a21["user.go — UserRepo, NewUserRepo, Add"]

    i1 --> a3["ports/"]
    a3 --> a31["user.go — UserRepo interface, ErrUserNotFound"]

    i1 --> a4["core/"]
    a4 --> a41["domain/user/"]
    a41 --> a411["user.go — User, New"]
    a41 --> a412["name.go — Username, NewUsername, ErrEmptyUsername"]
    a41 --> a413["user_test.go, name_test.go<br/>package user_test, empty stubs"]
    a4 --> a42["services/"]
    a42 --> a421["errors.go — ErrBadRequest, ErrInternalFailure"]
    a42 --> a422["users/users.go — API, Service, NewService"]
    a42 --> a423["users/createapplication.go — CreateAccount"]
    a42 --> a424["users/createapplication_test.go<br/>fake in-memory repo + TestCreateAccount"]
```

## 6. Test / verification flow

```mermaid
flowchart TD
    T["go test ./..."] --> T1["core/services/users<br/>createapplication_test.go"]
    T1 --> F["local fake userRepo struct<br/>appends to users slice"]
    F --> TS["NewService(repo) -> CreateAccount('alice')"]
    TS --> C1{"resp.UserID == repo.users[0].ID.String()?"}
    TS --> C2{"repo.users[0].Username == 'alice'?"}
    C1 -->|"no"| FAIL["t.Fatalf"]
    C2 -->|"no"| FAIL
    C1 -->|"yes"| PASS["test passes"]
    C2 -->|"yes"| PASS

    T --> T4["adapters/primary/grpc<br/>server_test.go — bufconn, real grpc.Server"]
    T4 --> F2["first-party fake userRepo<br/>drives the real users.Service"]
    T --> T5["adapters/primary/grpc/users<br/>createaccount_test.go — TestCreateAccount"]
    T5 --> F3["apiStub fake users.API<br/>asserts InvalidArgument / Internal mapping"]

    T --> T2["core/domain/user<br/>name_test.go — TestNewUsername<br/>asserts nothing yet"]
    T2 --> T3["user_test.go — package declaration only"]
```

Commands:

```sh
go build ./...          # compile all packages
go vet ./...            # static checks
go test ./...           # unit + bufconn tests (fake repo, no Postgres needed)
go run ./cmd/api        # Fiber on :8000 AND gRPC on :50051, sharing one users.Service
go run ./cmd/web        # HTTP only, Fiber on :8000
make tools              # install protoc-gen-go / protoc-gen-go-grpc into $(go env GOPATH)/bin
make proto              # regenerate gen/user/v1 from proto/user/v1 (needs protoc on PATH)
```

## 7. Current state vs. ideal flow

| Step | Implemented? | Detail |
| --- | --- | --- |
| HTTP transport | ✅ | Fiber app with functional options, blocking `Listen` |
| Route `POST /users` | ⚠️ | Registered via `fiber.Group("/users", handler)` — matches any method on that prefix, not `POST` only |
| Request binding | ✅ | Anonymous struct parsed with `c.BodyParser`, `400` on failure |
| Domain validation | ✅ | `user.NewUsername` trims + rejects empty (`ErrEmptyUsername`) |
| Domain entity creation | ✅ | `user.New` assigns `uuid.New()` |
| Error mapping | ⚠️ | Every service error becomes `500`; `services.ErrBadRequest` / `ErrInternalFailure` and `ports.ErrUserNotFound` are declared but never used yet |
| Persistence | ⚠️ | `postgres.UserRepo.Add` is a stub returning `nil`; `db *sql.DB` is never opened or used |
| Views | ⚠️ | `views/layouts`, `views/pages`, `views/partials` files exist but are empty and never loaded |
| gRPC transport | ✅ | `grpc.NewServer` with functional options, blocking `Serve`, graceful `GracefulStop` |
| gRPC service registration | ✅ | `initAppRoutes` calls `userv1.RegisterUserServiceServer` with the adapter from `primary/grpc/users` |
| gRPC error mapping | ✅ | Domain validation errors → `codes.InvalidArgument`, everything else → `codes.Internal` |
| Shared business logic | ✅ | Both adapters are handed the same `*users.Service`; `internal/core` is untouched by the transport change |
| Proto codegen | ✅ | `proto/user/v1/user.proto` → `gen/user/v1` via `make proto` (protoc 36.2, protoc-gen-go-grpc v1.6.2) |
| Dual-transport shutdown | ✅ | `cmd/api` drains both servers on `SIGINT`/`SIGTERM` (`Stop()` + `Shutdown()`) |
| Username length rule | ✅ | `minUsernameLen = 5` / `maxUsernameLen = 100` in `name.go`, plus a letters/digits/`_`/`-` charset check (`ErrUsernameCharset`) |
| Tests | ⚠️ | `createapplication_test.go` and the two gRPC adapter tests assert; domain tests are empty stubs |

## 8. gRPC inbound request flow — `user.v1.UserService/CreateAccount`

```mermaid
flowchart TD
    C["gRPC client<br/>generated stub or grpcurl"] -->|"CreateAccountRequest{username}"| SRV["grpc.Server<br/>adapters/primary/grpc/server.go"]

    subgraph PRIMARY["Primary adapter — internal/adapters/primary/grpc"]
        SRV --> RT["routes.go: initAppRoutes()<br/>userv1.RegisterUserServiceServer"]
        RT --> H["users/createaccount.go<br/>UserService.CreateAccount"]
        H --> CALL["userAPI.CreateAccount(ctx,<br/>users.CreateAccountReq)"]
    end

    CALL --> SVC["core/services/users<br/>the same Service the REST path uses"]
    SVC -->|"error wrapped with %w"| MAP["users/errors.go: toStatusError"]
    SVC -->|"nil"| RESP["CreateAccountResponse{user_id}"]

    MAP --> IA["codes.InvalidArgument<br/>empty / length / charset"]
    MAP --> IN["codes.Internal<br/>everything else"]
    RESP --> C
    IA --> C
    IN --> C
```

Notes:
- The adapter holds the inbound port as `users.API`, so the transport cannot reach past the application
  boundary into the domain or the repository.
- `UserService` embeds `userv1.UnimplementedUserServiceServer`, which is what satisfies the generated
  `UserServiceServer` interface and keeps the adapter forward compatible when new RPCs are added.
- Error translation lives in one place (`toStatusError`) and relies on `errors.Is`, because the service
  wraps the domain sentinels with `%w`.
- Cancellation, deadlines and metadata arrive as a plain `context.Context`, which the core already takes
  as its first argument — a gRPC deadline therefore propagates all the way to the repo call.
- Verified live: `CreateAccount(charlie)` returns a `userId`; `CreateAccount("b")` returns
  `code = InvalidArgument, desc = "invalid username supplied: username must be between 5 and 100 characters"`.
