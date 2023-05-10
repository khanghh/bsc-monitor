package abiutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// FourBytesSigOf calculate 4-bytes signature from given text
func FourBytesSigOf(sig string) string {
	return common.Bytes2Hex(crypto.Keccak256([]byte(sig))[:4])
}

// privateABI is an alias for the abi.ABI type and
// is used to prevent modification of the embedded abi.ABI field in a struct
type privateABI = abi.ABI

type Interface struct {
	privateABI                       // embedded abi struct
	Name       string                // interface name
	Elements   map[string]ABIElement // map from 4-bytes to abi element
}

func (intf *Interface) UnpackInput(v interface{}, name string, data []byte) error {
	var args abi.Arguments
	if method, exist := intf.Methods[name]; exist {
		if len(data)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output: %s - Bytes: [%+v]", string(data), data)
		}
		args = method.Inputs
	}
	if len(args) > 0 {
		unpacked, err := args.Unpack(data)
		if err != nil {
			return err
		}
		return args.Copy(v, unpacked)
	}
	return nil
}

func NewInterface(name string, elems []ABIElement) (Interface, error) {
	methods := make(map[string]abi.Method)
	events := make(map[string]abi.Event)
	elements := make(map[string]ABIElement)
	for _, item := range elems {
		elements[FourBytesSigOf(item.Identifier())] = item
		switch item.Type {
		case "function":
			name := abi.ResolveNameConflict(item.Name, func(s string) bool { _, ok := methods[s]; return ok })
			methods[name] = abi.NewMethod(name, item.Name, abi.Function, item.StateMutability, false, false, item.Inputs, item.Outputs)
		case "event":
			name := abi.ResolveNameConflict(item.Name, func(s string) bool { _, ok := events[s]; return ok })
			events[name] = abi.NewEvent(name, item.Name, item.Anonymous, item.Inputs)
		default:
			return Interface{}, fmt.Errorf("invalid abi entry type: %v", item.Type)
		}
	}
	return Interface{
		privateABI: abi.ABI{Methods: methods, Events: events},
		Name:       name,
		Elements:   elements,
	}, nil
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

type ABIElement struct {
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

func (e *ABIElement) MarshalJSON() ([]byte, error) {
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

func (e *ABIElement) Identifier() string {
	types := make([]string, len(e.Inputs))
	for i, arg := range e.Inputs {
		types[i] = arg.Type.String()
	}
	return fmt.Sprintf("%v(%v)", e.Name, strings.Join(types, ","))
}

// Contract holds information about a contract such as name, implemented interfaces,
// methods owned by the contract itself.
type Contract struct {
	privateABI
	Name       string                 // Name of the contract
	Implements map[string]Interface   // Known interfaces that the contract implemented
	OwnMethods map[string]abi.Method  // Methods owned by contract itself only, not included in any interfaces
	Unknown    map[string]interface{} // Unknown ABI elements
}

func (c *Contract) Interface(name string) *Interface {
	inft, ok := c.Implements[name]
	if ok {
		return &inft
	}
	return nil
}

func NewContract(name string, elems []ABIElement, ifs []Interface) (*Contract, error) {
	unknown := make(map[string]interface{})
	ownMethods := make(map[string]abi.Method)
	contractABI := abi.ABI{
		Methods: make(map[string]abi.Method),
		Events:  make(map[string]abi.Event),
		Errors:  make(map[string]abi.Error),
	}
	for _, field := range elems {
		switch field.Type {
		case "constructor":
			contractABI.Constructor = abi.NewMethod("", "", abi.Constructor, field.StateMutability, false, false, field.Inputs, nil)
		case "function":
			name := abi.ResolveNameConflict(field.Name, func(s string) bool { _, ok := contractABI.Methods[s]; return ok })
			method := abi.NewMethod(name, field.Name, abi.Function, field.StateMutability, false, false, field.Inputs, field.Outputs)
			contractABI.Methods[name] = method
			ownMethods[name] = method
		case "fallback":
			// New introduced function type in v0.6.0, check more detail
			// here https://solidity.readthedocs.io/en/v0.6.0/contracts.html#fallback-function
			if contractABI.HasFallback() {
				return nil, errors.New("only single fallback is allowed")
			}
			contractABI.Fallback = abi.NewMethod("", "", abi.Fallback, field.StateMutability, false, false, nil, nil)
		case "receive":
			// New introduced function type in v0.6.0, check more detail
			// here https://solidity.readthedocs.io/en/v0.6.0/contracts.html#fallback-function
			if contractABI.HasReceive() {
				return nil, errors.New("only single receive is allowed")
			}
			if field.StateMutability != "payable" {
				return nil, errors.New("the statemutability of receive can only be payable")
			}
			contractABI.Receive = abi.NewMethod("", "", abi.Receive, field.StateMutability, false, false, nil, nil)
		case "event":
			name := abi.ResolveNameConflict(field.Name, func(s string) bool { _, ok := contractABI.Events[s]; return ok })
			contractABI.Events[name] = abi.NewEvent(name, field.Name, field.Anonymous, field.Inputs)
		case "error":
			// Errors cannot be overloaded or overridden but are inherited,
			// no need to resolve the name conflict here.
			contractABI.Errors[field.Name] = abi.NewError(field.Name, field.Inputs)
		default:
			unknown[field.Name] = field
		}
	}
	impls := make(map[string]Interface)
	for _, item := range ifs {
		impls[item.Name] = item
		for _, method := range item.Methods {
			contractABI.Methods[method.Name] = method
		}
	}
	return &Contract{
		privateABI: contractABI,
		Implements: impls,
		Unknown:    unknown,
	}, nil
}
