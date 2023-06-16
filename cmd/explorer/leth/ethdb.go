//
// Created on 2022/12/20 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package leth

import (
	"path/filepath"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

// closeTrackingDB wraps the Close method of a database. When the database is closed by the
// service, the wrapper removes it from the node's database map. This ensures that Node
// won't auto-close the database if it is closed by the service that opened it.
type closeTrackingDB struct {
	ethdb.Database
	leth *LightEthereum
}

func (db *closeTrackingDB) Close() error {
	db.leth.lock.Lock()
	delete(db.leth.databases, db)
	db.leth.lock.Unlock()
	return db.Database.Close()
}

// wrapDatabase ensures the database will be auto-closed when Node is closed.
func (leth *LightEthereum) wrapDatabase(db ethdb.Database) ethdb.Database {
	wrapper := &closeTrackingDB{db, leth}
	leth.databases[wrapper] = struct{}{}
	return wrapper
}

// closeDatabases closes all open databases.
func (leth *LightEthereum) closeDatabases() (errors []error) {
	for db := range leth.databases {
		delete(leth.databases, db)
		if err := db.Database.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// ResolvePath resolves path in the instance directory.
func (leth *LightEthereum) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if leth.config.DataDir == "" {
		return ""
	}
	return filepath.Join(leth.config.DataDir, path)
}

// OpenDatabase opens an existing database with the given name (or creates one if no
// previous can be found) from within the node's instance directory. If the node is
// ephemeral, a memory database is returned.
func (leth *LightEthereum) OpenDatabase(name string, cache, handles int, namespace string, readonly bool) (ethdb.Database, error) {
	leth.lock.Lock()
	defer leth.lock.Unlock()

	var db ethdb.Database
	var err error
	if leth.config.DataDir == "" {
		db = rawdb.NewMemoryDatabase()
	} else {
		db, err = rawdb.NewLevelDBDatabase(leth.ResolvePath(name), cache, handles, namespace, readonly)
	}

	if err == nil {
		db = leth.wrapDatabase(db)
	}
	return db, err
}

// OpenDatabaseWithFreezer opens an existing database with the given name (or
// creates one if no previous can be found) from within the node's data directory,
// also attaching a chain freezer to it that moves ancient chain data from the
// database to immutable append-only files. If the node is an ephemeral one, a
// memory database is returned.
func (leth *LightEthereum) OpenDatabaseWithFreezer(name string, cache, handles int, freezer, namespace string, readonly, disableFreeze, isLastOffset, pruneAncientData, skipCheckFreezerType bool) (ethdb.Database, error) {
	leth.lock.Lock()
	defer leth.lock.Unlock()

	var db ethdb.Database
	var err error
	if leth.config.DataDir == "" {
		db = rawdb.NewMemoryDatabase()
	} else {
		root := leth.ResolvePath(name)
		switch {
		case freezer == "":
			freezer = filepath.Join(root, "ancient")
		case !filepath.IsAbs(freezer):
			freezer = leth.ResolvePath(freezer)
		}
		db, err = rawdb.NewLevelDBDatabaseWithFreezer(root, cache, handles, freezer, namespace, readonly, disableFreeze, isLastOffset, pruneAncientData, skipCheckFreezerType)
	}

	if err == nil {
		db = leth.wrapDatabase(db)
	}
	return db, err
}
