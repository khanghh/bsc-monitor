package abiutils

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type MethodId [4]byte

// FourBytesMethod
type FourBytesMethod struct {
	Id      MethodId // 4-bytes signature
	Methods []string
}

// Interface
type Interface struct {
	Name    string
	Methods map[string]abi.Method
}

func NewInterface(methods []abi.Method) *Interface {
	return &Interface{}
}

// Contract holds information about a contract such as name, implemented interfaces,
// methods owned by the contract itself.
type Contract struct {
	Name       string                // Name of the contract
	Implements []*Interface          // Known interfaces that the contract implemented
	OwnMethods map[string]abi.Method // Methods owned by contract itself only, not included in any interfaces
}
