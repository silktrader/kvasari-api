package artworks

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"time"
)

type ArtworkRepository interface {
	AddArtwork(data AddArtworkData, userId string) (string, time.Time, error)
	DeleteArtwork(artworkId string, userId string) bool
	SetReaction(userId string, artworkId string, date time.Time, feedback ReactionData) error
}

type artworkRepository struct {
	Connection *sql.DB
}

func NewRepository(connection *sql.DB) ArtworkRepository {
	return &artworkRepository{connection}
}

var (
	ErrNotFound     = errors.New("not found")
	ErrSameReaction = errors.New("same reaction detected")
)

func (ar *artworkRepository) AddArtwork(data AddArtworkData, userId string) (id string, updated time.Time, err error) {
	// generate a new unique ID server side
	// SQLite has limited ability in this regard, while Postgresql and others have adequate features or extensions
	newUUID, err := uuid.NewV4()
	if err != nil {
		return id, updated, fmt.Errorf("couldn't generate a unique user id for %q: %w", data.Title, err)
	}
	id = newUUID.String()

	var now = time.Now()

	result, err := ar.Connection.Exec(
		"INSERT INTO artworks(id, title, type, picture_url, author_id, description, year, location, created, added, updated) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, data.Title, data.Type, data.PictureURL, userId, data.Description, data.Year, data.Location, data.Created, now, now)
	if err != nil {
		return id, updated, fmt.Errorf("couldn't add user %q: %w", data.Title, err)
	}
	rows, err := result.RowsAffected()
	if rows < 1 || err != nil {
		return id, updated, err
	}

	return id, now, nil
}

// OwnsArtwork verifies whether a given artwork exists, wasn't deleted and is owned by the specified user
func (ar *artworkRepository) OwnsArtwork(artworkId string, userId string) bool {
	var exists = false
	var err = ar.Connection.QueryRow("SELECT TRUE FROM artworks WHERE id = ? AND author_id = ? AND deleted = false", artworkId, userId).Scan(&exists)
	return err != nil && exists

}

// DeleteArtwork will perform a soft delete and return a negative result in case:
//   - the artwork doesn't exist
//   - the artwork isn't owned by the specified user
//   - the artwork was previously deleted
//
// tk handle with errors
func (ar *artworkRepository) DeleteArtwork(artworkId string, userId string) bool {

	result, err := ar.Connection.Exec(`
		UPDATE artworks SET deleted = TRUE WHERE artworks.id = ? AND author_id = ? AND deleted = FALSE`,
		artworkId,
		userId,
	)
	if err != nil {
		return false
	}
	results, err := result.RowsAffected()
	if err != nil || results != 1 {
		return false
	}
	return true
}

func (ar *artworkRepository) SetReaction(userId string, artworkId string, date time.Time, data ReactionData) error {

	if data.Reaction == None {
		return ar.removeReaction(userId, artworkId)
	}
	return ar.upsertReaction(userId, artworkId, date, data)
}

func (ar *artworkRepository) upsertReaction(userId string, artworkId string, date time.Time, data ReactionData) error {
	_, err := ar.Connection.Exec(`
		INSERT INTO artwork_feedback(artwork, user, reaction, date)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (artwork, user) DO UPDATE SET reaction = ?, date = ?`,
		artworkId, userId, data.Reaction, date, data.Reaction, date)

	return err
}

func (ar *artworkRepository) removeReaction(userId string, artworkId string) error {
	_, err := ar.Connection.Exec(`
		DELETE FROM artwork_feedback WHERE artwork = ? AND user = ?`,
		artworkId, userId)

	return err
}
