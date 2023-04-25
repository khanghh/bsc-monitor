package abiutils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/asm"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var methodSigRegex = regexp.MustCompile(`(\w+)\(([^\(\)]*)\)(?:\s*returns\s*\(([^\(\)]*)\))?$`)

func parseArguments(str string) (abi.Arguments, error) {
	args := make(abi.Arguments, 0)
	if len(str) == 0 {
		return args, nil
	}
	argArr := strings.Split(str, ",")
	for _, arg := range argArr {
		tokens := strings.Fields(arg)
		var name string
		typeStr := tokens[len(tokens)-1] // get the last token as type
		if len(tokens) == 2 {
			name = tokens[0]
		}
		argType, err := abi.NewType(typeStr, typeStr, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
		args = append(args, abi.Argument{
			Name:    name,
			Type:    argType,
			Indexed: false,
		})
	}
	return args, nil
}

// ParseMethodSig parses method identifier string into abi.Method
func ParseMethodSig(str string) (ABIEntry, error) {
	matches := methodSigRegex.FindStringSubmatch(str)
	if matches == nil || len(matches) < 2 {
		return ABIEntry{}, fmt.Errorf("invalid method signature")
	}
	name := matches[1]
	var inputs, outputs abi.Arguments
	var err error
	if len(matches) > 2 {
		if inputs, err = parseArguments(matches[2]); err != nil {
			return ABIEntry{}, err
		}
	}
	if len(matches) == 4 {
		if outputs, err = parseArguments(matches[2]); err != nil {
			return ABIEntry{}, err
		}
	}
	return ABIEntry{
		Type:    "function",
		Name:    name,
		Inputs:  inputs,
		Outputs: outputs,
	}, nil
}

// ParseMethodIds parses the contract byte code to get all 4-bytes method ids
// Single function calls will follow the following repeating pattern:
// DUP1
// PUSH4 <4-byte function signature>
// EQ
// PUSH2 <jumpdestination for the function>
// JUMPI
func ParseMethodIds(bytecode []byte) []MethodId {
	ret := []MethodId{}
	pattern := []vm.OpCode{vm.DUP1, vm.PUSH4, 0x00, vm.PUSH2, vm.JUMPI}
	matchPattern := func(ins []vm.OpCode) bool {
		if len(ins) < len(pattern) {
			return false
		}
		segment := ins[len(ins)-len(pattern):]
		for i, op := range pattern {
			if op != 0x00 && op != segment[i] {
				return false
			}
		}
		return true
	}
	push4Args := [4]byte{}
	inJumpTable := false
	instructions := []vm.OpCode{}
	it := asm.NewInstructionIterator(bytecode)
	for it.Next() {
		instructions = append(instructions, it.Op())
		if it.Op() == vm.CALLDATALOAD && !inJumpTable {
			inJumpTable = true
		}
		if inJumpTable {
			if it.Op() == vm.PUSH4 {
				copy(push4Args[:], it.Arg())
			}
			if it.Op() == vm.JUMPI && matchPattern(instructions) {
				exited := false
				for _, id := range ret {
					if id == push4Args {
						exited = true
						break
					}
				}
				if !exited {
					ret = append(ret, push4Args)
				}
				instructions = []vm.OpCode{}
			}
			if it.Op() == vm.REVERT {
				break
			}
		}
	}
	return ret
}

// ABIParser parses all methods in contracts and detects which interfaces the contract was implemented
type ABIParser struct {
	db         ethdb.Database
	interfaces map[string]Interface
}

func (p *ABIParser) isImplemented(intf Interface, sigs []MethodId) bool {
	methodMap := make(map[string]bool)
	for _, method := range intf.Methods {
		methodMap[string(method.ID)] = true
	}

	for _, id := range sigs {
		if !methodMap[string(id[:])] {
			return false
		}
	}
	return true
}

// GetInterfaces get all implemented interfaces of the given list of method ids
func (p *ABIParser) GetInterfaces(ids []MethodId) []Interface {
	ret := make([]Interface, 0)
	for _, intf := range p.interfaces {
		if p.isImplemented(intf, ids) {
			ret = append(ret, intf)
		}
	}
	return ret
}

func (p *ABIParser) ParseContract(bytecode []byte) (*Contract, error) {
	return nil, nil
}

func loadInterfaces(db ethdb.Database) map[string]Interface {
	interfaces := make(map[string]Interface)
	rawIfs := readInterfaceABIs(db)
	for _, rawIf := range rawIfs {
		item, err := NewInterface(rawIf.Name, rawIf.ABI)
		if err != nil {
			log.Error("Invalid contract interface", "name", rawIf.Name, "error", err)
			continue
		}
		interfaces[item.Name] = item
	}
	log.Info(fmt.Sprintf("Loaded %d contract interfaces", len(interfaces)))
	return interfaces
}

func NewParser(db ethdb.Database) (*ABIParser, error) {
	return &ABIParser{
		db:         db,
		interfaces: loadInterfaces(db),
	}, nil
}
