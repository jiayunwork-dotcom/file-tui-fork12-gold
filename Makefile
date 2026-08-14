.PHONY: build run clean test

BINARY_NAME=file-tui
CMD_DIR=./cmd/file-tui

build:
	go build -o $(BINARY_NAME) $(CMD_DIR)

run:
	go run $(CMD_DIR)

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux $(CMD_DIR)

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-mac $(CMD_DIR)

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows.exe $(CMD_DIR)

build-all: build-linux build-mac build-windows

test:
	go test ./...

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-linux $(BINARY_NAME)-mac $(BINARY_NAME)-windows.exe
