MOCKS_DIR = $(CURDIR)/mocks
mockgen_cmd=mockgen

.PHONY: mocks

mocks: mocks/snapshot/publisher.go

mocks/snapshot/publisher.go: pkg/types/publisher.go
	$(mockgen_cmd) -package snapshot_mock -destination $@ -source $< Publisher Tx

clean:
	rm -f mocks/snapshot/publisher.go

build:
	go fmt ./...
	go build
