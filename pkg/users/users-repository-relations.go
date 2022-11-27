package users

import "github.com/mattn/go-sqlite3"

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

// Unban attempts to remove a user ban and can return:
//  1. ErrNotFound: the source didn't ban the target to start with
//  2. a generic SQL error that occurred during the operation
//  3. nil
func (ur *userRepository) Unban(sourceId string, targetAlias string) error {
	result, err := ur.Connection.Exec(
		`DELETE FROM bans WHERE source = ? AND target IN (SELECT id FROM users WHERE alias = ?)`,
		sourceId,
		targetAlias,
	)

	if err != nil {
		return err
	}

	banned, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if banned == 0 {
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
