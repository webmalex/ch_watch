BINARY := ch_watch
CMD := ./cmd/ch_watch
BIN_DIR := ./bin
OUTPUT := $(BIN_DIR)/$(BINARY)

.PHONY: build install clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(OUTPUT) $(CMD)

install:
	go install $(CMD)

clean:
	rm -rf $(BIN_DIR)
