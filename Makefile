.PHONY: build plugin-linux example-base examples-e2e test check todo-generate todo-e2e todo-backend-generate todo-backend-e2e overrides-generate overrides-e2e async-generate async-e2e

SQLC_VERSION ?= 1.30.0
PLUGIN_GOARCH ?= $(shell go env GOARCH)
EXAMPLE_BASE_IMAGE ?= sqlc-ocaml-example-base:local
export EXAMPLE_BASE_IMAGE

build:
	go build -o bin/sqlc-gen-ocaml ./cmd/sqlc-gen-ocaml

test:
	go test ./...

check: test
	go vet ./...

plugin-linux:
	mkdir -p bin
	GOOS=linux GOARCH=$(PLUGIN_GOARCH) CGO_ENABLED=0 go build -o bin/sqlc-gen-ocaml-linux ./cmd/sqlc-gen-ocaml

example-base:
	docker build -t $(EXAMPLE_BASE_IMAGE) -f docker/example-base.Dockerfile .

todo-generate: plugin-linux
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/todo sqlc/sqlc:$(SQLC_VERSION) generate

todo-e2e: example-base todo-generate
	cd examples/todo && ./e2e.sh

todo-backend-generate: plugin-linux
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/todo_backend sqlc/sqlc:$(SQLC_VERSION) generate

todo-backend-e2e: example-base todo-backend-generate
	cd examples/todo_backend && ./e2e.sh

overrides-generate: plugin-linux
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/overrides sqlc/sqlc:$(SQLC_VERSION) generate

overrides-e2e: example-base overrides-generate
	cd examples/overrides && ./e2e.sh

async-generate: plugin-linux
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/async sqlc/sqlc:$(SQLC_VERSION) generate

async-e2e: example-base async-generate
	cd examples/async && ./e2e.sh

examples-e2e: example-base plugin-linux
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/todo sqlc/sqlc:$(SQLC_VERSION) generate
	cd examples/todo && ./e2e.sh
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/todo_backend sqlc/sqlc:$(SQLC_VERSION) generate
	cd examples/todo_backend && ./e2e.sh
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/overrides sqlc/sqlc:$(SQLC_VERSION) generate
	cd examples/overrides && ./e2e.sh
	docker run --rm -v "$(CURDIR):/src" -w /src/examples/async sqlc/sqlc:$(SQLC_VERSION) generate
	cd examples/async && ./e2e.sh
