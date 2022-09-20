package publisher

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/statediff/indexer/test_helpers"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"

	fixt "github.com/cerc-io/ipld-eth-state-snapshot/fixture"
	snapt "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
	"github.com/cerc-io/ipld-eth-state-snapshot/test"
)

var (
	pgConfig = test.DefaultPgConfig
	nodeInfo = test.DefaultNodeInfo
	// tables ordered according to fkey depedencies
	allTables = []*snapt.Table{
		&snapt.TableIPLDBlock,
		&snapt.TableNodeInfo,
		&snapt.TableHeader,
		&snapt.TableStateNode,
		&snapt.TableStorageNode,
	}
)

func writeFiles(t *testing.T, dir string) *publisher {
	pub, err := NewPublisher(dir, nodeInfo)
	test.NoError(t, err)
	test.NoError(t, pub.PublishHeader(&fixt.Block1_Header))
	tx, err := pub.BeginTx()
	test.NoError(t, err)

	headerID := fixt.Block1_Header.Hash().String()
	test.NoError(t, pub.PublishStateNode(&fixt.Block1_StateNode0, headerID, fixt.Block1_Header.Number, tx))

	test.NoError(t, tx.Commit())
	return pub
}

// verify that we can parse the csvs
// TODO check actual data
func verifyFileData(t *testing.T, path string, tbl *snapt.Table) {
	file, err := os.Open(path)
	test.NoError(t, err)
	r := csv.NewReader(file)
	test.NoError(t, err)
	r.FieldsPerRecord = len(tbl.Columns)

	for {
		_, err := r.Read()
		if err == io.EOF {
			break
		}
		test.NoError(t, err)
	}
}

func TestWriting(t *testing.T) {
	dir := t.TempDir()
	// tempdir like /tmp/TempFoo/001/, TempFoo defaults to 0700
	test.NoError(t, os.Chmod(filepath.Dir(dir), 0755))

	pub := writeFiles(t, dir)

	for _, tbl := range perBlockTables {
		verifyFileData(t, TableFile(pub.dir, tbl.Name), tbl)
	}
	for i := uint32(0); i < pub.txCounter; i++ {
		for _, tbl := range perNodeTables {
			verifyFileData(t, TableFile(pub.txDir(i), tbl.Name), tbl)
		}
	}
}

// Note: DB user requires role membership "pg_read_server_files"
func TestPgCopy(t *testing.T) {
	test.NeedsDB(t)

	dir := t.TempDir()
	test.NoError(t, os.Chmod(filepath.Dir(dir), 0755))
	pub := writeFiles(t, dir)

	ctx := context.Background()
	driver, err := postgres.NewSQLXDriver(ctx, pgConfig, nodeInfo)
	test.NoError(t, err)
	db := postgres.NewPostgresDB(driver)

	test_helpers.TearDownDB(t, db)

	// copy from files
	pgCopyStatement := `COPY %s FROM '%s' CSV`
	for _, tbl := range perBlockTables {
		stm := fmt.Sprintf(pgCopyStatement, tbl.Name, TableFile(pub.dir, tbl.Name))
		_, err = db.Exec(ctx, stm)
		test.NoError(t, err)
	}
	for i := uint32(0); i < pub.txCounter; i++ {
		for _, tbl := range perNodeTables {
			stm := fmt.Sprintf(pgCopyStatement, tbl.Name, TableFile(pub.txDir(i), tbl.Name))
			_, err = db.Exec(ctx, stm)
			test.NoError(t, err)
		}
	}

	// check header was successfully committed
	pgQueryHeader := `SELECT cid, block_hash
					  FROM eth.header_cids
				      WHERE block_number = $1`
	type res struct {
		CID       string
		BlockHash string
	}
	var header res
	err = db.QueryRow(ctx, pgQueryHeader, fixt.Block1_Header.Number.Uint64()).Scan(
		&header.CID, &header.BlockHash)
	test.NoError(t, err)

	headerNode, err := ipld.NewEthHeader(&fixt.Block1_Header)
	test.NoError(t, err)
	test.ExpectEqual(t, headerNode.Cid().String(), header.CID)
	test.ExpectEqual(t, fixt.Block1_Header.Hash().String(), header.BlockHash)
}
