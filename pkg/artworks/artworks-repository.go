package artworks

import (
	"database/sql"
	"fmt"
	"github.com/gofrs/uuid"
	"time"
)

type ArtworkRepository interface {
	AddArtwork(data AddArtworkData, userId string) (string, time.Time, error)
	DeleteArtwork(artworkId string, userId string) bool
}

type artworkRepository struct {
	Connection *sql.DB
}

func NewRepository(connection *sql.DB) ArtworkRepository {
	return &artworkRepository{connection}
}

func (ar *artworkRepository) AddArtwork(data AddArtworkData, userId string) (id string, updated time.Time, err error) {

	// generate a new unique ID
	newUUID, err := uuid.NewV4()
	id = newUUID.String()
	if err != nil {
		return id, updated, fmt.Errorf("couldn't generate a unique user id for %q: %w", data.Title, err)
	}

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
func (ar *artworkRepository) DeleteArtwork(artworkId string, userId string) bool {

	result, err := ar.Connection.Exec("UPDATE artworks SET deleted = TRUE WHERE artworks.id = ? AND author_id = ? AND deleted = FALSE", artworkId, userId)
	if err != nil {
		return false
	}
	results, err := result.RowsAffected()
	if err != nil || results != 1 {
		return false
	}
	return true
}
