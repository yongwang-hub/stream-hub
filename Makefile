# 声明以下为"伪目标"（不对应磁盘文件，每次都执行），避免与同名文件冲突
.PHONY: build test race vet fmt run migrate smoke clean frontend amd64 arm64 all

# 本地编译产物的输出路径（bin/ 已被 .gitignore 忽略）
BINARY := bin/stream-hub

# 版本号：与 scripts/build.mjs 同一来源——读取 package.json 的 version，统一加 V 前缀（如 0.1.0 -> V0.1.0）
VERSION := V$(shell grep -m1 '"version"' package.json | sed -E 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
# 日期：取 YYMMDD，与 build.mjs 的发布包目录命名保持一致
DATE := $(shell date +%y%m%d)

# build：编译【当前平台】的二进制到 bin/stream-hub（仅本地编译，不会打包成发布目录）
build:
	go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BINARY) ./cmd/server

# test：运行全部单元测试（-count=1 禁用缓存，保证结果是真实的）
test:
	go test ./... -count=1

# race：带竞态检测器跑测试，用来发现并发相关的隐藏 bug
race:
	go test ./... -race -count=1

# vet：静态检查，在编译之外扫描可疑代码（如 printf 格式串不匹配、不可达代码、锁被值拷贝等）
vet:
	go vet ./...

# fmt：格式检查，列出不符合 gofmt 规范的文件（只列不改；要自动改格式用 gofmt -w）
fmt:
	gofmt -l ./cmd ./internal

# run：启动服务（显式指定配置文件路径）
run:
	go run ./cmd/server -config config/config.yaml

# migrate：单独执行数据库迁移/建表（不启动 HTTP 服务，供部署流水线 / CI 在启动前先建表）
migrate:
	go run ./cmd/migrate -config config/config.yaml

# smoke：冒烟测试——先编译，再请求 /healthz 探活，验证服务能正常起来
smoke: build
	curl -fsS http://127.0.0.1:8080/healthz

# clean：清理编译产物（删除本地二进制 bin/ 和发布包目录 package/）
clean:
	rm -f $(BINARY)
	rm -rf package/

# frontend：确保前端产物 web/dist 存在；不存在时自动用 pnpm 构建（需要 pnpm 已安装）
frontend:
	@test -d web/dist || pnpm -C web build

# build-pkg：交叉编译并打包成与 build.mjs 一致的发布目录（内部目标，一般不直接调用）
#   产物：package/stream-hub-linux-<arch>-<V版本>.<日期>/  （内含二进制 stream-hub + dist/ 前端）
#   用法见下方的 amd64 / arm64 / all
build-pkg: frontend
	@mkdir -p package/stream-hub-linux-$(ARCH)-$(VERSION).$(DATE)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o package/stream-hub-linux-$(ARCH)-$(VERSION).$(DATE)/stream-hub ./cmd/server
	cp -r web/dist package/stream-hub-linux-$(ARCH)-$(VERSION).$(DATE)/dist
	@echo "built: package/stream-hub-linux-$(ARCH)-$(VERSION).$(DATE)/"

# amd64：出 linux/amd64 平台的发布包
amd64:
	$(MAKE) build-pkg GOOS=linux GOARCH=amd64 ARCH=amd64

# arm64：出 linux/arm64 平台的发布包
arm64:
	$(MAKE) build-pkg GOOS=linux GOARCH=arm64 ARCH=arm64

# all：一次性出 amd64 + arm64 两个发布包
all: amd64 arm64
