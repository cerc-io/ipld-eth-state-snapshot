package types

import (
	"bytes"

	"github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/common"
)

var nullHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")

func IsNullHash(hash common.Hash) bool {
	return bytes.Equal(hash.Bytes(), nullHash.Bytes())
}

func CommitOrRollback(tx Tx, err error) error {
	var rberr error
	defer func() {
		if rberr != nil {
			logrus.Errorf("rollback failed: %s", rberr)
		}
	}()
	if rec := recover(); rec != nil {
		rberr = tx.Rollback()
		panic(rec)
	} else if err != nil {
		rberr = tx.Rollback()
	} else {
		err = tx.Commit()
	}
	return err
}
