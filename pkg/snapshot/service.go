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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/sirupsen/logrus"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/postgres"
)

var (
	nullHash          = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	emptyNode, _      = rlp.EncodeToBytes([]byte{})
	emptyContractRoot = crypto.Keccak256Hash(emptyNode)
)

type Service struct {
	ethDB         ethdb.Database
	stateDB       state.Database
	ipfsPublisher *Publisher
}

func NewSnapshotService(con Config) (*Service, error) {
	pgdb, err := postgres.NewDB(con.DBConfig, con.Node)
	if err != nil {
		return nil, err
	}
	edb, err := rawdb.NewLevelDBDatabase(con.LevelDBPath, 256, 1024, "eth-pg-ipfs-state-snapshot")
	if err != nil {
		return nil, err
	}
	return &Service{
		ethDB:         edb,
		stateDB:       state.NewDatabase(edb),
		ipfsPublisher: NewPublisher(pgdb),
	}, nil
}

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
		node, err := trieDB.Node(it.Hash())
		if err != nil {
			return err
		}
		var nodeElements []interface{}
		if err := rlp.DecodeBytes(node, &nodeElements); err != nil {
			return err
		}
		ty, err := CheckKeyType(nodeElements)
		if err != nil {
			return err
		}
		stateNode := Node{
			NodeType: ty,
			Path:     nodePath,
			Value:    node,
		}
		switch ty {
		case Leaf:
			// if the node is a leaf, decode the account and if publish the associated storage trie nodes if there are any
			var account state.Account
			if err := rlp.DecodeBytes(nodeElements[1].([]byte), &account); err != nil {
				return fmt.Errorf("error decoding account for leaf node at path %x nerror: %v", nodePath, err)
			}
			partialPath := trie.CompactToHex(nodeElements[0].([]byte))
			valueNodePath := append(nodePath, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			stateNode.Key = common.BytesToHash(leafKey)
			stateID, err := s.ipfsPublisher.PublishStateNode(stateNode, headerID)
			if err != nil {
				return err
			}
			if err := s.storageSnapshot(account.Root, stateID); err != nil {
				return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %v", account, err)
			}
		case Extension, Branch:
			stateNode.Key = common.BytesToHash([]byte{})
			if _, err := s.ipfsPublisher.PublishStateNode(stateNode, headerID); err != nil {
				return err
			}
		default:
			return errors.New("unexpected node type")
		}
	}
	return nil
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
		node, err := s.stateDB.TrieDB().Node(it.Hash())
		if err != nil {
			return err
		}
		var nodeElements []interface{}
		if err := rlp.DecodeBytes(node, &nodeElements); err != nil {
			return err
		}
		ty, err := CheckKeyType(nodeElements)
		if err != nil {
			return err
		}
		storageNode := Node{
			NodeType: ty,
			Path:     nodePath,
			Value:    node,
		}
		switch ty {
		case Leaf:
			partialPath := trie.CompactToHex(nodeElements[0].([]byte))
			valueNodePath := append(nodePath, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			storageNode.Key = common.BytesToHash(leafKey)
		case Extension, Branch:
			storageNode.Key = common.BytesToHash([]byte{})
		default:
			return errors.New("unexpected node type")
		}
		if err := s.ipfsPublisher.PublishStorageNode(storageNode, stateID); err != nil {
			return err
		}
	}
	return nil
}
