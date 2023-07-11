package leth

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

func checkBlockChainVersion(chainDb ethdb.Database) error {
	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}

	if bcVersion != nil && *bcVersion > core.BlockChainVersion {
		return fmt.Errorf("database version is v%d, Geth %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
	} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
		if bcVersion != nil { // only print warning on upgrade, not on init
			log.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
		} else {
			log.Info("Initialize blockchain database version", "dbVer", core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}
	return nil
}
