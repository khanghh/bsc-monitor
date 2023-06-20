package service

import "github.com/ethereum/go-ethereum/ethdb"

// closeTrackingDB wraps the Close method of a database. When the database is closed by the
// service, the wrapper removes it from the node's database map. This ensures that Node
// won't auto-close the database if it is closed by the service that opened it.
type closeTrackingDB struct {
	ethdb.Database
	namespace string
	stack     *ServiceStack
}

func (db *closeTrackingDB) Close() error {
	db.stack.lock.Lock()
	delete(db.stack.databases, db.namespace)
	db.stack.lock.Unlock()
	return db.Database.Close()
}

// wrapDatabase ensures the database will be auto-closed when Node is closed.
func (n *ServiceStack) wrapDatabase(namespace string, db ethdb.Database) ethdb.Database {
	wrapper := &closeTrackingDB{db, namespace, n}
	n.databases[namespace] = wrapper
	return wrapper
}

// closeDatabases closes all open databases.
func (n *ServiceStack) closeDatabases() (errors []error) {
	for name, db := range n.databases {
		delete(n.databases, name)
		if err := db.Database.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}
