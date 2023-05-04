package abiutils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/asm"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

var methodSigRegex = regexp.MustCompile(`(\w+)\(([^\(\)]*)\)(?:\s*returns\s*\(([^\(\)]*)\))?$`)

const fourbytesCacheSize = 1024

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
func ParseMethodSig(str string) (ABIElement, error) {
	matches := methodSigRegex.FindStringSubmatch(str)
	if matches == nil || len(matches) < 2 {
		return ABIElement{}, fmt.Errorf("invalid method signature")
	}
	name := matches[1]
	var inputs, outputs abi.Arguments
	var err error
	if len(matches) > 2 {
		if inputs, err = parseArguments(matches[2]); err != nil {
			return ABIElement{}, err
		}
	}
	if len(matches) == 4 {
		if outputs, err = parseArguments(matches[3]); err != nil {
			return ABIElement{}, err
		}
	}
	return ABIElement{
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
func ParseMethodIds(bytecode []byte) []string {
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

	methodIds := map[string]bool{}
	push4Args := [4]byte{}
	inJumpTable := false
	instructions := make([]vm.OpCode, 0, 64)

	it := asm.NewInstructionIterator(bytecode)
	for it.Next() {
		op := it.Op()
		instructions = append(instructions, it.Op())
		if op == vm.CALLDATALOAD && !inJumpTable {
			inJumpTable = true
		}
		if inJumpTable {
			switch op {
			case vm.PUSH4:
				copy(push4Args[:], it.Arg())
			case vm.JUMPI:
				if matchPattern(instructions) {
					methodIds[common.Bytes2Hex(push4Args[:])] = true
					instructions = instructions[:0]
				}
			case vm.REVERT:
				break
			}
		}
	}
	ret := make([]string, 0, len(methodIds))
	for key := range methodIds {
		ret = append(ret, key)
	}
	return ret
}

// ABIParser parses all methods in contracts and detects which interfaces the contract was implemented
type ABIParser struct {
	db             ethdb.Database
	interfaces     map[string]Interface
	fourbytesCache *lru.Cache
}

func (p *ABIParser) LookupFourBytes(id string) []ABIElement {
	if cached, ok := p.fourbytesCache.Get(id); ok {
		return cached.([]ABIElement)
	}
	ret := []ABIElement{}
	data := extdb.ReadFourBytesABIs(p.db, common.FromHex(id))
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &ret); err != nil {
		log.Debug("Look up 4-bytes error", "id", hexutil.Bytes(id[:]), "error", err)
	}
	// TODO(khanghh): remove duplicate items
	for _, intf := range p.interfaces {
		if elem, exist := intf.Elements[id]; exist {
			ret = append(ret, elem)
		}
	}
	p.fourbytesCache.Add(id, ret)
	return ret
}

func (p *ABIParser) isImplemented(intf Interface, sigs []string) bool {
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
// returns list of matched interfaces and list of unkown method ids
func (p *ABIParser) GetInterfaces(ids []string) ([]Interface, []string) {
	implements := []Interface{}
	ifMethods := map[string]bool{}
	for _, intf := range p.interfaces {
		if !p.isImplemented(intf, ids) {
			continue
		}
		implements = append(implements, intf)
		for _, method := range intf.Methods {
			ifMethods[common.Bytes2Hex(method.ID)] = true
		}
	}
	unknownMethods := []string{}
	for _, item := range ids {
		if !ifMethods[item] {
			unknownMethods = append(unknownMethods, item)
		}
	}
	return implements, unknownMethods
}

func (p *ABIParser) ParseContract(bytecode []byte) (*Contract, error) {
	ids := ParseMethodIds(bytecode)
	if len(ids) == 0 {
		return nil, fmt.Errorf("empty contract")
	}
	ifs, methodIds := p.GetInterfaces(ids)
	entries := []ABIElement{}
	for _, id := range methodIds {
		items := p.LookupFourBytes(id)
		if len(items) > 0 {
			entries = append(entries, items...)
		} else {
			entries = append(entries, ABIElement{Name: id})
		}
	}
	return NewContract("", entries, ifs)
}

func loadInterfaces(db ethdb.Database) map[string]Interface {
	interfaces := make(map[string]Interface)
	for name, intf := range defaultInterfaces {
		interfaces[name] = intf
	}
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

func NewParser(db ethdb.Database) *ABIParser {
	abiCache, _ := lru.New(fourbytesCacheSize)
	return &ABIParser{
		db:             db,
		interfaces:     loadInterfaces(db),
		fourbytesCache: abiCache,
	}
}
