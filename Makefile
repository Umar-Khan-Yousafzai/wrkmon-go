BINARY := wrkmon-go
PKG    := github.com/Umar-Khan-Yousafzai/wrkmon-go
CMD    := ./cmd/wrkmon-go

.PHONY: build run clean test lint

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...
