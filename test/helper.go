package test

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	ethnode "github.com/ethereum/go-ethereum/statediff/indexer/node"
)

var (
	DefaultNodeInfo = ethnode.Info{
		ID:           "test_nodeid",
		ClientName:   "test_client",
		GenesisBlock: "TEST_GENESIS",
		NetworkID:    "test_network",
		ChainID:      0,
	}
	DefaultPgConfig = postgres.Config{
		Hostname:     "localhost",
		Port:         5432,
		DatabaseName: "vulcanize_test",
		Username:     "vulcanize",
		Password:     "vulcanize_password",

		MaxIdle:         0,
		MaxConnLifetime: 0,
		MaxConns:        4,
	}
)

func NeedsDB(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_WITH_DB") == "" {
		t.Skip("set TEST_WITH_DB to enable test")
	}
}

func NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ExpectEqual asserts the provided interfaces are deep equal
func ExpectEqual(t *testing.T, want, got interface{}) {
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("Values not equal:\nExpected:\t%v\nActual:\t\t%v", want, got)
	}
}

func ExpectEqualBytes(t *testing.T, want, got []byte) {
	if !bytes.Equal(want, got) {
		t.Fatalf("Bytes not equal:\nExpected:\t%v\nActual:\t\t%v", want, got)
	}
}
