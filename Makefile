.PHONY: build test check todo-generate todo-e2e todo-backend-generate todo-backend-e2e overrides-generate overrides-e2e

build:
	go build -o bin/sqlc-gen-ocaml ./cmd/sqlc-gen-ocaml

test:
	go test ./...

check: test
	go vet ./...

todo-generate:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/sqlc-gen-ocaml-linux ./cmd/sqlc-gen-ocaml
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/todo sqlc/sqlc:1.30.0 generate

todo-e2e: todo-generate
	cd examples/todo && ./e2e.sh

todo-backend-generate:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/sqlc-gen-ocaml-linux ./cmd/sqlc-gen-ocaml
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/todo_backend sqlc/sqlc:1.30.0 generate

todo-backend-e2e: todo-backend-generate
	cd examples/todo_backend && ./e2e.sh

overrides-generate:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/sqlc-gen-ocaml-linux ./cmd/sqlc-gen-ocaml
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/overrides sqlc/sqlc:1.30.0 generate

overrides-e2e: overrides-generate
	cd examples/overrides && ./e2e.sh
