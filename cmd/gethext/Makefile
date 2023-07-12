.PHONY: gethext help clean

.DEFAULT_GOAL := gethext

GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_DATE=$(shell date)
BUILD_DIR=$(CURDIR)/build/bin

LDFLAGS:=-X 'main.gitCommit=$(GIT_COMMIT)' -X 'main.gitDate=$(GIT_DATE)'

ifeq ($(DEBUG),1)
	GCFLAGS:=all=-N -l
else 
	LDFLAGS:=$(LDFLAGS) -w -s
endif

LDFLAGS:=-ldflags="$(LDFLAGS)"
GCFLAGS:=-gcflags="$(GCFLAGS)"

help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

gethext: ## Build gethext
	@echo "Building target: $@" 
	go build $(LDFLAGS) $(GCFLAGS) -o $(BUILD_DIR)/$@ $(CURDIR)/cmd/$@
	@echo "Done building."

plugins/%: ## Build plugins
	@echo "Building target: $*" 
ifeq ($(shell go env GOOS), darwin)
	$(eval OUTPUT:=$(BUILD_DIR)/plugins/$*.dylib)
else
	$(eval OUTPUT:=$(BUILD_DIR)/plugins/$*.so)
endif
	go build -buildmode=plugin $(LDFLAGS) $(GCFLAGS) -o $(OUTPUT) $(CURDIR)/cmd/gethext/plugins/$*/main
	@echo "Done building."

clean: ## Clean build directory
	@rm -rf $(BUILD_DIR)/*

all: gethext
