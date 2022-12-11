package artworks

import (
	"database/sql"
	"errors"
	"github.com/silktrader/kvasari/pkg/ntime"
	"github.com/silktrader/kvasari/pkg/rest"
	"github.com/silktrader/kvasari/pkg/users"
)

type ArtworkRepository interface {
	AddArtwork(data AddArtworkData) (string, ntime.NTime, error)
	DeleteArtwork(artworkId string, userId string) bool
	GetArtwork(artworkId string, requesterId string) (*Artwork, error)

	SetReaction(userId string, artworkId string, date ntime.NTime, feedback AddReactionRequest) error
	RemoveReaction(userId string, artworkId string) error
	AddComment(userId string, artworkId string, data AddCommentData) (id string, date ntime.NTime, err error)
	DeleteComment(userId string, commentId string) error

	GetProfileData(userId string) (ProfileData, error)
	GetUserArtworks(userId string, pageSize int, page int) ([]ArtworkProfilePreview, int, error)
	GetStream(userId string, since string, latest string) (data StreamData, err error)
}

type artworkRepository struct {
	Connection     *sql.DB
	UserRepository users.UserRepository
}

func NewRepository(connection *sql.DB, userRepository users.UserRepository) ArtworkRepository {
	return &artworkRepository{connection, userRepository}
}

var (
	ErrNotFound    = errors.New("not found")
	ErrNotModified = errors.New("not modified")
)

func closeRows(rows *sql.Rows) {
	_ = rows.Close()
}

func (ar *artworkRepository) AddArtwork(data AddArtworkData) (string, ntime.NTime, error) {

	var id = rest.MustGetNewUUID()
	var now = ntime.Now()

	result, err := ar.Connection.Exec(`
		INSERT INTO artworks(id, title, type, picture_url, author_id, description, year, location, created, added, updated)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, data.Title, data.Type, data.PictureURL, data.AuthorId, data.Description, data.Year, data.Location, data.Created, now, now)

	// don't bother checking for unique constraints with UUID generation
	if err != nil {
		return id, now, err
	}

	// tk check whether needed
	rows, err := result.RowsAffected()
	if err != nil || rows < 1 {
		return id, now, err
	}

	return id, now, nil
}

func (ar *artworkRepository) GetArtwork(artworkId string, requesterId string) (*Artwork, error) {

	// get artwork metadata, ensuring a banned user is denied access
	var metadata = ArtworkMetadata{}
	err := ar.Connection.QueryRow(`
		SELECT alias, name, title, type, picture_url, description, year, location,
		       artworks.created, added, artworks.updated
		FROM artworks JOIN users ON artworks.author_id = users.id
		WHERE artworks.id = ? AND NOT deleted
		AND ? NOT IN (SELECT target FROM bans WHERE source = artworks.author_id)`,
		artworkId, requesterId).Scan(
		&metadata.AuthorAlias,
		&metadata.AuthorName,
		&metadata.Title,
		&metadata.Type,
		&metadata.PictureUrl,
		&metadata.Description,
		&metadata.Year,
		&metadata.Location,
		&metadata.Created,
		&metadata.Added,
		&metadata.Updated,
	)

	// short exit when essential metadata is missing, or when banned users attempt a read
	if err != nil {
		// no need to unwrap errors
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// now fetch comments; at this point it's known the user isn't banned
	var comments = make([]CommentResponse, 0)
	commentRows, err := ar.Connection.Query(`
		SELECT artwork_comments.id, alias, name, comment, date FROM artwork_comments
		JOIN users ON artwork_comments.user = users.id
		WHERE artwork = ?
		ORDER BY date DESC
		`, artworkId)

	if err != nil {
		return nil, err
	}

	defer closeRows(commentRows)

	for commentRows.Next() {
		var comment CommentResponse
		if err = commentRows.Scan(&comment.Id, &comment.AuthorAlias, &comment.AuthorName,
			&comment.Comment, &comment.Date); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	if err = commentRows.Err(); err != nil {
		return nil, err
	}

	// finally fetch reactions, beware of package clash with reactions array
	var reacts = make([]ReactionResponse, 0)
	reactionsRows, err := ar.Connection.Query(`
		SELECT alias, name, reaction, date FROM artwork_feedback
		JOIN users ON artwork_feedback.user = users.id
		WHERE artwork = ?
		ORDER BY date DESC
		`, artworkId)

	if err != nil {
		return nil, err
	}

	defer closeRows(reactionsRows)

	for reactionsRows.Next() {
		var reaction ReactionResponse
		if err = reactionsRows.Scan(&reaction.AuthorAlias, &reaction.AuthorName,
			&reaction.Reaction, &reaction.Date); err != nil {
			return nil, err
		}
		reacts = append(reacts, reaction)
	}

	// return pointer to new instance, to facilitate non partial error returning
	return &Artwork{
		Metadata:  metadata,
		Comments:  comments,
		Reactions: reacts,
	}, reactionsRows.Err()
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

func (ar *artworkRepository) SetReaction(userId string, artworkId string, date ntime.NTime, data AddReactionRequest) error {

	res, err := ar.Connection.Exec(`
		INSERT INTO artwork_feedback(artwork, user, reaction, date)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (artwork, user) DO UPDATE SET reaction = ?, date = ? WHERE reaction != ?`,
		artworkId, userId, data.Reaction, date, data.Reaction, date, data.Reaction)

	if err != nil {
		return err
	}

	if changed, err := res.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotModified
	}

	return err
}

func (ar *artworkRepository) RemoveReaction(userId string, artworkId string) error {
	res, err := ar.Connection.Exec(`
		DELETE FROM artwork_feedback WHERE artwork = ? AND user = ?`,
		artworkId, userId)

	if err != nil {
		return err
	}

	deleted, err := res.RowsAffected()
	switch {
	case err != nil:
		return err
	case deleted == 0:
		return ErrNotFound
	default:
		return err
	}
}

func (ar *artworkRepository) AddComment(userId string, artworkId string, data AddCommentData) (id string, date ntime.NTime, err error) {

	id = rest.MustGetNewUUID()
	date = ntime.Now()

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
	artworks, total, err := ar.GetUserArtworks(userId, 20, 0)

	// return actual data or last error
	return ProfileData{total, artworks, followers, followed}, err
}

// GetUserArtworks returns a slice of paginated artworks uploaded by the user, along with the total number of uploads.
// tk distinguish between the initial profile artworks and further requests, add pagination struct
func (ar *artworkRepository) GetUserArtworks(userId string, pageSize int, page int) (artworks []ArtworkProfilePreview, tally int, err error) {

	artworks = make([]ArtworkProfilePreview, 0, 12)
	rows, err := ar.Connection.Query(`
		SELECT id, title, picture_url, added, count(id) over() as cnt FROM artworks
		WHERE author_id = ? ORDER BY added DESC LIMIT ? OFFSET ?`,
		userId, pageSize, page,
	)

	if err != nil {
		return artworks, tally, err
	}

	defer closeRows(rows)

	for rows.Next() {
		var artwork ArtworkProfilePreview
		if err = rows.Scan(&artwork.Id, &artwork.Title, &artwork.PictureURL, &artwork.Added, &tally); err != nil {
			return artworks, tally, err
		}
		artworks = append(artworks, artwork)
	}

	return artworks, tally, rows.Err()
}

type StreamData struct {
	Artworks    []ArtworkStreamPreview
	NewArtworks []ArtworkStreamPreview
	DeletedIds  []string
}

func (ar *artworkRepository) GetStream(userId string, since string, latest string) (data StreamData, err error) {

	// default artworks capacity set to default paginated size
	var (
		artworks    = make([]ArtworkStreamPreview, 0, 12)
		newArtworks = make([]ArtworkStreamPreview, 0)
		deletedIds  = make([]string, 0)
	)

	rows, err := ar.Connection.Query(`
		SELECT arts.id, title, alias, name, picture_url, added, new, deleted,
		       coalesce(comments_count, 0) as comments_count,
		       coalesce(feedback_count, 0) as feedback_count
		FROM (SELECT *, added > ? as new FROM artworks
			WHERE author_id IN (SELECT target FROM followers WHERE follower = ?)
			AND added < ? OR added > ? OR (deleted = TRUE AND added > ? AND added < ?)) as arts
		JOIN users ON arts.author_id = users.id
		LEFT JOIN (SELECT artwork, count(id) as comments_count FROM artwork_comments GROUP BY artwork) as comments
		    ON arts.id = comments.artwork
		LEFT JOIN (SELECT artwork, count(*) as feedback_count FROM artwork_feedback GROUP BY artwork) as feedback
		    ON arts.id = feedback.artwork
		ORDER BY added DESC LIMIT 12;`,
		latest, userId, since, latest, latest, since,
	)

	if err != nil {
		return data, err
	}

	defer closeRows(rows)

	var deleted, recent bool
	for rows.Next() {
		var artwork ArtworkStreamPreview
		if err = rows.Scan(&artwork.Id, &artwork.Title, &artwork.AuthorAlias, &artwork.AuthorName,
			&artwork.PictureURL, &artwork.Added, &recent, &deleted, &artwork.Comments, &artwork.Reactions); err != nil {
			return data, err
		}

		switch {
		case recent:
			newArtworks = append(newArtworks, artwork)
		case deleted:
			deletedIds = append(deletedIds, artwork.Id)
		default:
			artworks = append(artworks, artwork)
		}
	}

	return StreamData{
		Artworks:    artworks,
		NewArtworks: newArtworks,
		DeletedIds:  deletedIds,
	}, rows.Err()
}
