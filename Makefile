.PHONY: build test vet lint run tidy cross

build:
	go build -o unsni ./cmd/unsni

test:
	go test -race ./...

vet:
	go vet ./...

tidy:
	go mod tidy

run: build
	./unsni run

# Cross-compile smoke check (cgo disabled, as designed for phase 1).
cross:
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null ./cmd/unsni
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null ./cmd/unsni
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o /dev/null ./cmd/unsni
