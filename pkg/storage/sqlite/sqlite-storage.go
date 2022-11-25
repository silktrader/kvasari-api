package sqlite

import (
	"database/sql"
	"errors"
	"os"

	"github.com/sirupsen/logrus"
)

type Storage struct {
	Connection *sql.DB
}

// tk this seems unorthodox; what if there are two Initialise calls? How to implement singletons? Nil checks?
// create struct instead to be returned?
func Initialise(logger *logrus.Logger, path string) (connection *sql.DB, err error) {
	// tk pass logger interface

	logger.Println("initialising SQLite DB")

	// the database already exists, check for its contents
	if _, err := os.Stat(path); err == nil {

		connection, err = getValidConnection(path)
		if err != nil {
			logger.WithError(err).Error("error while verifying existing database")
			return nil, err
		}
	} else {
		// create the file and initialise the schema; mind the explicit need for foreign keys constraints
		connection, err = sql.Open("sqlite3", getConnectionString(path))
		if err != nil {
			logger.WithError(err).Error("error while creating new database")
			return nil, err
		}
		_, err = connection.Exec(schema)
		if err != nil {
			logger.WithError(err).Error("error while building database schema")
			return nil, err
		}
	}

	// opening the DB will fail silently when the package is compiled without CGO_ENABLED
	if err = connection.Ping(); err != nil {
		return nil, err
	}
	return connection, err
}

func getValidConnection(path string) (connection *sql.DB, err error) {
	connection, err = sql.Open("sqlite3", getConnectionString(path))
	if err != nil {
		return nil, err
	}

	// read the schema as defined in the storage package
	desired, err := sql.Open("sqlite3", getConnectionString(":memory:"))
	if err != nil {
		return nil, err
	}
	_, err = desired.Exec(schema)
	if err != nil {
		return nil, err
	}

	// compare the defined schema with the actual one found in the existing database
	desiredTables, err := mapSchema(desired)
	if err != nil {
		return nil, err
	}
	actualTables, err := mapSchema(connection)
	if err != nil {
		return nil, err
	}

	// the database already exists and its schema matches the desired one
	if sameSchemaMap(desiredTables, actualTables) {
		return connection, nil
	}
	return nil, errors.New("schema mismatch")
}

func mapSchema(connection *sql.DB) (tables map[string]string, err error) {

	const query = "SELECT name, sql FROM sqlite_schema WHERE type = 'table' AND name != 'sqlite_sequence'"
	rows, err := connection.Query(query)
	if err != nil {
		return nil, err
	}

	tables = make(map[string]string)
	var name, sqlCode string
	for rows.Next() {
		err = rows.Scan(&name, &sqlCode)
		if err != nil {
			return tables, err
		}
		tables[name] = sqlCode
	}

	// the official documentation suggests that rows be closed, but doesn't handle errors
	// https://github.com/mattn/go-sqlite3/blob/master/_example/simple/simple.go
	defer func() {
		err = rows.Close()
	}()

	if err = rows.Err(); err != nil {
		return tables, err
	}

	return tables, nil
}

func sameSchemaMap(first, second map[string]string) bool {
	// the second map might be larger than the first, hence the additional length check
	if first == nil || second == nil || len(first) != len(second) {
		return false
	}
	for firstKey, firstValue := range first {
		if secondValue, found := second[firstKey]; !found || secondValue != firstValue {
			return false
		}
	}
	return true
}

// getConnectionString provides a configuration string that enables foreign keys constraints
func getConnectionString(path string) string {
	return path + "?_fk=on"
}

func Close(logger *logrus.Logger) {
	logger.Debug("database stopping")
	// tk is it safe to ignore errors?
	//_ = connection.Close()
}
