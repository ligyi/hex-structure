GOBIN := $(shell go env GOPATH)/bin

.PHONY: tools
tools: ## Install the protobuf code generators (protoc itself comes from your package manager)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

.PHONY: proto
proto: ## Regenerate Go code from proto/user/v1
	PATH="$(GOBIN):$$PATH" protoc \
		--proto_path=. \
		--go_out=. --go_opt=module=github.com/samverrall/hex-structure \
		--go-grpc_out=. --go-grpc_opt=module=github.com/samverrall/hex-structure \
		proto/user/v1/user.proto

.PHONY: build
build: ## Build every package
	go build ./...

.PHONY: run
run: ## Run the API (REST on :8000 and gRPC on :50051)
	go run ./cmd/api

.PHONY: test
test: ## Run the test suite
	go test ./...

