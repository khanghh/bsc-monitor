package abiutils

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/ethdb"
)

// ParseMethodSig parses method identifier string into abi.Method
func ParseMethodSig(str string) (*abi.Method, error) {
	return nil, nil
}

// ContractParser parses all methods in contracts and detects which interfaces the contract was implemented
type ContractParser struct {
	db         ethdb.Database
	interfaces map[string]Interface // known contract interfaces
}

// ParseMethodIds parses the contract byte code to get all 4-bytes method ids
func (p *ContractParser) ParseMethodIds(bytecode []byte) []MethodId {
	// Single function calls will follow the following repeating pattern:
	// DUP1
	// PUSH4 <4-byte function signature>
	// EQ
	// PUSH2 <jumpdestination for the function>
	// JUMPI
	return nil
}

func (p *ContractParser) GetMethods(methodIds []MethodId) map[MethodId]abi.Method {
	return nil
}

// ParseInterfaces
func (p *ContractParser) ParseInterfaces(methodIds []MethodId) []Interface {
	return nil
}

func NewContractParser(db ethdb.Database) (*ContractParser, error) {
	return &ContractParser{
		db: db,
	}, nil
}
