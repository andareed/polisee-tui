.PHONY: build run check race smoke
build:
	go build -buildvcs=false -o bin/polisee-tui .
run:
	go run -buildvcs=false .
check:
	@test -z "$$(gofmt -l .)" || (gofmt -l .; exit 1)
	go vet ./...
	go test ./...
	go build -buildvcs=false ./...
race:
	go test -race ./...
smoke: build
	python3 scripts/smoke.py
