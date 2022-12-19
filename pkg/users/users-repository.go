package users

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"github.com/silktrader/kvasari/pkg/ntime"
	"github.com/silktrader/kvasari/pkg/rest"
	"time"
)

type UserRepository interface {
	GetAll() ([]User, error)
	Register(data AddUserData) (*User, error)
	GetUserById(id string) (user User, err error)
	GetUserByAlias(alias string) (user User, err error)
	UpdateName(userId string, newName string) error
	UpdateAlias(userId string, newAlias string) error

	Follow(followerId string, targetAlias string, date ntime.NTime) error
	Unfollow(followerId string, targetAlias string) error
	GetFollowers(userAlias string) ([]Follower, error)

	Ban(sourceId string, targetAlias string, date ntime.NTime) error
	Unban(sourceId string, targetAlias string) error
	GetBans(sourceId string) ([]BannedUser, error)
	GetUserRelations(userId string) ([]RelationData, []RelationData, error)
}

type userRepository struct {
	Connection *sql.DB
}

var (
	ErrDupFollower = errors.New("user already follows target")
	ErrNotFound    = errors.New("not found")
	ErrDupBan      = errors.New("user is already banned")
	ErrAliasTaken  = errors.New("alias is already taken")
	ErrDupUser     = errors.New("email or alias is already registered")
)

func closeRows(rows *sql.Rows) {
	_ = rows.Close()
}

func NewRepository(connection *sql.DB) UserRepository {
	return &userRepository{connection}
}

func (ur *userRepository) GetAll() (users []User, err error) {

	rows, err := ur.Connection.Query("SELECT id, name, alias, email, created, updated FROM users")
	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	for rows.Next() {
		var user User
		if err = rows.Scan(&user.Id, &user.Name, &user.Alias, &user.Email, &user.Created, &user.Updated); err != nil {
			return users, err
		}

		users = append(users, user)
	}

	// return partial results in case of errors
	return users, rows.Err()
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
	defer closeRows(rows)

	for rows.Next() {
		var follower Follower
		if err = rows.Scan(&follower.Id, &follower.Alias, &follower.Name, &follower.Email, &follower.Followed); err != nil {
			return followers, err
		}
		followers = append(followers, follower)
	}

	if err = rows.Err(); err != nil {
		return followers, err
	}

	return followers, nil
}

// GetUserByAlias either returns a user matching the alias, or an error (along with an ignorable empty struct).
func (ur *userRepository) GetUserByAlias(alias string) (user User, err error) {
	err = ur.Connection.QueryRow("SELECT id, name, alias, created, updated FROM users WHERE alias = ?", alias).Scan(
		&user.Id,
		&user.Name,
		&user.Alias,
		&user.Created,
		&user.Updated,
	)
	return user, err
}

// GetUserById either returns a user matching the id, or an error (along with an ignorable empty struct).
func (ur *userRepository) GetUserById(id string) (user User, err error) {
	// if the query selects no rows, `Scan` will return ErrNoRows
	err = ur.Connection.QueryRow("SELECT id, name, alias, created, updated FROM users WHERE id = ?", id).Scan(
		&user.Id,
		&user.Name,
		&user.Alias,
		&user.Created,
		&user.Updated,
	)
	return user, err
}

func (ur *userRepository) Register(data AddUserData) (*User, error) {

	var id = rest.MustGetNewUUID()
	var now = ntime.Now()

	result, err := ur.Connection.Exec(`
		INSERT INTO users(id, name, alias, email, password, created, updated)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		id, data.Name, data.Alias, data.Email, data.Password, now, now)

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return nil, ErrDupUser
		}
	}

	// generic error
	if err != nil {
		return nil, fmt.Errorf("couldn't add user %q: %w", data.Alias, err)
	}

	// tk improve handling by returning appropriate error
	rows, err := result.RowsAffected()
	if rows < 1 || err != nil {
		return nil, err
	}

	return &User{
		id,
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
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return ErrAliasTaken
		}
	}

	return err
}

func (ur *userRepository) GetUserRelations(userId string) ([]RelationData, []RelationData, error) {

	var followers, followed = make([]RelationData, 0), make([]RelationData, 0)

	rows, err := ur.Connection.Query(`
		SELECT id, is_follower, alias, name, date
		FROM (
		    SELECT follower as id, TRUE as is_follower, date
		    FROM   followers
		    WHERE  target = ?
		    UNION
		    SELECT target as id, FALSE as is_follower, date
		    FROM   followers
		    WHERE  follower = ?
		) as x
		JOIN users USING (id)
		ORDER BY date DESC`,
		userId,
		userId,
	)

	if err != nil {
		return followers, followed, err
	}

	defer closeRows(rows)

	var isFollower bool
	for rows.Next() {
		var relation RelationData
		if err = rows.Scan(&relation.Id, &isFollower, &relation.Alias, &relation.Name, &relation.Date); err != nil {
			return followers, followed, err
		}

		// append the relation to either followers or followed slices
		if isFollower {
			followers = append(followers, relation)
		} else {
			followed = append(followed, relation)
		}
	}

	if err = rows.Err(); err != nil {
		return followers, followed, err
	}

	return followers, followed, err
}
