package leth

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	rpcTimeout = time.Duration(10 * time.Second)
)

// OdrBackend is Off-chain Data Retrieval backend for LightEthereum
type OdrBackend interface {
	// Database returns the database used by the backend.
	Database() ethdb.Database

	// GetBlockByNumber retrieves a block by its block number.
	GetBlockByNumber(number uint64) (*types.Block, error)

	// GetBlockByHash retrieves a block by its block hash.
	GetBlockByHash(hash common.Hash) (*types.Block, error)

	// GetChainSegment retrieves a chain segment with a specified number of blocks starting from 'headBlock',
	// return the chain segment and highest block number of the remote chain
	GetChainSegment(fromBlock uint64, toBlock uint64) (types.Blocks, uint64, error)

	// GetCode retrieves the code stored at the given address.
	GetCode(addr common.Address) ([]byte, error)

	// GetTransactionByHash retrieves a transaction by its hash, also returns the block hash, transaction index in block
	GetTransactionByHash(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)

	// GetTransactionReceipt retrieves the receipt of a transaction identified by its hash.
	GetTransactionReceipt(hash common.Hash) (*types.Receipt, error)

	// GetBlockReceipts retrieves all receipts for transactions included in the given block.
	GetBlockReceipts(block *types.Block) (types.Receipts, error)

	// Close closes the backend connection and releases any associated resources.
	Close()
}

type RpcOdr struct {
	db     ethdb.Database
	client *rpc.Client
}

// rpcBlock represent the RPC result of a block.
type rpcBlock struct {
	*types.Header
	Hash         common.Hash
	Transactions []rpcTransaction
	UncleHashes  []common.Hash
}

func (b *rpcBlock) UnmarshalJSON(msg []byte) error {
	if err := json.Unmarshal(msg, &b.Header); err != nil {
		return err
	}
	type rpcBody struct {
		Hash         *common.Hash      `json:"hash"`
		Transactions *[]rpcTransaction `json:"transactions"`
		UncleHashes  *[]common.Hash    `json:"uncles"`
	}
	return json.Unmarshal(msg, &rpcBody{
		Hash:         &b.Hash,
		Transactions: &b.Transactions,
		UncleHashes:  &b.UncleHashes,
	})
}

// rpcTransaction extends the types.Transaction and represents the RPC result of a transaction in a block.
type rpcTransaction struct {
	*types.Transaction
	BlockNumber string
	BlockHash   common.Hash
	From        common.Address
}

func (tx *rpcTransaction) UnmarshalJSON(msg []byte) error {
	if err := json.Unmarshal(msg, &tx.Transaction); err != nil {
		return err
	}
	type txExtraInfo struct {
		BlockNumber *string         `json:"blockNumber,omitempty"`
		BlockHash   *common.Hash    `json:"blockHash,omitempty"`
		From        *common.Address `json:"from,omitempty"`
	}
	return json.Unmarshal(msg, &txExtraInfo{
		BlockNumber: &tx.BlockNumber,
		BlockHash:   &tx.BlockHash,
		From:        &tx.From,
	})
}

func (r *RpcOdr) Database() ethdb.Database {
	return r.db
}

func (r *RpcOdr) GetBlockByNumber(number uint64) (*types.Block, error) {
	hash := rawdb.ReadCanonicalHash(r.db, number)
	if (hash != common.Hash{}) {
		if block := rawdb.ReadBlock(r.db, hash, number); block != nil {
			return block, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return ethclient.NewClient(r.client).BlockByNumber(ctx, big.NewInt(int64(number)))
}

func (r *RpcOdr) GetBlockByHash(hash common.Hash) (*types.Block, error) {
	number := rawdb.ReadHeaderNumber(r.db, hash)
	if number != nil {
		if block := rawdb.ReadBlock(r.db, hash, *number); block != nil {
			return block, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return ethclient.NewClient(r.client).BlockByHash(ctx, hash)
}

func (r *RpcOdr) fetchRawBlocks(ctx context.Context, startNum uint64, fetchCount uint64) ([]*rpcBlock, error) {
	batch := make([]rpc.BatchElem, fetchCount)
	var idx uint64
	for idx = 0; idx < fetchCount; idx++ {
		batch[idx] = rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []interface{}{hexutil.EncodeUint64(startNum + idx), true},
			Result: new(rpcBlock),
		}
	}
	if err := r.client.BatchCallContext(ctx, batch); err != nil {
		return nil, err
	}

	ret := make([]*rpcBlock, len(batch))
	for idx, elem := range batch {
		if elem.Error != nil {
			return nil, elem.Error
		}
		ret[idx] = elem.Result.(*rpcBlock)
	}
	return ret, nil
}

func (r *RpcOdr) GetHeaders(hashes []common.Hash) ([]*types.Header, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	batch := make([]rpc.BatchElem, len(hashes))
	headers := make([]*types.Header, len(hashes))
	for idx := range hashes {
		batch[idx] = rpc.BatchElem{
			Method: "eth_getUncleByBlockHashAndIndex",
			Args:   []interface{}{hashes[idx], hexutil.EncodeUint64(uint64(idx))},
			Result: &headers[idx],
		}
	}

	if err := r.client.BatchCallContext(ctx, batch); err != nil {
		return nil, err
	}

	for idx, elem := range batch {
		if elem.Error != nil {
			return nil, elem.Error
		}
		if headers[idx] == nil {
			return nil, fmt.Errorf("got null header for hash %s", hashes[idx])
		}
	}
	return headers, nil
}

func (r *RpcOdr) GetChainSegment(startNum uint64, maxCount uint64) (types.Blocks, uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	remoteHeight, err := ethclient.NewClient(r.client).BlockNumber(ctx)
	if err != nil {
		return nil, 0, err
	}

	fetchCount := remoteHeight - startNum
	if fetchCount > maxCount {
		fetchCount = maxCount
	}

	rawBlocks, err := r.fetchRawBlocks(ctx, startNum, fetchCount)
	if err != nil {
		return nil, remoteHeight, err
	}

	ret := make(types.Blocks, len(rawBlocks))
	for idx, blk := range rawBlocks {
		// Load uncles because they are not included in the block response.
		var uncles []*types.Header
		if len(blk.UncleHashes) > 0 {
			uncles, err = r.GetHeaders(blk.UncleHashes)
			if err != nil {
				return nil, remoteHeight, fmt.Errorf("faield to fetch uncles of blocks %s. %v", blk.Hash, err)
			}
		}
		// Fill the sender cache of transactions in the block.
		txs := make([]*types.Transaction, len(blk.Transactions))
		for i, tx := range blk.Transactions {
			if (tx.From != common.Address{}) {
				setSenderFromServer(tx.Transaction, tx.From, blk.Hash)
			}
			txs[i] = tx.Transaction
		}

		ret[idx] = types.NewBlockWithHeader(blk.Header).WithBody(txs, uncles)
	}

	return ret, remoteHeight, nil
}

func (r *RpcOdr) GetCode(addr common.Address) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	var result hexutil.Bytes
	err := r.client.CallContext(ctx, &result, "eth_getCode", addr, "latest")
	return result, err
}

// TODO(khanghh): impelemt RpcOdr.GetTransactionByHash
func (r *RpcOdr) GetTransactionByHash(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return nil, common.Hash{}, 0, 0, nil
}

func (r *RpcOdr) GetTransactionReceipt(hash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	var receipt *types.Receipt
	if err := r.client.CallContext(ctx, &receipt, "eth_getTransactionReceipt", hash); err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, ethereum.NotFound
	}
	return receipt, nil
}

func (r *RpcOdr) GetBlockReceipts(block *types.Block) (types.Receipts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	batch := []rpc.BatchElem{}
	for _, tx := range block.Transactions() {
		batch = append(batch, rpc.BatchElem{
			Method: "eth_getTransactionReceipt",
			Args:   []interface{}{tx.Hash()},
			Result: new(types.Receipt),
		})
	}
	if err := r.client.BatchCallContext(ctx, batch); err != nil {
		return nil, err
	}
	ret := make(types.Receipts, len(batch))
	for idx, elem := range batch {
		if elem.Error != nil {
			return nil, elem.Error
		}
		ret[idx] = elem.Result.(*types.Receipt)
	}
	return ret, nil
}

func (r *RpcOdr) Close() {
	r.client.Close()
}

func NewRpcOdr(db ethdb.Database, rpcUrl string) (*RpcOdr, error) {
	client, err := rpc.DialHTTP(rpcUrl)
	if err != nil {
		return nil, err
	}
	return &RpcOdr{
		db:     db,
		client: client,
	}, nil
}
