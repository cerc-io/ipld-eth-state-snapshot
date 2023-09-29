MOCKGEN ?= mockgen
MOCKS_DIR := $(CURDIR)/internal/mocks

mocks: $(MOCKS_DIR)/gen_indexer.go
.PHONY: mocks

$(MOCKS_DIR)/gen_indexer.go:
	$(MOCKGEN) --package mocks --destination $@ \
		--mock_names Indexer=MockgenIndexer \
		github.com/cerc-io/plugeth-statediff/indexer Indexer

test: mocks
	go clean -testcache && go test -p 1 -v ./...
