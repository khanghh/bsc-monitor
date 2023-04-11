package extdb

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
)

type RLPUnmarshaler interface {
	UnmarshalRLP([]byte) error
}

// TrieExt store extra data for key-value pairs in trie
type TrieExt struct {
	diskdb   ethdb.Database
	trie     state.Trie
	prefix   []byte
	keyCache map[common.Hash][]byte // map hash(key) => ext key
	changes  map[common.Hash][]byte // map hash(key) => ext val
	mtx      sync.Mutex
}

func (s *TrieExt) GetExtKey(key []byte) ([]byte, error) {
	hashKey := crypto.Keccak256Hash(key)
	if extKey, exist := s.keyCache[hashKey]; exist {
		return extKey, nil
	}
	val, err := s.trie.TryGet(key)
	if err != nil {
		return nil, err
	}
	s.keyCache[hashKey] = crypto.Keccak256Hash(key, val).Bytes()
	return s.keyCache[hashKey], nil
}

// Trie returns underlying trie of the ExtTrie
func (s *TrieExt) Trie() state.Trie {
	return s.trie
}

// TryGet return custom data of key, the key is the same to original trie
func (s *TrieExt) TryGet(key []byte) ([]byte, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if cached, exist := s.changes[crypto.Keccak256Hash(key)]; exist {
		return cached, nil
	}
	extKey, err := s.GetExtKey(key)
	if err != nil {
		return nil, err
	}
	return s.diskdb.Get(append(s.prefix, extKey...))
}

// Get is the same as TryGet but ignore the error
func (s *TrieExt) Get(key []byte) []byte {
	data, _ := s.TryGet(key)
	return data
}

func (s *TrieExt) Hash() common.Hash {
	return s.trie.Hash()
}

func (s *TrieExt) RLPDecode(key []byte, val interface{}) error {
	data, err := s.TryGet(key)
	if err != nil {
		return err
	}
	if decoder, ok := val.(RLPUnmarshaler); ok {
		return decoder.UnmarshalRLP(data)
	}
	return rlp.DecodeBytes(data, &val)
}

func (s *TrieExt) TryUpdate(key, value []byte) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if len(value) > 0 {
		if _, err := s.GetExtKey(key); err != nil {
			return err
		}
		s.changes[crypto.Keccak256Hash(key)] = value
	}
	return nil
}

func (s *TrieExt) TryDelete(key []byte) error {
	return s.TryUpdate(key, nil)
}

// Commit write all changes to db
func (s *TrieExt) Commit(writer ethdb.KeyValueWriter) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if writer == nil {
		writer = s.diskdb
	}

	var err error
	for hashKey, extVal := range s.changes {
		dbKey := append(s.prefix, s.keyCache[hashKey]...)
		if len(extVal) > 0 {
			err = writer.Put(dbKey[:], extVal)
		} else {
			err = writer.Delete(dbKey[:])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func NewTrieExt(db ethdb.Database, tr state.Trie, prefix []byte) *TrieExt {
	return &TrieExt{
		diskdb:   db,
		trie:     tr,
		prefix:   prefix,
		keyCache: make(map[common.Hash][]byte),
		changes:  make(map[common.Hash][]byte),
	}
}
