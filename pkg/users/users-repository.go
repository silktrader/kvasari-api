package users

import (
	"database/sql"
	"fmt"
	"github.com/gofrs/uuid"
	"time"
)

type UserRepository interface {
	GetUsers() ([]User, error)
	AddUser(data AddUserData) (*User, error)
	ExistsUserId(id string) bool

	UpdateUserName(userId string, data UpdateUserNameData) error
}

type userRepository struct {
	Connection *sql.DB
}

func NewRepository(connection *sql.DB) UserRepository {
	return &userRepository{connection}
}

func (ur *userRepository) GetUsers() (users []User, err error) {
	rows, err := ur.Connection.Query("select id, name, alias, email, created, updated from users")
	if err != nil {
		return nil, err
	}
	//defer func() {
	//	err = rows.Close()
	//}()

	var id, name, alias, email string
	var created, updated time.Time

	for rows.Next() {
		// return partial results in case of errors
		if err = rows.Scan(&id, &name, &alias, &email, &created, &updated); err != nil {
			return users, err
		}

		users = append(users, User{id, name, alias, email, created, updated})
	}

	if err = rows.Err(); err != nil {
		return users, err
	}

	if err = rows.Close(); err != nil {
		return users, err
	}

	return users, err
}

func (ur *userRepository) ExistsUserId(id string) bool {
	// will return false in the absence of positive results
	var exists = false
	err := ur.Connection.QueryRow("select true from users where id = ?", id).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

//func (ur *userRepository) CheckAlias(alias string) (user User, err error) {
//
//	var id, name, alias, email string
//	var created, updated time.Time
//
//	err = ur.Connection.QueryRow("select id, name, alias, email, created, updated from users").Scan(&id, &name, &alias, &)
//	if err != nil {
//		return nil, err
//	}
//}

func (ur *userRepository) AddUser(data AddUserData) (user *User, err error) {

	// generate a new unique ID
	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("couldn't generate a unique user id for %q: %w", data.Alias, err)
	}

	var now = time.Now()

	result, err := ur.Connection.Exec("INSERT INTO users(id, name, alias, email, salt, password, created, updated) VALUES(?, ?, ?, ?, ?, ?, ?, ?)",
		id.String(), data.Name, data.Alias, data.Email, data.Password, "", now, now)
	if err != nil {
		return nil, fmt.Errorf("couldn't add user %q: %w", data.Alias, err)
	}
	rows, err := result.RowsAffected()
	if rows < 1 || err != nil {
		return nil, err
	}

	return &User{
		id.String(),
		data.Alias,
		data.Name,
		data.Email,
		now,
		now,
	}, nil
}

func (ur *userRepository) UpdateUserName(userId string, data UpdateUserNameData) error {
	// avoid using DB triggers for possible future storage switches
	_, err := ur.Connection.Exec("UPDATE users SET name = ?, updated = ? WHERE id = ?", data.Name, time.Now(), userId)
	if err != nil {
		return err
	}
	return nil
}
