PROJECT_NAME := "github.com/sohaha/zzz"
PKG := "$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)

.PHONY: all dep lint vet test test-coverage build clean

all: build

dep: ## 获取依赖
	@go mod download

lint: ## Go 代码静态检查
	@golint -set_exit_status ${PKG_LIST}

vet: ## 运行 go vet
	@go vet ${PKG_LIST}

test: ## 运行单元测试
	@go test -short ${PKG_LIST}

test-coverage: ## 运行覆盖率测试
	@go test -short -coverprofile cover.out -covermode=atomic ${PKG_LIST}
	@cat cover.out >> coverage.txt

.PHONY: build
build: dep ## 构建可执行文件
	@go build -o build/main $(PKG)

.PHONY: install
install: dep ## 安装可执行文件
	@go install -ldflags '$(LDFLAGS)'

.PHONY: clean
clean: ## 清理旧的构建产物
	@rm -f ./build

.PHONY: help
help: ## 显示此帮助列表
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: docker-image
docker-image:
	docker build -t seekwe/zzz:v1.0.0 -f ./Dockerfile .

.PHONY: push-docker-image
push-docker-image:
	docker push seekwe/zzz:v1.0.0