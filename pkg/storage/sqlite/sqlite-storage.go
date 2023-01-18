package sqlite

import (
	"database/sql"
	"errors"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

type Storage struct {
	Logger     *logrus.Logger
	Connection *sql.DB
}

var ErrSchemaMismatch = errors.New("schema mismatch")

// New sets up a SQLite database connection and performs basic maintenance, such as:
//   - verify the current DB instance schema matches the designed one
func New(logger *logrus.Logger, path string) (storage Storage, err error) {
	storage.Logger = logger
	logger.Println("initialising SQLite DB")

	// the database already exists, check for its contents
	if _, err = os.Stat(path); err == nil {
		storage.Connection, err = getValidConnection(path)
		if err != nil {
			logger.WithError(err).Error("error while verifying existing database")
			return storage, err
		}
	} else {
		// create the file and initialise the schema; mind the explicit need for foreign keys constraints
		// can wrap errors to discriminate source; a debatable practice
		storage.Connection, err = sql.Open("sqlite3", getConnectionString(path))
		if err != nil {
			logger.WithError(err).Error("error while creating new database")
			return storage, err
		}

		if _, err = storage.Connection.Exec(schema); err != nil {
			logger.WithError(err).Error("error while building database schema")
			return storage, err
		}
	}

	// opening the DB will fail silently when the package is compiled without CGO_ENABLED
	return storage, storage.Connection.Ping()
}

func getValidConnection(path string) (connection *sql.DB, err error) {
	if connection, err = sql.Open("sqlite3", getConnectionString(path)); err != nil {
		return nil, err
	}

	// read the schema as defined in the storage package
	desired, err := sql.Open("sqlite3", getConnectionString(":memory:"))
	if err != nil {
		return nil, err
	}

	if _, err = desired.Exec(schema); err != nil {
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
	return nil, ErrSchemaMismatch
}

func mapSchema(connection *sql.DB) (tables map[string]string, err error) {
	rows, err := connection.Query(`SELECT name, sql FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	// for some reason in memory and on file sqlite schemas differ, possibly due to the hosting platform
	var replacer = strings.NewReplacer(
		"\n\t\t", "",
		"\r\n\t\t", "",
		"\r\n", "",
		"\n", "",
	)

	tables = make(map[string]string)
	var name, sqlCode string
	for rows.Next() {
		if err = rows.Scan(&name, &sqlCode); err != nil {
			return tables, err
		}
		tables[name] = replacer.Replace(sqlCode)
	}

	return tables, rows.Err()
}

func sameSchemaMap(first, second map[string]string) bool {
	// the second map might be larger than the first, hence the additional length check
	if len(first) != len(second) {
		return false
	}
	for firstKey, firstValue := range first {
		if secondValue, found := second[firstKey]; !found || secondValue != firstValue {
			return false
		}
	}
	return true
}

// getConnectionString provides a configuration string that enables foreign keys constraints.
func getConnectionString(path string) string {
	return path + "?_fk=on"
}

func (storage *Storage) Close() {
	storage.Logger.Infof("closing SQLite DB")
	_ = storage.Connection.Close()
}
