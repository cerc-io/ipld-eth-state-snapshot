BIN = $(GOPATH)/bin

## Mockgen tool
MOCKGEN = $(BIN)/mockgen
$(BIN)/mockgen:
	go install github.com/golang/mock/mockgen@v1.6.0

MOCKS_DIR = $(CURDIR)/mocks

.PHONY: mocks test

mocks: $(MOCKGEN) mocks/snapshot/publisher.go

mocks/snapshot/publisher.go: pkg/types/publisher.go
	$(MOCKGEN) -package snapshot_mock -destination $@ -source $< Publisher Tx

clean:
	rm -f mocks/snapshot/publisher.go

build:
	go fmt ./...
	go build

test: mocks
	go clean -testcache && go test -p 1 -v ./...

dbtest: mocks
	go clean -testcache && TEST_WITH_DB=true go test -p 1 -v ./...
