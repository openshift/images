BINARY := bin/ipfailover-tests-ext

.PHONY: build
build:
	@echo "Building extension binary..."
	@mkdir -p bin
	GOTOOLCHAIN=auto GOSUMDB=sum.golang.org go build -o $(BINARY) ./cmd
	@echo "✅ Binary built: $(BINARY)"

.PHONY: clean
clean:
	@rm -f $(BINARY)

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build  - Build extension binary"
	@echo "  clean  - Remove binaries"
