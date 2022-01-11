MOCKS_DIR = $(CURDIR)/mocks
mockgen_cmd=mockgen

.PHONY: mocks

# mocks: mocks/ethdb/database.go mocks/state/database.go mocks/snapshot/publisher.go
mocks: mocks/snapshot/publisher.go

# mocks/ethdb/database.go:
# 	$(mockgen_cmd) -package ethdb -destination $@ github.com/ethereum/go-ethereum/ethdb Database
# mocks/state/database.go:
# 	$(mockgen_cmd) -package state -destination $@ github.com/ethereum/go-ethereum/core/state Database
mocks/snapshot/publisher.go: pkg/types/publisher.go
	$(mockgen_cmd) -package snapshot_mock -destination $@ -source $< Publisher

clean:
	rm -f mocks/snapshot/publisher.go
