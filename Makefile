.PHONY: build lint fmt test clean

build:
	go build -o ./specrun ./cmd/specrun

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

test:
	go test -race -count=1 ./...

clean:
	rm -f ./specrun ./echo_tool
