package abiutils

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func newType(t, i string, components []abi.ArgumentMarshaling) abi.Type {
	abiType, _ := abi.NewType(t, i, components)
	return abiType
}

func TestABIEntryMarshalJSON(t *testing.T) {
	abiEntryStr := `{
		"inputs": [
			{
				"internalType": "address",
				"name": "from",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "value",
				"type": "uint256"
			}
		],
		"name": "transferFrom",
		"outputs": [
			{
				"internalType": "bool",
				"name": "",
				"type": "bool"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	}`
	entry := &ABIEntry{}
	json.Unmarshal([]byte(abiEntryStr), entry)
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}
