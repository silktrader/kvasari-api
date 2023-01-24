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
	GetFilteredUsers(filter string, requesterId string) ([]User, error)
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

	GetDetails(alias string, requesterId string) (details UserDetails, err error)
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

func (ur *userRepository) GetFilteredUsers(filter string, requesterId string) ([]User, error) {
	var users = make([]User, 0) // always return an empty list, rather than null
	var filterPattern = fmt.Sprintf("%%%s%%", filter)
	rows, err := ur.Connection.Query(`
		SELECT id, name, alias, email, created, updated FROM users
		WHERE id != ?
		AND (alias LIKE ? OR name LIKE ?)
		AND ? NOT IN (SELECT target FROM bans WHERE source = users.id)`,
		requesterId, filterPattern, filterPattern, requesterId)
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
	return user, ur.Connection.QueryRow("SELECT id, name, alias, created, updated FROM users WHERE alias = ?", alias).Scan(
		&user.Id,
		&user.Name,
		&user.Alias,
		&user.Created,
		&user.Updated,
	)
}

// GetUserById either returns a user matching the id, or an error (along with an ignorable empty struct).
func (ur *userRepository) GetUserById(id string) (user User, err error) {
	// if the query selects no rows, `Scan` will return ErrNoRows
	return user, ur.Connection.QueryRow("SELECT id, name, alias, created, updated FROM users WHERE id = ?", id).Scan(
		&user.Id,
		&user.Name,
		&user.Alias,
		&user.Created,
		&user.Updated,
	)
}

func (ur *userRepository) Register(data AddUserData) (*User, error) {
	var id = rest.MustGetNewUUID()
	var now = ntime.Now()
	if _, err := ur.Connection.Exec(`
		INSERT INTO users(id, name, alias, email, password, created, updated)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		id, data.Name, data.Alias, data.Email, data.Password, now, now); err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return nil, ErrDupUser
		}
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

func (ur *userRepository) GetDetails(alias string, requesterId string) (details UserDetails, err error) {
	if err = ur.Connection.QueryRow(`
		WITH author_artworks AS (SELECT id FROM artworks WHERE author_id = users.id AND NOT deleted)
		SELECT
			name,
			email,
			created,
			updated,
			(SELECT EXISTS (SELECT TRUE FROM followers WHERE follower = users.id AND target = ?)) as followsUser,
			(SELECT EXISTS (SELECT TRUE FROM followers WHERE follower = ? AND target = users.id)) as followedByUser,
			(SELECT EXISTS (SELECT TRUE FROM bans WHERE target = users.id AND source = ?)) as blockedByUser,
			(SELECT count(follower) FROM followers WHERE target = users.id) as followers,
			(SELECT count(target) FROM followers WHERE follower = users.id) as following,
			(SELECT count(id) FROM author_artworks) as artworks,
			(SELECT count(id) FROM artwork_comments WHERE artwork IN author_artworks) as comments,
			(SELECT count(user) FROM artwork_feedback WHERE artwork IN author_artworks) as reactions
		FROM users
		WHERE alias = ? AND ? NOT IN (SELECT target FROM bans WHERE source = users.id)`,
		requesterId,
		requesterId,
		requesterId,
		alias,
		requesterId,
	).Scan(
		&details.Name,
		&details.Email,
		&details.Created,
		&details.Updated,
		&details.FollowsUser,
		&details.FollowedByUser,
		&details.BlockedByUser,
		&details.Followers,
		&details.Following,
		&details.Artworks,
		&details.Comments,
		&details.Reactions,
	); errors.Is(err, sql.ErrNoRows) {
		return details, ErrNotFound
	}
	return details, err
}
