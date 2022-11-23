package users

import (
	"database/sql"
	"fmt"
	"github.com/gofrs/uuid"
	"time"
)

type UserRepository interface {
	GetAll() ([]User, error)
	GetFollowers(userAlias string) ([]Follower, error)
	GetFollowersById(id string) ([]Follower, error)
	Register(data AddUserData) (*User, error)
	ExistsUserId(id string) bool
	ExistsUserAlias(alias string) bool
	GetUserByAlias(alias string) (user User, err error)
	UpdateName(userId string, name string) error
	UpdateAlias(userId string, name string) error
	IsFollowing(followerId string, targetId string) bool
	Follow(followerId string, targetId string) error
	Unfollow(followerId string, targetId string) (bool, error)
	Ban(initiatorId string, bannedId string) error
}

type userRepository struct {
	Connection *sql.DB
}

func NewRepository(connection *sql.DB) UserRepository {
	return &userRepository{connection}
}

func (ur *userRepository) GetAll() (users []User, err error) {
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

		users = append(users, User{
			ID:      id,
			Alias:   alias,
			Name:    name,
			Email:   email,
			Created: created,
			Updated: updated,
		})
	}

	if err = rows.Err(); err != nil {
		return users, err
	}

	if err = rows.Close(); err != nil {
		return users, err
	}

	return users, err
}

func (ur *userRepository) GetFollowers(userAlias string) ([]Follower, error) {

	// initialise empty slice to avoid null serialisation; IDE complains about `[]Follower{}`
	var followers = make([]Follower, 0)

	rows, err := ur.Connection.Query("SELECT id, alias, name, email, date FROM (SELECT follower, date FROM followers WHERE target = (SELECT id FROM users WHERE users.alias = ?)) as fws JOIN users ON fws.follower = users.id", userAlias)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var follower Follower
		if err = rows.Scan(&follower.ID, &follower.Alias, &follower.Name, &follower.Email, &follower.Followed); err != nil {
			return followers, err
		}
		followers = append(followers, follower)
	}

	if err = rows.Err(); err != nil {
		return followers, err
	}
	if err = rows.Close(); err != nil {
		return followers, err
	}

	return followers, nil
}

func (ur *userRepository) GetFollowersById(id string) ([]Follower, error) {

	var followers = make([]Follower, 0)
	rows, err := ur.Connection.Query("SELECT id, alias, name, email, date FROM (SELECT follower, date FROM followers WHERE target = ?) as fws JOIN users ON fws.follower = users.id", id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var follower Follower
		if err = rows.Scan(&follower.ID, &follower.Alias, &follower.Name, &follower.Email, &follower.Followed); err != nil {
			return followers, err
		}
		followers = append(followers, follower)
	}

	if err = rows.Err(); err != nil {
		return followers, err
	}
	if err = rows.Close(); err != nil {
		return followers, err
	}

	return followers, nil

}

func (ur *userRepository) ExistsUserId(id string) (exists bool) {
	// will return false in the absence of positive results
	err := ur.Connection.QueryRow("SELECT TRUE FROM users WHERE id = ?", id).Scan(&exists)
	return err == nil && exists
}

func (ur *userRepository) ExistsUserAlias(alias string) (exists bool) {
	// will return false in the absence of positive results
	err := ur.Connection.QueryRow("SELECT TRUE FROM users WHERE alias = ?", alias).Scan(&exists)
	return err == nil && exists
}

// GetUserByAlias either returns a user matching the alias, or an error, along with an empty struct.
func (ur *userRepository) GetUserByAlias(alias string) (user User, err error) {
	err = ur.Connection.QueryRow("SELECT id, name, email, created, updated FROM users WHERE alias = ?", alias).Scan(&user.ID, &user.Name, &user.Alias, &user.Created, &user.Updated)
	if err != nil {
		return User{}, err
	}
	return user, err
}

func (ur *userRepository) Register(data AddUserData) (user *User, err error) {

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

func (ur *userRepository) UpdateName(userId string, name string) error {
	// avoid using DB triggers for possible future storage switches
	_, err := ur.Connection.Exec("UPDATE users SET name = ?, updated = ? WHERE id = ?", name, time.Now(), userId)
	return err
}

func (ur *userRepository) UpdateAlias(userId string, alias string) error {
	// avoid using DB triggers for possible future storage switches
	_, err := ur.Connection.Exec("UPDATE users SET alias = ?, updated = ? WHERE id = ?", alias, time.Now(), userId)
	return err
}

func (ur *userRepository) IsFollowing(followerId string, targetId string) (exists bool) {
	var err = ur.Connection.QueryRow("SELECT TRUE FROM followers WHERE follower = ? AND target = ?", followerId, targetId).Scan(&exists)
	return err == nil && exists
}

func (ur *userRepository) Follow(followerId string, targetId string) error {
	_, err := ur.Connection.Exec("INSERT INTO followers (follower, target, date) VALUES (?, ?, ?)", followerId, targetId, time.Now())
	return err
}

// Unfollow returns true when users are properly unfollowed or an error otherwise.
func (ur *userRepository) Unfollow(followerId string, targetId string) (bool, error) {
	result, err := ur.Connection.Exec("DELETE FROM followers WHERE follower = ? AND target = ?", followerId, targetId)
	if err != nil {
		return false, err
	}
	unfollowed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return unfollowed == 1, err
}

func (ur *userRepository) Ban(initiatorId string, bannedId string) error {
	//_, err = ur.Connection.Exec("INSERT INTO bans ")
	return nil
}
