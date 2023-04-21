package abiutils

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestAbiListUnmarshalJSON(t *testing.T) {
	testData := `{
		"18160ddd": "totalSupply() returns(uint256)",
		"a9059cbb": "transfer(address,uint256)",
		"23b872dd": [
		  "transferFrom(address,address,uint256)",
		  {
			"constant": false,
			"inputs": [
			  {
				"name": "_from",
				"type": "address"
			  },
			  {
				"name": "_to",
				"type": "address"
			  },
			  {
				"name": "_value",
				"type": "uint256"
			  }
			],
			"name": "transferFrom",
			"outputs": [
			  {
				"name": "",
				"type": "bool"
			  }
			],
			"payable": false,
			"stateMutability": "nonpayable",
			"type": "function"
		  }
		]
	  }`

	methodSigs := make(map[string]abiList)
	if err := json.Unmarshal([]byte(testData), &methodSigs); err != nil {
		panic(err)
	}
	for _, val := range methodSigs {
		fmt.Println(val)
	}
}
