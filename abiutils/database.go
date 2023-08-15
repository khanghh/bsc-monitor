package abiutils

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/extdb"
	"github.com/ethereum/go-ethereum/log"
)

type ABIElements []ABIElement

func (list *ABIElements) addUnique(item ABIElement) bool {
	for _, entry := range *list {
		if item.Identifier() == entry.Identifier() {
			return false
		}
	}
	*list = append(*list, item)
	return true
}

func (list *ABIElements) UnmarshalJSON(data []byte) error {
	if text, err := strconv.Unquote(string(data)); err == nil {
		if entry, err := ParseMethodSig(text); err == nil {
			*list = append(*list, entry)
		}
		return nil
	}

	rawEntries := []json.RawMessage{}
	if err := json.Unmarshal(data, &rawEntries); err != nil {
		return err
	}
	for _, raw := range rawEntries {
		var (
			entry ABIElement
			err   error
		)
		if text, qerr := strconv.Unquote(string(raw)); qerr == nil {
			entry, err = ParseMethodSig(text)
		} else {
			err = json.Unmarshal(raw, &entry)
		}
		if err != nil {
			return err
		}
		*list = append(*list, entry)
	}
	return nil
}

func UnmarshalABI(data []byte) ([]ABIElement, error) {
	list := ABIElements{}
	err := json.Unmarshal(data, &list)
	return list, err
}

func readFourBytesABIs(db ethdb.Database, fourbytes []byte) ABIElements {
	ret := ABIElements{}
	data := extdb.ReadFourBytesABIs(db, fourbytes)
	json.Unmarshal(data, &ret)
	return ret
}

func import4BytesABIs(db ethdb.Database, abis map[string]ABIElements, override bool) (int, error) {
	if len(abis) == 0 {
		return 0, nil
	}
	imported := 0
	batch := db.NewBatch()
	for id, list := range abis {
		fourbytes, err := hex.DecodeString(id)
		if err != nil {
			continue
		}
		elems := list
		if !override {
			elems = readFourBytesABIs(db, fourbytes)
			modified := false
			for _, entry := range list {
				modified = elems.addUnique(entry) || modified
			}
			if !modified {
				elems = nil
			}
		}
		if len(elems) > 0 {
			data, err := json.Marshal(elems)
			if err != nil {
				return 0, err
			}
			extdb.WriteFourBytesABIs(db, fourbytes, data)
			imported += len(elems)
		}
	}
	return imported, batch.Write()
}

// rawInterface is data struct hold information about an contract interface to be stored in extdb
type rawInterface struct {
	Name string       `json:"name"` // Name of interface
	ABI  []ABIElement `json:"abi"`  // List of signatures fo methods, events, errors
}

func readInterfaceABIs(db ethdb.Database) []rawInterface {
	it := db.NewIterator(extdb.InterfaceABIPrefix, nil)
	ret := make([]rawInterface, 0)
	for it.Next() {
		if bytes.HasSuffix(it.Key(), extdb.InterfaceABISuffix) {
			raw := rawInterface{}
			if err := json.Unmarshal(it.Value(), &raw); err != nil {
				log.Error("could not load interface abi", "key", hexutil.Encode(it.Key()))
				continue
			}
			ret = append(ret, raw)
		}
	}
	return ret
}

func importInterfaces(db ethdb.Database, ifs map[string]ABIElements, override bool) (int, int, error) {
	batch := db.NewBatch()
	importList := []rawInterface{}
	for name, item := range ifs {
		raw := rawInterface{name, item}
		if override {
			importList = append(importList, raw)
		} else if exits, _ := db.Has(extdb.InterfaceABIKey(name)); !exits {
			importList = append(importList, raw)
		}
	}
	numEntries := 0
	for _, item := range importList {
		data, _ := json.Marshal(item)
		extdb.WriteInterfaceABI(batch, item.Name, data)
		numEntries += len(item.ABI)
	}
	return len(importList), numEntries, batch.Write()
}

func ImportABIsData(db ethdb.Database, reader io.Reader, override bool) error {
	dec := json.NewDecoder(reader)
	var data struct {
		FourBytes  map[string]ABIElements `json:"4bytes"`     // 4-bytes sigs to abi list
		Interfaces map[string]ABIElements `json:"interfaces"` // interface name to abi list
	}
	if err := dec.Decode(&data); err != nil {
		return err
	}

	abiCount, err := import4BytesABIs(db, data.FourBytes, override)
	if err != nil {
		log.Error("Could not import 4-bytes ABI entries", "error", err)
		return err
	}
	log.Info(fmt.Sprintf("Imported %d 4-bytes ABI entries", abiCount))

	ifCount, abiCount, err := importInterfaces(db, data.Interfaces, override)
	if err != nil {
		log.Error("Could not import contract interfaces", "error", err)
		return err
	}
	log.Info(fmt.Sprintf("Imported %d contract interfaces, total ABI entries: %d", abiCount, ifCount))
	return nil
}
