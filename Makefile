PROJECT_NAME := "github.com/sohaha/zzz"
PKG := "$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)

.PHONY: all dep lint vet test test-coverage build clean

all: build

dep: ## Get the dependencies
	@go mod download

lint: ## Lint Golang files
	@golint -set_exit_status ${PKG_LIST}

vet: ## Run go vet
	@go vet ${PKG_LIST}

test: ## Run unittests
	@go test -short ${PKG_LIST}

test-coverage: ## Run tests with coverage
	@go test -short -coverprofile cover.out -covermode=atomic ${PKG_LIST}
	@cat cover.out >> coverage.txt

.PHONY: build
build: dep ## Build the binary file
	@go build -i -o build/main $(PKG)

.PHONY: install
install: dep ## Install the binary file
	@go install -ldflags '$(LDFLAGS)'

.PHONY: clean
clean: ## Remove previous build
	@rm -f ./build

.PHONY: help
help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: docker-image
docker-image:
	docker build -t seekwe/zzz:v1.0.0 -f ./Dockerfile .

.PHONY: push-docker-image
push-docker-image:
	docker push seekwe/zzz:v1.0.0