package abiutils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/status-im/keycard-go/hexutils"
)

type MethodId [4]byte

func (id MethodId) String() string {
	return hexutils.BytesToHex(id[:])
}

func (id MethodId) UnmarshalJSON(data []byte) error {
	val, err := hex.DecodeString(string(data))
	if err != nil {
		return err
	}
	copy(id[:], val)
	return nil
}

func HexToMethodId(s string) MethodId {
	id := MethodId{}
	copy(id[:], hexutils.HexToBytes(s))
	return id
}

type Interface struct {
	Name    string
	Methods map[string]abi.Method
	Events  map[string]abi.Event
	Errors  map[string]abi.Error
}

type FourByteMethod struct {
	Id      MethodId
	Methods []abi.Method
}

type abiEntryMarshaling struct {
	Type            string               `json:"type"`
	Name            string               `json:"name"`
	Inputs          []argumentMarshaling `json:"inputs,omitempty"`
	Outputs         []argumentMarshaling `json:"outputs,omitempty"`
	StateMutability string               `json:"stateMutability,omitempty"`
	Anonymous       bool                 `json:"anonymous,omitempty"`
}

type argumentMarshaling struct {
	Name         string               `json:"name"`
	Type         string               `json:"type"`
	InternalType string               `json:"internalType,omitempty"`
	Components   []argumentMarshaling `json:"components,omitempty"`
	Indexed      bool                 `json:"indexed,omitempty"`
}

type ABIEntry struct {
	Type    string
	Name    string
	Inputs  []abi.Argument
	Outputs []abi.Argument

	// Status indicator which can be: "pure", "view",
	// "nonpayable" or "payable".
	StateMutability string

	// Event relevant indicator represents the event is
	// declared as anonymous.
	Anonymous bool
}

func (e *ABIEntry) MarshalJSON() ([]byte, error) {
	marshaling := abiEntryMarshaling{
		Type:            e.Type,
		Name:            e.Name,
		StateMutability: e.StateMutability,
		Anonymous:       e.Anonymous,
	}
	for _, arg := range e.Inputs {
		marshaling.Inputs = append(marshaling.Inputs, argumentMarshaling{
			Name:         arg.Name,
			Type:         arg.Type.String(),
			InternalType: arg.Type.String(),
			Indexed:      arg.Indexed,
		})
	}
	for _, arg := range e.Outputs {
		marshaling.Outputs = append(marshaling.Outputs, argumentMarshaling{
			Name:         arg.Name,
			Type:         arg.Type.String(),
			InternalType: arg.Type.String(),
			Indexed:      arg.Indexed,
		})
	}
	return json.Marshal(marshaling)
}

func (e *ABIEntry) getSig() string {
	types := make([]string, len(e.Inputs))
	for i, arg := range e.Inputs {
		types[i] = arg.Type.String()
	}
	return fmt.Sprintf("%v(%v)", e.Name, strings.Join(types, ","))
}

func (e *ABIEntry) getID() []byte {
	return crypto.Keccak256([]byte(e.getSig()))[:4]
}

// Contract holds information about a contract such as name, implemented interfaces,
// methods owned by the contract itself.
type Contract struct {
	abi.ABI
	Name       string                // Name of the contract
	Implements map[string]Interface  // Known interfaces that the contract implemented
	OwnMethods map[string]abi.Method // Methods owned by contract itself only, not included in any interfaces
}

func (c *Contract) Interface(name string) (Interface, bool) {
	impl, ok := c.Implements[name]
	return impl, ok
}

func (c *Contract) MethodById(id MethodId) (*abi.Method, error) {
	return nil, nil
}

func (c *Contract) Pack(name string, v ...interface{}) {
}

func (c *Contract) Unpack(name string, data []byte) ([]interface{}, error) {
	return nil, nil
}

func NewContract(name string, abi *abi.ABI) *Contract {
	return &Contract{
		Name: name,
	}
}