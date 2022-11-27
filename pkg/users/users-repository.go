package users

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/mattn/go-sqlite3"
	"time"
)

type UserRepository interface {
	GetAll() ([]User, error)
	Register(data AddUserData) (*User, error)
	ExistsUserId(id string) bool
	ExistsUserAlias(alias string) bool
	GetUserById(id string) (user User, err error)
	GetUserByAlias(alias string) (user User, err error)
	UpdateName(userId string, newName string) error
	UpdateAlias(userId string, newAlias string) error

	IsFollowing(followerId string, targetId string) bool
	Follow(followerId string, targetAlias string) error
	Unfollow(followerId string, targetAlias string) error
	GetFollowers(userAlias string) ([]Follower, error)
	GetFollowersById(id string) ([]Follower, error)

	Ban(sourceId string, targetAlias string) error
	GetBans(sourceId string) ([]BannedUser, error)
}

type userRepository struct {
	Connection *sql.DB
}

var (
	ErrDupFollower = errors.New("user already follows target")
	ErrNotFound    = errors.New("user not found")
	ErrDupBan      = errors.New("user is already banned")
	ErrAliasTaken  = errors.New("alias is already taken")
)

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
			Id:      id,
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

	rows, err := ur.Connection.Query(`
		SELECT id, alias, name, email, date
		FROM (SELECT follower, date FROM followers WHERE target = (SELECT id FROM users WHERE users.alias = ?)) as fws
		JOIN users ON fws.follower = users.id`,
		userAlias,
	)
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
	rows, err := ur.Connection.Query(`
		SELECT id, alias, name, email, date
		FROM (SELECT follower, date FROM followers WHERE target = ?) as fws
		JOIN users ON fws.follower = users.id`,
		id,
	)
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

// GetUserByAlias either returns a user matching the alias, or an error (along with an ignorable empty struct).
func (ur *userRepository) GetUserByAlias(alias string) (user User, err error) {
	err = ur.Connection.QueryRow("SELECT id, name, email, created, updated FROM users WHERE alias = ?", alias).Scan(
		&user.Id,
		&user.Name,
		&user.Alias,
		&user.Created,
		&user.Updated,
	)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// GetUserById either returns a user matching the id, or an error (along with an ignorable empty struct).
func (ur *userRepository) GetUserById(id string) (user User, err error) {
	// if the query selects no rows, *Row's `Scan` will return ErrNoRows
	if err = ur.Connection.QueryRow("SELECT id, name, email, created, updated FROM users WHERE id = ?", id).Scan(
		&user.Id,
		&user.Name,
		&user.Alias,
		&user.Created,
		&user.Updated,
	); err != nil {
		return user, err
	}
	return user, nil
}

func (ur *userRepository) Register(data AddUserData) (user *User, err error) {

	// generate a new unique Id
	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("couldn't generate a unique user id for %q: %w", data.Alias, err)
	}

	var now = time.Now()

	result, err := ur.Connection.Exec(
		"INSERT INTO users(id, name, alias, email, salt, password, created, updated) VALUES(?, ?, ?, ?, ?, ?, ?, ?)",
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

func (ur *userRepository) UpdateName(userId string, newName string) error {
	// avoid using DB triggers for possible future storage switches
	_, err := ur.Connection.Exec("UPDATE users SET name = ?, updated = ? WHERE id = ?", newName, time.Now(), userId)
	return err
}

// UpdateAlias will change the specified user's alias, but won't return errors in case of no changes
func (ur *userRepository) UpdateAlias(userId string, newAlias string) error {
	// avoid using DB triggers for possible future storage switches
	// idempotent PUT request doesn't require a change detection
	_, err := ur.Connection.Exec("UPDATE users SET alias = ?, updated = ? WHERE id = ?", newAlias, time.Now(), userId)

	// detect alias uniqueness violations which signal that the alias is taken
	if sqliteErr, ok := err.(sqlite3.Error); ok {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return ErrAliasTaken
		}
	}
	return err
}

func (ur *userRepository) IsFollowing(followerId string, targetId string) (exists bool) {
	var err = ur.Connection.QueryRow(
		"SELECT TRUE FROM followers WHERE follower = ? AND target = ?",
		followerId,
		targetId,
	).Scan(&exists)
	return err == nil && exists
}

func (ur *userRepository) Follow(followerId string, targetAlias string) error {
	res, err := ur.Connection.Exec(`
		INSERT INTO followers (follower, target, date)
		SELECT ?, id as targetId, datetime('now')
		FROM users WHERE alias = ? AND ? NOT IN (SELECT target FROM bans WHERE source = targetId)`,
		followerId,
		targetAlias,
		followerId,
	)

	// detects whether the requester is already among the target's followers
	if sqliteErr, ok := err.(sqlite3.Error); ok {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return ErrDupFollower
		}
	}

	// unspecified error occurred, should be handled as 50x
	if err != nil {
		return err
	}

	// when no rows are affected the requester was either banned or the target user doesn't exist
	rows, err := res.RowsAffected()
	if rows != 1 {
		return ErrNotFound
	}
	return err
}

func (ur *userRepository) Unfollow(followerId string, targetAlias string) error {
	result, err := ur.Connection.Exec(
		`DELETE FROM followers WHERE follower = ? AND target IN (SELECT id FROM users WHERE alias = ?)`,
		followerId,
		targetAlias,
	)
	if err != nil {
		return err
	}
	unfollowed, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if unfollowed == 0 {
		return ErrNotFound
	}

	return err
}

// Ban will return true for successful bans, false when no new bans are detected, or an error when the operation fails.
func (ur *userRepository) Ban(sourceId string, targetAlias string) error {
	tx, err := ur.Connection.Begin()
	if err != nil {
		return err
	}

	// rolling back after a transaction commit will result in a safe NOP
	defer tx.Rollback()

	// the INSERT must follow the DELETE statement, so to return a relevant `RowsAffected` count
	res, err := tx.Exec(`
		DELETE FROM followers WHERE follower IN (SELECT id FROM users WHERE alias = ?) AND target = ?;
		INSERT INTO bans (source, date, target) SELECT ?, datetime('now'), id FROM users WHERE alias = ?;
	`, targetAlias, sourceId, sourceId, targetAlias)

	// detects whether the requester is already among the target's followers
	if sqliteErr, ok := err.(sqlite3.Error); ok {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return ErrDupBan
		}
	}

	// unspecified error, handled with 500x server response
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return err
}

func (ur *userRepository) IsBanning(sourceId string, targetId string) (isBanning bool) {
	var err = ur.Connection.QueryRow(
		"SELECT TRUE FROM bans WHERE source = ? AND target = ?",
		sourceId,
		targetId,
	).Scan(&isBanning)
	return err == nil && isBanning

}

func (ur *userRepository) GetBans(id string) ([]BannedUser, error) {
	var banned = make([]BannedUser, 0)
	rows, err := ur.Connection.Query(`
		SELECT id, alias, name, date
		FROM (SELECT target, date FROM bans WHERE source = ?) as banned JOIN users on banned.target = users.id`,
		id,
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var bannedUser BannedUser
		if err = rows.Scan(&bannedUser.Id, &bannedUser.Alias, &bannedUser.Name, &bannedUser.Banned); err != nil {
			return banned, err
		}
		banned = append(banned, bannedUser)
	}

	if err = rows.Err(); err != nil {
		return banned, err
	}

	if err = rows.Close(); err != nil {
		return banned, err
	}

	return banned, nil

}
