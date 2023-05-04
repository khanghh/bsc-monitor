package abiutils

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/stretchr/testify/assert"
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
	ifs, remaining := parser.GetInterfaces(testERC20Sigs)
	assert.Contains(t, getNameList(ifs), "IERC20")
	assert.Equal(t, remaining, unknowns)
	ifs, remaining = parser.GetInterfaces(testERC721Sigs)
	assert.Contains(t, getNameList(ifs), "IERC721")
	assert.Equal(t, remaining, unknowns)
}
