# Makefile for ipfailover tests extension
# This delegates to the actual test implementation in ipfailover/keepalived/tests/

ACTUAL_TEST_DIR := ../ipfailover/keepalived/tests
BINARY := ipfailover-tests-ext

.PHONY: build
build:
	@echo "Building ipfailover test extension..."
	$(MAKE) -C $(ACTUAL_TEST_DIR) build
	@mkdir -p bin
	@cp $(ACTUAL_TEST_DIR)/bin/$(BINARY) bin/$(BINARY)
	@echo "✅ Binary copied to tests-extension/bin/$(BINARY)"

.PHONY: verify
verify:
	@echo "Running verification..."
	@if [ ! -d "$(ACTUAL_TEST_DIR)" ]; then \
		echo "❌ ERROR: Test directory $(ACTUAL_TEST_DIR) not found"; \
		exit 1; \
	fi
	@echo "✅ Test directory structure verified"

.PHONY: verify-metadata
verify-metadata:
	@echo "Verifying metadata..."
	@if [ ! -f "$(ACTUAL_TEST_DIR)/cmd/main.go" ]; then \
		echo "❌ ERROR: main.go not found"; \
		exit 1; \
	fi
	@echo "✅ Test extension metadata verified"

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	$(MAKE) -C $(ACTUAL_TEST_DIR) clean || true
	@echo "✅ Cleaned"
