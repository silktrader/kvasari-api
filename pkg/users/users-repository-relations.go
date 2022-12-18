package users

import (
	"errors"
	"github.com/mattn/go-sqlite3"
	"github.com/silktrader/kvasari/pkg/ntime"
)

func (ur *userRepository) Follow(followerId string, targetAlias string, date ntime.NTime) error {
	res, err := ur.Connection.Exec(`
		INSERT INTO followers (follower, target, date)
		SELECT ?, id as targetId, ?
		FROM users WHERE alias = ? AND ? NOT IN (SELECT target FROM bans WHERE source = targetId)`,
		followerId, date, targetAlias, followerId,
	)

	// detects whether the requester is already among the target's followers
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return ErrDupFollower
		}
	}

	// unspecified error occurred, should be handled as 50x
	if err != nil {
		return err
	}

	// when no rows are affected the requester was either banned or the target user doesn't exist
	if followed, e := res.RowsAffected(); e != nil {
		return e
	} else if followed == 0 {
		return ErrNotFound
	}

	return err
}

// Unfollow returns nil for successful operations, or adequate errors on failure,
// such as when a target user isn't followed.
func (ur *userRepository) Unfollow(followerId string, targetAlias string) error {
	result, err := ur.Connection.Exec(
		`DELETE FROM followers WHERE follower = ? AND target IN (SELECT id FROM users WHERE alias = ?)`,
		followerId,
		targetAlias,
	)
	if err != nil {
		return err
	}

	if unfollowed, e := result.RowsAffected(); e != nil {
		return e
	} else if unfollowed == 0 {
		return ErrNotFound
	}

	return err
}

// Ban returns nil for successful operations, or adequate errors on failure,
// such as when the target is already banned. A ban elicits the successive unfollowing of the source by the target.
func (ur *userRepository) Ban(sourceId string, targetAlias string, date ntime.NTime) error {
	tx, err := ur.Connection.Begin()
	if err != nil {
		return err
	}

	// rolling back after a transaction commit will result in a safe NOP
	defer func() {
		_ = tx.Rollback()
	}()

	// the INSERT must follow the DELETE statement, so to return a relevant `RowsAffected` count
	res, err := tx.Exec(`
		DELETE FROM followers WHERE follower IN (SELECT id FROM users WHERE alias = ?) AND target = ?;
		INSERT INTO bans (source, date, target) SELECT ?, ?, id FROM users WHERE alias = ?;
	`, targetAlias, sourceId, sourceId, date, targetAlias)

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

	if affected, e := res.RowsAffected(); e != nil {
		return e
	} else if affected == 0 {
		return ErrNotFound
	}
	return err
}

// Unban attempts to remove a user ban and can return:
//  1. ErrNotFound: the source didn't ban the target to start with
//  2. a generic SQL error that occurred during the operation
//  3. nil
func (ur *userRepository) Unban(sourceId string, targetAlias string) error {
	result, err := ur.Connection.Exec(
		`DELETE FROM bans WHERE source = ? AND target IN (SELECT id FROM users WHERE alias = ?)`,
		sourceId, targetAlias,
	)

	if err != nil {
		return err
	}

	// return an appropriate error when the target user isn't found
	if banned, e := result.RowsAffected(); e != nil {
		return e
	} else if banned == 0 {
		return ErrNotFound
	}

	return err
}

// GetBans fetches all the users banned by the source ID, providing the
// ID, alias, name and the date of the ban of each targeted user.
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

	defer closeRows(rows)

	for rows.Next() {
		var bannedUser BannedUser
		if err = rows.Scan(&bannedUser.Id, &bannedUser.Alias, &bannedUser.Name, &bannedUser.Banned); err != nil {
			return banned, err
		}
		banned = append(banned, bannedUser)
	}

	return banned, rows.Err()
}
