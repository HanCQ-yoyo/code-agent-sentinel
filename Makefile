.PHONY: build test web web-install run clean release build-cross build-cross-fast

# ---- 常量 ----
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DIST      = dist
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# ---- 本地开发 ----
build: web
	go build -o bin/sentinel ./cmd/sentinel

test:
	go test ./...

web-install:
	cd web && npm install

web:
	cd web && npm run build
	rm -rf internal/api/web_dist
	cp -r web/dist internal/api/web_dist
	# 保留 embed 所需(已是 dist 内容)

run: build
	./bin/sentinel

clean:
	rm -rf bin web/dist internal/api/web_dist/assets dist
	@# 恢复 embed 所需的 index.html 占位符(make web 会整体替换 web_dist)
	@printf '<!-- placeholder; run make web -->\n' > internal/api/web_dist/index.html

# ---- 内部辅助: 构建+打包单个平台(纯 shell) ----
# 用法: $(call build-one,goos,goarch)
#   前端已 embed 进二进制,产物只需单个可执行文件,打包为 tar.gz / zip。
#   若 dist/sentinel-<os>-<arch>-<commit>.{tar.gz,zip} 已存在则跳过。
define build-one
	@GOOS="$(1)"; GOARCH="$(2)"; \
	if [ "$$GOOS" = "windows" ]; then EXT=".exe"; else EXT=""; fi; \
	BASE="sentinel-$$GOOS-$$GOARCH-$(COMMIT)$$EXT"; \
	if [ "$$GOOS" = "windows" ]; then ARCHIVE="$(DIST)/$${BASE%.exe}.zip"; \
	else ARCHIVE="$(DIST)/$${BASE%.exe}.tar.gz"; fi; \
	if [ -f "$$ARCHIVE" ]; then \
		echo "  $$GOOS/$$GOARCH [$(COMMIT)] 已存在,跳过"; \
	else \
		echo "  $$GOOS/$$GOARCH -> $(DIST)/sentinel-$$GOOS-$$GOARCH$$EXT"; \
		GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 \
			go build -ldflags "-s -w" \
			-o "$(DIST)/sentinel-$$GOOS-$$GOARCH$$EXT" ./cmd/sentinel; \
		cd $(DIST); \
		if [ "$$GOOS" = "windows" ]; then \
			zip "$${BASE%.exe}.zip" "sentinel-$$GOOS-$$GOARCH$$EXT" && rm "sentinel-$$GOOS-$$GOARCH$$EXT"; \
		else \
			tar czf "$${BASE%.exe}.tar.gz" "sentinel-$$GOOS-$$GOARCH$$EXT" && rm "sentinel-$$GOOS-$$GOARCH$$EXT"; \
		fi; \
	fi
endef

# ---- 跨平台打包 ----
# 一次性构建全部平台
release: web
	@mkdir -p $(DIST)
	@echo "==> 构建 $(VERSION) [$(COMMIT)] for $(PLATFORMS)"
	$(call build-one,linux,amd64)
	$(call build-one,linux,arm64)
	$(call build-one,darwin,amd64)
	$(call build-one,darwin,arm64)
	$(call build-one,windows,amd64)
	@echo "==> 完成: $(DIST)/"
	@ls -lh $(DIST)/

# 构建单个平台(默认 linux/amd64): make build-cross [GOOS=linux] [GOARCH=arm64]
GOOS  ?= linux
GOARCH ?= amd64

build-cross: web
	@mkdir -p $(DIST)
	@echo "==> 构建 $(VERSION) [$(COMMIT)] for $(GOOS)/$(GOARCH)"
	$(call build-one,$(GOOS),$(GOARCH))
	@echo "==> 完成: $(DIST)/"
	@ls -lh $(DIST)/

# 同 build-cross,但跳过前端重建(适用于前端未改动时)
build-cross-fast:
	@mkdir -p $(DIST)
	@echo "==> 构建 $(VERSION) [$(COMMIT)] for $(GOOS)/$(GOARCH)"
	$(call build-one,$(GOOS),$(GOARCH))
	@echo "==> 完成: $(DIST)/"
	@ls -lh $(DIST)/
