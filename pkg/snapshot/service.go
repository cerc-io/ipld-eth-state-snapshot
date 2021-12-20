// Copyright © 2020 Vulcanize, Inc
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package snapshot

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/statediff/indexer/postgres"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/sirupsen/logrus"
)

var (
	nullHash          = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	emptyNode, _      = rlp.EncodeToBytes([]byte{})
	emptyCodeHash     = crypto.Keccak256([]byte{})
	emptyContractRoot = crypto.Keccak256Hash(emptyNode)
)

// Service holds ethDB and stateDB to read data from lvldb and Publisher
// to publish trie in postgres DB.
type Service struct {
	ethDB         ethdb.Database
	stateDB       state.Database
	ipfsPublisher *Publisher
}

// NewSnapshotService creates Service.
func NewSnapshotService(con *Config) (*Service, error) {
	pgDB, err := postgres.NewDB(con.connectionURI, con.DBConfig, con.Node)
	if err != nil {
		return nil, err
	}

	edb, err := rawdb.NewLevelDBDatabaseWithFreezer(con.LevelDBPath, 1024, 256, con.AncientDBPath, "eth-pg-ipfs-state-snapshot", false)
	if err != nil {
		return nil, err
	}

	return &Service{
		ethDB:         edb,
		stateDB:       state.NewDatabase(edb),
		ipfsPublisher: NewPublisher(pgDB),
	}, nil
}

// CreateLatestSnapshot creates snapshot for the latest block.
func (s *Service) CreateLatestSnapshot() error {
	// extract header from lvldb and publish to PG-IPFS
	// hold onto the headerID so that we can link the state nodes to this header
	logrus.Info("Creating snapshot at head")

	hash := rawdb.ReadHeadHeaderHash(s.ethDB)
	height := rawdb.ReadHeaderNumber(s.ethDB, hash)
	if height == nil {
		return fmt.Errorf("unable to read header height for header hash %s", hash.String())
	}

	header := rawdb.ReadHeader(s.ethDB, hash, *height)
	if header == nil {
		return fmt.Errorf("unable to read canonical header at height %d", height)
	}

	logrus.Infof("head hash: %s head height: %d", hash.Hex(), *height)

	headerID, err := s.ipfsPublisher.PublishHeader(header)
	if err != nil {
		return err
	}

	t, err := s.stateDB.OpenTrie(header.Root)
	if err != nil {
		return err
	}

	trieDB := s.stateDB.TrieDB()
	return s.createSnapshot(t.NodeIterator([]byte{}), trieDB, headerID)
}

// CreateSnapshot creates snapshot for given block height.
func (s *Service) CreateSnapshot(height uint64) error {
	// extract header from lvldb and publish to PG-IPFS
	// hold onto the headerID so that we can link the state nodes to this header
	logrus.Infof("Creating snapshot at height %d", height)
	hash := rawdb.ReadCanonicalHash(s.ethDB, height)
	header := rawdb.ReadHeader(s.ethDB, hash, height)
	if header == nil {
		return fmt.Errorf("unable to read canonical header at height %d", height)
	}

	headerID, err := s.ipfsPublisher.PublishHeader(header)
	if err != nil {
		return err
	}

	t, err := s.stateDB.OpenTrie(header.Root)
	if err != nil {
		return err
	}

	trieDB := s.stateDB.TrieDB()
	return s.createSnapshot(t.NodeIterator([]byte{}), trieDB, headerID)
}

func (s *Service) createSnapshot(it trie.NodeIterator, trieDB *trie.Database, headerID int64) error {
	for it.Next(true) {
		if it.Leaf() { // "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
			continue
		}

		if bytes.Equal(nullHash.Bytes(), it.Hash().Bytes()) {
			continue
		}

		nodePath := make([]byte, len(it.Path()))
		copy(nodePath, it.Path())

		var (
			nodeData []byte
			ty       nodeType
			err      error
		)

		nodeData, err = trieDB.Node(it.Hash())
		if err != nil {
			return err
		}

		var nodeElements []interface{}
		if err = rlp.DecodeBytes(nodeData, &nodeElements); err != nil {
			return err
		}

		ty, err = CheckKeyType(nodeElements)
		if err != nil {
			return err
		}

		stateNode := &node{
			nodeType: ty,
			path:     nodePath,
			value:    nodeData,
		}

		switch ty {
		case leaf:
			// if the node is a leaf, decode the account and publish the associated storage trie nodes if there are any
			var account types.StateAccount
			if err = rlp.DecodeBytes(nodeElements[1].([]byte), &account); err != nil {
				return fmt.Errorf("error decoding account for leaf node at path %x nerror: %w", nodePath, err)
			}

			partialPath := trie.CompactToHex(nodeElements[0].([]byte))
			valueNodePath := append(nodePath, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			stateNode.key = common.BytesToHash(leafKey)

			stateID, err := s.ipfsPublisher.PublishStateNode(stateNode, headerID)
			if err != nil {
				return err
			}

			// publish any non-nil code referenced by codehash
			if !bytes.Equal(account.CodeHash, emptyCodeHash) {
				codeHash := common.BytesToHash(account.CodeHash)
				codeBytes := rawdb.ReadCode(s.ethDB, codeHash)
				if len(codeBytes) == 0 {
					logrus.Error("Code is missing", "account", common.BytesToHash(it.LeafKey()))
					return errors.New("missing code")
				}

				if err = s.ipfsPublisher.PublishCode(codeHash, codeBytes); err != nil {
					return err
				}
			}

			if err = s.storageSnapshot(account.Root, stateID); err != nil {
				return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %w", account, err)
			}
		case extension, branch:
			stateNode.key = common.BytesToHash([]byte{})
			if _, err := s.ipfsPublisher.PublishStateNode(stateNode, headerID); err != nil {
				return err
			}
		default:
			return errors.New("unexpected node type")
		}
	}
	return it.Error()
}

func (s *Service) storageSnapshot(sr common.Hash, stateID int64) error {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return nil
	}

	sTrie, err := s.stateDB.OpenTrie(sr)
	if err != nil {
		return err
	}

	it := sTrie.NodeIterator(make([]byte, 0))
	for it.Next(true) {
		// skip value nodes
		if it.Leaf() {
			continue
		}

		if bytes.Equal(nullHash.Bytes(), it.Hash().Bytes()) {
			continue
		}

		nodePath := make([]byte, len(it.Path()))
		copy(nodePath, it.Path())

		var (
			nodeData []byte
			ty       nodeType
		)

		nodeData, err = s.stateDB.TrieDB().Node(it.Hash())
		if err != nil {
			return err
		}

		var nodeElements []interface{}
		if err = rlp.DecodeBytes(nodeData, &nodeElements); err != nil {
			return err
		}

		ty, err = CheckKeyType(nodeElements)
		if err != nil {
			return err
		}

		storageNode := &node{
			nodeType: ty,
			path:     nodePath,
			value:    nodeData,
		}

		switch ty {
		case leaf:
			partialPath := trie.CompactToHex(nodeElements[0].([]byte))
			valueNodePath := append(nodePath, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			storageNode.key = common.BytesToHash(leafKey)
		case extension, branch:
			storageNode.key = common.BytesToHash([]byte{})
		default:
			return errors.New("unexpected node type")
		}

		if err = s.ipfsPublisher.PublishStorageNode(storageNode, stateID); err != nil {
			return err
		}
	}

	return it.Error()
}
