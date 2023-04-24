package abiutils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

// ABIParser parses all methods in contracts and detects which interfaces the contract was implemented
type ABIParser struct {
	db         ethdb.Database
	interfaces map[string]Interface
}

// ParseMethodIds parses the contract byte code to get all 4-bytes method ids
func (p *ABIParser) ParseMethodIds(bytecode []byte) []MethodId {
	// Single function calls will follow the following repeating pattern:
	// DUP1
	// PUSH4 <4-byte function signature>
	// EQ
	// PUSH2 <jumpdestination for the function>
	// JUMPI
	return nil
}

func (p *ABIParser) GetMethodById(methodId MethodId) []abi.Method {
	return nil
}

// GetInterfaces
func (p *ABIParser) GetInterfaces(methodIds []MethodId) []Interface {
	return nil
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
