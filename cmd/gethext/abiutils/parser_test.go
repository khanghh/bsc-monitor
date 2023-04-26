package abiutils

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"
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
