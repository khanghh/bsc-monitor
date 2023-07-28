package leth

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	rpcTimeout = time.Duration(5 * time.Second)
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

	// GetCodeAt retrieves the code stored at 'addr'.
	GetCodeAt(addr common.Address) ([]byte, error)

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

func (r *RpcOdr) Database() ethdb.Database {
	return r.db
}

func (r *RpcOdr) GetBlockByNumber(number uint64) (*types.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return ethclient.NewClient(r.client).BlockByNumber(ctx, big.NewInt(int64(number)))
}

func (r *RpcOdr) GetBlockByHash(hash common.Hash) (*types.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return ethclient.NewClient(r.client).BlockByHash(ctx, hash)
}

func (r *RpcOdr) GetChainSegment(startNum uint64, maxCount uint64) (types.Blocks, uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	remoteHeight, err := ethclient.NewClient(r.client).BlockNumber(ctx)
	if err != nil {
		return nil, 0, err
	}

	fetchCount := maxCount
	if remoteHeight-startNum < maxCount {
		fetchCount = remoteHeight - startNum
	}

	batch := []rpc.BatchElem{}
	var idx uint64
	for idx = 1; idx < fetchCount; idx++ {
		blockNum := int64(startNum + idx)
		batch = append(batch, rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []interface{}{big.NewInt(blockNum), true},
			Result: new(types.Block),
		})
	}
	if err := r.client.BatchCallContext(ctx, batch); err != nil {
		return nil, 0, err
	}

	ret := types.Blocks{}
	for _, elem := range batch {
		if elem.Error != nil {
			break
		}
		ret = append(ret, elem.Result.(*types.Block))
	}
	return ret, remoteHeight, nil
}

func (r *RpcOdr) GetCodeAt(addr common.Address) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return ethclient.NewClient(r.client).CodeAt(ctx, addr, nil)
}

func (r *RpcOdr) GetTransactionByHash(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return nil, common.Hash{}, 0, 0, nil
}

func (r *RpcOdr) GetTransactionReceipt(hash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return ethclient.NewClient(r.client).TransactionReceipt(ctx, hash)
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
