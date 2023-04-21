package abiutils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/exp/maps"
)

type abiList []ABIEntry

func (list *abiList) addUnique(item ABIEntry) bool {
	for _, entry := range *list {
		if item.getSig() == entry.getSig() {
			return false
		}
	}
	*list = append(*list, item)
	return true
}

func (list *abiList) UnmarshalJSON(data []byte) error {
	var (
		entry ABIEntry
		err   error
	)

	if text, err := strconv.Unquote(string(data)); err == nil {
		if entry, err = ParseMethodSig(text); err == nil {
			*list = append(*list, entry)
		}
		return nil
	}

	rawEntries := []json.RawMessage{}
	if err := json.Unmarshal(data, &rawEntries); err != nil {
		return err
	}
	for _, raw := range rawEntries {
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

// addEntries add a list of ABIEntry into 4-bytes sigs map without duplicate
func addEntries(abis map[string]abiList, list abiList) error {
	for _, entry := range list {
		id := hex.EncodeToString(entry.getID())
		if entries, ok := abis[id]; ok {
			entries.addUnique(entry)
			continue
		}
		abis[id] = abiList{entry}
	}
	return nil
}

func readFourBytesABIs(db ethdb.Database, fourbytes []byte) abiList {
	ret := abiList{}
	data := extdb.ReadFourBytesABIs(db, fourbytes)
	json.Unmarshal(data, &ret)
	return ret
}

func import4BytesABIs(db ethdb.Database, abis map[string]abiList, override bool) (int, error) {
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
		entries := list
		if !override {
			entries = readFourBytesABIs(db, fourbytes)
			modified := false
			for _, entry := range list {
				modified = entries.addUnique(entry) || modified
			}
			if !modified {
				entries = nil
			}
		}
		if len(entries) > 0 {
			data, err := json.Marshal(entries)
			if err != nil {
				return 0, err
			}
			extdb.WriteFourBytesABIs(db, fourbytes, data)
			imported += len(entries)
		}
	}
	return imported, batch.Write()
}

// rawInterface is data struct hold information about an contract interface to be stored in extdb
type rawInterface struct {
	Name    string   // Name of interface
	Methods []string // List of method signatures
	Events  []string // List of event signatures
	Errors  []string // List of error signatures
}

func abisToIterfaces(ifabis map[string]abiList) []rawInterface {
	ifs := make([]rawInterface, 0)
	for name, entries := range ifabis {
		methods := make([]string, 0)
		events := make([]string, 0)
		errors := make([]string, 0)
		for _, entry := range entries {
			switch entry.Type {
			case "function":
				methods = append(methods, entry.getSig())
			case "event":
				events = append(events, entry.getSig())
			case "error":
				errors = append(errors, entry.getSig())
			}
		}
		ifs = append(ifs, rawInterface{name, methods, events, errors})
	}
	return ifs
}

func readInterfaceList(db ethdb.Database) map[string]rawInterface {
	ret := make(map[string]rawInterface)
	entries := make([]rawInterface, 0)
	enc := extdb.ReadInterfaceList(db)
	rlp.DecodeBytes(enc, &entries)
	for _, item := range entries {
		ret[item.Name] = item
	}
	return ret
}

func importInterfaces(db ethdb.Database, ifs []rawInterface, override bool) (int, error) {
	if !override {
		allIfs := readInterfaceList(db)
		for _, item := range ifs {
			allIfs[item.Name] = item
		}
		ifs = maps.Values(allIfs)
	}
	enc, _ := rlp.EncodeToBytes(ifs)
	extdb.WriteInterfaceList(db, enc)
	return len(ifs), nil
}

func ImportABIsData(db ethdb.Database, reader io.Reader, override bool) error {
	dec := json.NewDecoder(reader)
	var data struct {
		FourBytes  map[string]abiList `json:"4bytes"`     // 4-bytes sigs to abi list
		Interfaces map[string]abiList `json:"interfaces"` // interface name to abi list
	}
	if err := dec.Decode(&data); err != nil {
		return err
	}

	fourbytesABIs, ifABIs := data.FourBytes, data.Interfaces
	ifs := abisToIterfaces(ifABIs)
	for _, list := range ifABIs {
		addEntries(fourbytesABIs, list)
	}

	abiCount, err := import4BytesABIs(db, fourbytesABIs, override)
	if err != nil {
		log.Error("Could not import 4-bytes ABI entries", "error", err)
		return err
	}
	ifCount, err := importInterfaces(db, ifs, override)
	if err != nil {
		log.Error("Could not import contract interfaces", "error", err)
		return err
	}
	log.Info(fmt.Sprintf("Imported %d ABI entries and %d interfaces", abiCount, ifCount))
	return nil
}
