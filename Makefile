.PHONY: build test web web-install run clean

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
	rm -rf bin web/dist internal/api/web_dist/assets
	@# 恢复 embed 所需的 index.html 占位符(make web 会整体替换 web_dist)
	@printf '<!-- placeholder; run make web -->\n' > internal/api/web_dist/index.html
