BINARY := go-l2ping
BUILD_DIR := $(shell pwd)/build
SOURCES = main.go

.PHONY: build
build: $(BUILD_DIR)/$(BINARY)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/$(BINARY): $(BUILD_DIR) $(SOURCES)
	GOOS=linux GOARCH=arm GOARM=5 go build -o $(BUILD_DIR)/$(BINARY) .

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
