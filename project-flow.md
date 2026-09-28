# Project Flow

```mermaid
flowchart LR
    U[Client / Browser] -->|POST /users, JSON| A[Primary adapter: Fiber web app]
    G[gRPC client] -->|CreateAccount RPC| GA[Primary adapter: gRPC server]

    A --> R[Route: /users]
    R --> H[Handler: CreateAccount]
    H --> S[Core service: Users Service]

    GA --> C[Contract: gen/user/v1 user.v1.UserService]
    C --> GH[gRPC service: UserService.CreateAccount]
    GH --> S

    S --> D[Domain logic / validation]
    S --> P[Port: UserRepository interface]
    P --> I[Secondary adapter: Postgres repo]
    I --> DB[(Postgres database)]

    DB --> I
    I --> P
    P --> S
    S --> H
    S --> GH
    GH --> C
    C --> G
    H --> A
    A --> U

    subgraph CompositionRoot[Main wiring - cmd/api]
        M[Runs both adapters on one shared service]
    end

    M --> I
    M --> S
    M --> A
    M --> GA
```

## Explanation

1. A request enters through either the web adapter (REST) or the gRPC adapter.
2. The route / registered RPC method calls the user account handler.
3. The handler delegates to the same core service in both cases.
4. The service applies domain validation and business rules.
5. It depends on a repository port, implemented by the Postgres adapter.
6. The adapter persists the data in the database.
7. Results flow back to the client — as JSON over HTTP, or as a protobuf message with a gRPC status code.

This is a hexagonal architecture pattern: the core business logic is isolated from HTTP, gRPC and database
details, which is why swapping in gRPC only added an adapter plus a generated contract.
