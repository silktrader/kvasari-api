package artworks

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/silktrader/kvasari/pkg/users"
	"time"
)

type ArtworkRepository interface {
	AddArtwork(data AddArtworkData, userId string) (string, time.Time, error)
	DeleteArtwork(artworkId string, userId string) bool
	SetReaction(userId string, artworkId string, date time.Time, feedback ReactionData) error
	AddComment(userId string, artworkId string, data CommentData) (id string, date time.Time, err error)
	DeleteComment(userId string, commentId string) error

	GetProfileData(userId string) (ProfileData, error)
	GetUserArtworks(userId string, pageSize int, page int) ([]ArtworkProfilePreview, error)
}

type artworkRepository struct {
	Connection     *sql.DB
	UserRepository users.UserRepository
}

func NewRepository(connection *sql.DB, userRepository users.UserRepository) ArtworkRepository {
	return &artworkRepository{connection, userRepository}
}

var (
	ErrNotFound = errors.New("not found")
)

func closeRows(rows *sql.Rows) {
	_ = rows.Close()
}

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

func (ar *artworkRepository) AddComment(userId string, artworkId string, data CommentData) (id string, date time.Time, err error) {

	newId, err := uuid.NewV4()
	if err != nil {
		return id, date, err
	}

	id = newId.String()
	date = time.Now()

	_, err = ar.Connection.Exec(`
		INSERT INTO artwork_comments (id, artwork, user, comment, date) VALUES (?, ?, ?, ?, ?)`,
		id, artworkId, userId, data.Comment, date)

	return id, date, err
}

func (ar *artworkRepository) DeleteComment(userId string, commentId string) error {

	result, err := ar.Connection.Exec(`DELETE FROM artwork_comments WHERE id = ? AND user = ?`, commentId, userId)

	if err != nil {
		return err
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if deleted != 1 {
		return ErrNotFound
	}
	return err
}

func (ar *artworkRepository) GetProfileData(userId string) (ProfileData, error) {

	// fetch the user's relations
	followers, followed, err := ar.UserRepository.GetUserRelations(userId)

	if err != nil {
		return ProfileData{}, err
	}

	// fetch the user's sorted artwork
	artworks, err := ar.GetUserArtworks(userId, 20, 0)

	// return actual data or last error
	return ProfileData{artworks, followers, followed}, err
}

func (ar *artworkRepository) GetUserArtworks(userId string, pageSize int, page int) ([]ArtworkProfilePreview, error) {

	var artworks = make([]ArtworkProfilePreview, 0)
	rows, err := ar.Connection.Query(`
		SELECT id, title, picture_url, added FROM artworks WHERE author_id = ? ORDER BY added DESC LIMIT ? OFFSET ?`,
		userId, pageSize, page,
	)

	if err != nil {
		return artworks, err
	}

	defer closeRows(rows)

	for rows.Next() {
		var artwork ArtworkProfilePreview
		if err = rows.Scan(&artwork.ID, &artwork.Title, &artwork.PictureURL, &artwork.Added); err != nil {
			return artworks, err
		}
		artworks = append(artworks, artwork)
	}

	return artworks, rows.Err()
}
