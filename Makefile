.PHONY: build test web web-install run clean

build:
	go build -o bin/sentinel ./cmd/sentinel

test:
	go test ./...

web-install:
	cd web && npm install

web:
	cd web && npm run build

run: build
	./bin/sentinel

clean:
	rm -rf bin web/dist
