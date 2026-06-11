.PHONY: all build test clean dev dev-frontend docker-build docker-login docker-push docker-release docker-run docker-stop seed fmt lint update help

APP_NAME  := note
GO_SRC    := ./cmd/note
VITE_DIR  := web/frontend
STATIC_DIR := web/static
DOCKER_IMAGE ?= rag-note
TAG ?= latest
ALIYUN_REGISTRY := crpi-51pd4blge4jwd9y0.cn-hangzhou.personal.cr.aliyuncs.com
ALIYUN_NAMESPACE := hakuming-images
ALIYUN_IMAGE := $(ALIYUN_REGISTRY)/$(ALIYUN_NAMESPACE)/$(APP_NAME):$(TAG)

# 版本信息（注入到 gateway 包）
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT := $(shell git rev-parse HEAD)
BUILD_DATE := $(shell date -u +%Y-%m-%d.%H:%M:%S)
LDFLAGS := -s -w \
	-X note/internal/gateway.GitBranch=$(GIT_BRANCH) \
	-X note/internal/gateway.GitCommit=$(GIT_COMMIT) \
	-X note/internal/gateway.BuildDate=$(BUILD_DATE)

# ---- 开发 ----

# 启动后端（前端构建到 web/static，Go 服务统一托管）
dev: build-frontend
	@echo ""
	@echo "  ╔══════════════════════════════════════╗"
	@echo "  ║  🌐  http://localhost:8080          ║"
	@echo "  ║  📝  RAG Note (Go + Vue)            ║"
	@echo "  ║  $(GIT_BRANCH) $(shell echo $(GIT_COMMIT) | cut -c1-7)                 ║"
	@echo "  ╚══════════════════════════════════════╝"
	@echo ""
	@go run $(GO_SRC) server

# 启动前端开发服务器（:5173，API 代理到 :8080）
dev-frontend:
	@echo "==> Vite dev server on :5173"
	@cd $(VITE_DIR) && npm run dev

# ---- 构建 ----

# 构建全部：前端 + Go 二进制
all: build-frontend build-server
	@echo "==> Build complete: $(APP_NAME) [$(GIT_BRANCH) $(shell echo $(GIT_COMMIT) | cut -c1-7)]"

# 构建前端产物到 web/static
build-frontend:
	@echo "==> Building frontend..."
	@cd $(VITE_DIR) && npm ci && npm run build

# 构建 Go 二进制（本地架构）
build-server:
	@echo "==> Building server: $(APP_NAME) [$(GIT_BRANCH) $(shell echo $(GIT_COMMIT) | cut -c1-7)] $(BUILD_DATE)"
	@CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME) $(GO_SRC)

# 交叉编译 Linux amd64（用于 Docker/服务器部署）
build-linux:
	@echo "==> Building server (linux/amd64): $(APP_NAME)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME) $(GO_SRC)

# ---- 测试 ----

test:
	@echo "==> Running tests..."
	@go test ./...

test-verbose:
	@echo "==> Running tests (verbose)..."
	@go test -v ./...

test-cover:
	@echo "==> Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "==> Coverage report: coverage.html"

# ---- 依赖管理 ----

# 更新所有 Go 依赖
update:
	@echo "==> Updating dependencies..."
	@go list -m -u all 2>/dev/null || true
	@go get -u ./...
	@go mod tidy
	@echo "==> Dependencies updated"

# ---- 演示数据 ----

seed:
	@echo "==> Generating demo notes..."
	@go run $(GO_SRC) seed

# ---- Docker ----

# 本地构建（基础镜像走阿里云）
docker-build:
	@echo "==> Building Docker image: $(DOCKER_IMAGE)"
	@docker build -f Dockerfile.local -t $(DOCKER_IMAGE) .

# 登录阿里云镜像仓库
docker-login:
	@echo "==> Logging into Alibaba Cloud registry..."
	@docker login $(ALIYUN_REGISTRY)

# 推送镜像到阿里云
docker-push:
	@echo "==> Pushing to Alibaba Cloud: $(ALIYUN_IMAGE)"
	@docker tag $(DOCKER_IMAGE) $(ALIYUN_IMAGE)
	@docker push $(ALIYUN_IMAGE)
	@echo "==> Done: $(ALIYUN_IMAGE)"

# 一键构建 + 推送
docker-release: docker-build docker-push
	@echo "==> Release complete: $(ALIYUN_IMAGE)"

docker-run:
	@echo "==> Running Docker container on :8080"
	@docker run -d --name $(APP_NAME) \
		-p 8080:8080 \
		-v $(APP_NAME)-data:/app/data \
		$(DOCKER_IMAGE)

docker-stop:
	@echo "==> Stopping container: $(APP_NAME)"
	@docker stop $(APP_NAME) 2>/dev/null || true
	@docker rm $(APP_NAME) 2>/dev/null || true

# ---- 代码质量 ----

fmt:
	@echo "==> Formatting Go code..."
	@go fmt ./...

# 优先用 golangci-lint，没有则 fallback 到 go vet
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "==> Running golangci-lint..."; \
		golangci-lint run ./...; \
	else \
		echo "==> Running go vet (install golangci-lint for better results)..."; \
		go vet ./...; \
	fi

# ---- 清理 ----

clean:
	@echo "==> Cleaning build artifacts..."
	@rm -f $(APP_NAME)
	@rm -rf $(STATIC_DIR)/*
	@rm -f coverage.out coverage.html
	@echo "==> Clean complete"

# ---- 帮助 ----

help:
	@echo ""
	@echo "  make dev              - Build frontend + start Go server"
	@echo "  make dev-frontend     - Start Vite dev server"
	@echo "  make all              - Build frontend + Go binary"
	@echo "  make build-frontend   - Build Vue frontend only"
	@echo "  make build-server     - Build Go binary (local arch)"
	@echo "  make build-linux      - Build Go binary (linux/amd64)"
	@echo "  make test             - Run all tests"
	@echo "  make test-cover       - Run tests with coverage HTML report"
	@echo "  make update           - Update all Go dependencies"
	@echo "  make seed             - Generate demo notes"
	@echo "  make docker-build     - Build Docker image (local, via Aliyun)"
	@echo "  make docker-login     - Log into Alibaba Cloud registry"
	@echo "  make docker-push      - Push image to Alibaba Cloud"
	@echo "  make docker-release   - Build + push (one command)"
	@echo "  make docker-run       - Start Docker container"
	@echo "  make docker-stop      - Stop & remove Docker container"
	@echo "  make fmt              - go fmt ./..."
	@echo "  make lint             - golangci-lint (or go vet)"
	@echo "  make clean            - Remove build artifacts"
	@echo ""

# ---- 版本信息 ----

version:
	@echo "branch:  $(GIT_BRANCH)"
	@echo "commit:  $(GIT_COMMIT)"
	@echo "date:    $(BUILD_DATE)"
