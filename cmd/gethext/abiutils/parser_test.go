package abiutils

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestParseMethodIds(t *testing.T) {
	data, err := os.ReadFile("./tests/erc20.bin")
	if err != nil {
		panic(err)
	}
	bytecode := make([]byte, hex.DecodedLen(len(data)))
	if _, err := hex.Decode(bytecode, data); err != nil {
		panic(err)
	}

	methodIds := ParseMethodIds(bytecode)
	for _, id := range methodIds {
		fmt.Println(hexutil.Bytes(id[:]))
	}
}
