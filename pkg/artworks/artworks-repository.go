package artworks

import (
	"database/sql"
	"fmt"
	"github.com/gofrs/uuid"
	"time"
)

type ArtworkRepository interface {
	AddArtwork(data AddArtworkData, userId string) (string, time.Time, error)
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
