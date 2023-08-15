package abiutils

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/status-im/keycard-go/hexutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMethodIds(t *testing.T) {
	data, err := os.ReadFile("./tests/example.bin")
	if err != nil {
		panic(err)
	}
	bytecode := make([]byte, hex.DecodedLen(len(data)))
	if _, err := hex.Decode(bytecode, data); err != nil {
		panic(err)
	}

	methodIds := ParseMethodIds(bytecode)
	for _, id := range methodIds {
		fmt.Println(id)
	}
}

func TestABIParserGetInterfaces(t *testing.T) {
	unknowns := []string{
		"aabb1337", // test unknown methods
		"abcdefaa", // test unknown methods
	}
	testERC20Sigs := []string{
		"dd62ed3e", // allowance(address,address)
		"095ea7b3", // approve(address,uint256)
		"70a08231", // balanceOf(address)
		"18160ddd", // totalSupply()
		"a9059cbb", // transfer(address,uint256)
		"23b872dd", // transferFrom(address,address,uint256)
		"aabb1337", // test unknown methods
		"abcdefaa", // test unknown methods
	}
	testERC721Sigs := []string{
		"095ea7b3", // approve(address,uint256)
		"70a08231", // balanceOf(address)
		"081812fc", // getApproved(uint256)
		"e985e9c5", // isApprovedForAll(address,address)
		"6352211e", // ownerOf(uint256)
		"42842e0e", // safeTransferFrom(address,address,uint256)
		"b88d4fde", // safeTransferFrom(address,address,uint256,bytes)
		"a22cb465", // setApprovalForAll(address,bool)
		"23b872dd", // transferFrom(address,address,uint256)
		"aabb1337", // test unknown methods
		"abcdefaa", // test unknown methods
	}
	getNameList := func(ifs []Interface) []string {
		ret := make([]string, 0, len(ifs))
		for _, item := range ifs {
			ret = append(ret, item.Name)
		}
		return ret
	}
	parser := NewParser(rawdb.NewMemoryDatabase())
	ifs, remaining := parser.ParseInterfaces(testERC20Sigs)
	assert.Contains(t, getNameList(ifs), "IERC20")
	assert.Equal(t, remaining, unknowns)
	ifs, remaining = parser.ParseInterfaces(testERC721Sigs)
	assert.Contains(t, getNameList(ifs), "IERC721")
	assert.Equal(t, remaining, unknowns)
}

func TestUnpackInput(t *testing.T) {
	erc20 := defaultInterfaces["IERC20"]
	type TransferArgs struct {
		From   common.Address
		To     common.Address
		Amount *big.Int
	}

	tests := []struct {
		input        []byte
		method       string
		expectedArgs TransferArgs
	}{
		{
			input:  hexutils.HexToBytes("000000000000000000000000a73bc58956dc002ab777452aa0b60d37b4f6d6370000000000000000000000000000000000000000000000000de0b6b3a7640000"),
			method: "transfer",
			expectedArgs: TransferArgs{
				To:     common.HexToAddress("0xA73BC58956dC002Ab777452aa0b60d37B4f6d637"),
				Amount: big.NewInt(1000000000000000000),
			},
		},
		{
			input:  hexutils.HexToBytes("00000000000000000000000064108bbde14cc327ebba159e1937a9791ce0e8a9000000000000000000000000c58bb74606b73c5043b75d7aa25ebe1d5d4e7c720000000000000000000000000000000000000000000000000000000062de20d3"),
			method: "transferFrom",
			expectedArgs: TransferArgs{
				From:   common.HexToAddress("0x64108bbDe14CC327EBba159e1937A9791Ce0e8a9"),
				To:     common.HexToAddress("0xc58Bb74606b73c5043B75d7Aa25ebe1D5D4E7c72"),
				Amount: big.NewInt(1658724563),
			},
		},
	}

	for _, test := range tests {
		var args TransferArgs
		err := erc20.UnpackInput(&args, test.method, test.input)
		require.NoError(t, err)
		assert.Equal(t, test.expectedArgs, args)
	}
}
