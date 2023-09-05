package extdb

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/olekukonko/tablewriter"
)

type counter uint64

func (c counter) String() string {
	return fmt.Sprintf("%d", c)
}

func (c counter) Percentage(current uint64) string {
	return fmt.Sprintf("%d", current*100/uint64(c))
}

// stat stores sizes and count for a parameter
type stat struct {
	size  common.StorageSize
	count counter
}

// Add size to the stat and increase the counter by 1
func (s *stat) Add(size common.StorageSize) {
	s.size += size
	s.count++
}

func (s *stat) Size() string {
	return s.size.String()
}

func (s *stat) Count() string {
	return s.count.String()
}

// InspectDatabase traverses the entire database and checks the size of all different categories of data.
func InspectDatabase(db ethdb.Database, keyPrefix, keyStart []byte) error {
	it := db.NewIterator(keyPrefix, keyStart)
	defer it.Release()

	var (
		count  int64
		start  = time.Now()
		logged = time.Now()

		// Key-value store statistics
		accounts      stat
		contracts     stat
		interfaceABIs stat
		indexStates   stat
		indexRecords  stat
		fourBytes     stat

		// Meta- and unaccounted data
		metadata    stat
		unaccounted stat

		// Totals
		total common.StorageSize
	)
	// Inspect key-value database first.
	for it.Next() {
		var (
			key  = it.Key()
			size = common.StorageSize(len(key) + len(it.Value()))
		)
		total += size
		switch {
		case bytes.HasPrefix(key, AccountInfoPrefix) && len(key) == (len(AccountInfoPrefix)+common.AddressLength):
			accounts.Add(size)
		case bytes.HasPrefix(key, ContractInfoPrefix) && len(key) == (len(ContractInfoPrefix)+common.AddressLength):
			contracts.Add(size)
		case bytes.HasPrefix(key, AccountIndexRefsPrefix) && len(key) == (len(AccountIndexRefsPrefix)+common.HashLength):
			indexStates.Add(size)
		case bytes.HasPrefix(key, AccountSentTxPrefix) && len(key) == (len(AccountSentTxPrefix)+common.AddressLength+8):
			indexRecords.Add(size)
		case bytes.HasPrefix(key, AccountInternalTxPrefix) && len(key) == (len(AccountInternalTxPrefix)+common.AddressLength+8):
			indexRecords.Add(size)
		case bytes.HasPrefix(key, AccountTokenTxPrefix) && len(key) == (len(AccountTokenTxPrefix)+common.AddressLength+8):
			indexRecords.Add(size)
		case bytes.HasPrefix(key, TokenHolderPrefix) && len(key) == (len(TokenHolderPrefix)+common.AddressLength+8):
			indexRecords.Add(size)
		case bytes.HasPrefix(key, FourBytesMethodPrefix) && len(key) == (len(FourBytesMethodPrefix)+4):
			fourBytes.Add(size)
		case bytes.HasPrefix(key, InterfaceABIPrefix) && bytes.HasSuffix(key, InterfaceABISuffix):
			interfaceABIs.Add(size)
		default:
			var accounted bool
			for _, meta := range [][]byte{
				LastIndexStateKey, LastIndexBlockKey, TotalAccountsKey, TotalContractsKey,
			} {
				if bytes.Equal(key, meta) {
					metadata.Add(size)
					accounted = true
					break
				}
			}
			if !accounted {
				unaccounted.Add(size)
			}
		}
		count++
		if count%1000 == 0 && time.Since(logged) > 8*time.Second {
			log.Info("Inspecting database", "count", count, "elapsed", common.PrettyDuration(time.Since(start)))
			logged = time.Now()
		}
	}

	// Display the database statistic.
	stats := [][]string{
		{"Key-Value store", "Accounts", accounts.Size(), accounts.Count()},
		{"Key-Value store", "Contracts", contracts.Size(), contracts.Count()},
		{"Key-Value store", "Account Index States", indexStates.Size(), indexStates.Count()},
		{"Key-Value store", "Account Index Data", indexRecords.Size(), indexRecords.Count()},
		{"Key-Value store", "Method Signatures", fourBytes.Size(), fourBytes.Count()},
		{"Key-Value store", "Interface ABIs", interfaceABIs.Size(), interfaceABIs.Count()},
		{"Key-Value store", "Metadata", metadata.Size(), metadata.Count()},
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Database", "Category", "Size", "Items"})
	table.SetFooter([]string{"", "Total", total.String(), " "})
	table.AppendBulk(stats)
	table.Render()

	if unaccounted.size > 0 {
		log.Error("Database contains unaccounted data", "size", unaccounted.size, "count", unaccounted.count)
	}
	return nil
}
