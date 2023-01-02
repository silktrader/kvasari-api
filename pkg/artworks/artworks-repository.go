package artworks

import (
	"database/sql"
	"errors"
	"github.com/silktrader/kvasari/pkg/ntime"
	"github.com/silktrader/kvasari/pkg/rest"
	"github.com/silktrader/kvasari/pkg/users"
)

type Storer interface {
	AddArtwork(data AddArtworkData) (ntime.NTime, error)
	DeleteArtwork(artworkId, userId string) bool
	GetArtwork(artworkId, requesterId string) (*Artwork, error)
	GetImageMetadata(artworkId, requesterId string) (ImageMetadata, error)

	AddComment(userId, artworkId string, data AddCommentData) (string, ntime.NTime, error)
	DeleteComment(userId, commentId string) error
	GetArtworkComments(artworkId, requesterId string) ([]CommentResponse, error)

	SetReaction(userId, artworkId string, date ntime.NTime, feedback AddReactionRequest) error
	RemoveReaction(userId, artworkId string) error
	GetArtworkReactions(artworkId, requesterId string) ([]ReactionResponse, error)

	GetUserArtworks(userAlias, requesterId string, pageData PageData) (UserArtworks, error)
	GetStream(userId, since, latest string) (data StreamData, err error)
}

type Store struct {
	Connection *sql.DB
	UserStore  users.UserRepository
}

func NewStore(connection *sql.DB, userStore users.UserRepository) *Store {
	return &Store{connection, userStore}
}

var (
	ErrNotFound    = errors.New("not found")
	ErrNotModified = errors.New("not modified")
)

func closeRows(rows *sql.Rows) {
	_ = rows.Close()
}

func (ar *Store) AddArtwork(data AddArtworkData) (ntime.NTime, error) {

	var now = ntime.Now()

	result, err := ar.Connection.Exec(`
		INSERT INTO artworks(id, type, format, author_id, added, updated)
		VALUES(?, ?, ?, ?, ?, ?)`,
		data.Id, data.Type, data.Format, data.AuthorId, now, now)

	// don't bother checking for unique constraints with UUID generation
	if err != nil {
		return now, err
	}

	// tk check whether needed
	rows, err := result.RowsAffected()
	if err != nil || rows < 1 {
		return now, err
	}

	return now, nil
}

func (ar *Store) GetArtwork(artworkId, requesterId string) (*Artwork, error) {

	// get artwork metadata, ensuring a banned user is denied access
	// with Postgres it'd make sense to update comments and reactions counts by way of a trigger
	// SQLite blocks at every write though, so bursts of comments and reactions (writes) can be problematic
	var artwork = Artwork{}
	err := ar.Connection.QueryRow(`
		SELECT alias, name, title, type, format, description, year, location,
		       artworks.created, added, artworks.updated,
		       (SELECT count(*) FROM artwork_comments WHERE artwork = ?) as comments,
		       (SELECT count(*) FROM artwork_feedback WHERE artwork = ?) as reactions
		FROM artworks JOIN users ON artworks.author_id = users.id
		WHERE artworks.id = ? AND NOT deleted
		AND ? NOT IN (SELECT target FROM bans WHERE source = artworks.author_id)`,
		artworkId, artworkId, artworkId, requesterId).Scan(
		&artwork.AuthorAlias,
		&artwork.AuthorName,
		&artwork.Title,
		&artwork.Type,
		&artwork.Format,
		&artwork.Description,
		&artwork.Year,
		&artwork.Location,
		&artwork.Created,
		&artwork.Added,
		&artwork.Updated,
		&artwork.Comments,
		&artwork.Reactions,
	)

	// no need to unwrap errors actually
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	return &artwork, err
}

func (ar *Store) GetArtworkComments(artworkId, requesterId string) ([]CommentResponse, error) {

	// now fetch comments; at this point it's known the user isn't banned
	var comments = make([]CommentResponse, 0)
	rows, err := ar.Connection.Query(`
		SELECT artwork_comments.id, alias, name, comment, date FROM artwork_comments
		JOIN users ON artwork_comments.user = users.id
		WHERE artwork = ?
		AND ? NOT IN (SELECT target FROM bans WHERE source IN (SELECT author_id FROM artworks WHERE artworks.id = ?))
		ORDER BY date DESC
		`, artworkId, requesterId, artworkId)

	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	for rows.Next() {
		var comment CommentResponse
		if err = rows.Scan(&comment.Id, &comment.AuthorAlias, &comment.AuthorName,
			&comment.Comment, &comment.Date); err != nil {
			return comments, err
		}
		comments = append(comments, comment)
	}

	// always returning a collection, no matter whether the artwork exists or the requester is banned
	return comments, rows.Err()
}

func (ar *Store) GetArtworkReactions(artworkId, requesterId string) ([]ReactionResponse, error) {

	// fetch reactions, beware of package clash with reactions array
	var reactionResponses = make([]ReactionResponse, 0)
	rows, err := ar.Connection.Query(`
		SELECT alias, name, reaction, date FROM artwork_feedback
		JOIN users ON artwork_feedback.user = users.id
		WHERE artwork = ?
		AND ? NOT IN (SELECT target FROM bans WHERE source IN (SELECT author_id FROM artworks WHERE artworks.id = ?))
		ORDER BY date DESC
		`, artworkId, requesterId, artworkId)

	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	// return partial results in case of errors
	for rows.Next() {
		var reaction ReactionResponse
		if err = rows.Scan(&reaction.AuthorAlias, &reaction.AuthorName,
			&reaction.Reaction, &reaction.Date); err != nil {
			return reactionResponses, err
		}
		reactionResponses = append(reactionResponses, reaction)
	}

	return reactionResponses, rows.Err()

}

// OwnsArtwork verifies whether a given artwork exists, wasn't deleted and is owned by the specified user
func (ar *Store) OwnsArtwork(artworkId, userId string) bool {
	var exists = false
	var err = ar.Connection.QueryRow(`
		SELECT TRUE FROM artworks WHERE id = ? AND author_id = ? AND deleted = false`,
		artworkId, userId,
	).Scan(&exists)
	return err != nil && exists
}

// DeleteArtwork will perform a soft delete and return a negative result in case:
//   - the artwork doesn't exist
//   - the artwork isn't owned by the specified user
//   - the artwork was previously deleted
//
// tk handle with errors
func (ar *Store) DeleteArtwork(artworkId, userId string) bool {
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

func (ar *Store) GetImageMetadata(artworkId, requesterId string) (metadata ImageMetadata, err error) {
	return metadata, ar.Connection.QueryRow(`
		SELECT format
		FROM   artworks
		WHERE  id = ?
		       AND deleted = false
		       AND ( author_id = ? OR author_id IN (SELECT target FROM followers WHERE follower = ?));`,
		artworkId, requesterId, requesterId,
	).Scan(&metadata.Format)
}

func (ar *Store) SetReaction(userId, artworkId string, date ntime.NTime, data AddReactionRequest) error {
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

func (ar *Store) RemoveReaction(userId, artworkId string) error {
	res, err := ar.Connection.Exec(`
		DELETE FROM artwork_feedback WHERE artwork = ? AND user = ?`,
		artworkId, userId)

	if err != nil {
		return err
	}

	if deleted, e := res.RowsAffected(); e != nil {
		return e
	} else if deleted == 0 {
		return ErrNotFound
	}

	return nil
}

func (ar *Store) AddComment(userId, artworkId string, data AddCommentData) (string, ntime.NTime, error) {
	var id = rest.MustGetNewUUID()
	var date = ntime.Now()
	_, err := ar.Connection.Exec(`
		INSERT INTO artwork_comments (id, artwork, user, comment, date) VALUES (?, ?, ?, ?, ?)`,
		id, artworkId, userId, data.Comment, date)
	return id, date, err
}

func (ar *Store) DeleteComment(userId, commentId string) error {
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

/*
GetUserArtworks returns paginated artworks uploaded by the target user, in reverse chronological order:

  - artworks added before the 'since' timestamp
  - artworks added after the 'latest' timestamp; those that were uploaded after the latest user request
  - the IDs of artworks deleted before pageData.since but after pageData.latest
*/
func (ar *Store) GetUserArtworks(targetAlias, requesterId string, pageData PageData) (UserArtworks, error) {
	rows, err := ar.Connection.Query(`
		SELECT id, title, format, added, new, deleted, coalesce(c, 0) as comments, coalesce(r, 0) as reactions FROM
			(SELECT id, title, format, added, added > ? as new, deleted FROM artworks
			WHERE author_id IN (SELECT id FROM users WHERE alias = ?)
			AND ? NOT IN (SELECT target FROM bans WHERE source = artworks.author_id)
			AND (deleted = FALSE AND added < ?)
			OR (deleted = FALSE AND added > ?)
			OR (deleted = TRUE AND added > ? AND added < ?)) as x
		LEFT JOIN (SELECT artwork as id, count(artwork) as c FROM artwork_comments GROUP BY artwork) USING (id)
		LEFT JOIN (SELECT artwork as id, count(artwork) as r FROM artwork_feedback GROUP BY artwork) USING (id)
		ORDER BY added DESC LIMIT ?;`,
		pageData.latest,
		targetAlias,
		requesterId,
		pageData.since,
		pageData.latest,
		pageData.latest, pageData.since,
		pageData.pageSize,
	)
	if err != nil {
		return UserArtworks{}, err
	}
	defer closeRows(rows)

	// default artworks capacity set to default paginated size
	var (
		requested         = make([]ArtworkData, 0, 12)
		newArtworks       = make([]ArtworkData, 0)
		deleted           = make([]string, 0)
		isNew, wasDeleted bool
	)

	for rows.Next() {
		var artwork ArtworkData
		if err = rows.Scan(
			&artwork.Id,
			&artwork.Title,
			&artwork.Format,
			&artwork.Added,
			&isNew,
			&wasDeleted,
			&artwork.Comments,
			&artwork.Reactions,
		); err != nil {
			return UserArtworks{}, err
		}
		if isNew {
			newArtworks = append(newArtworks, artwork)
		} else if wasDeleted {
			deleted = append(deleted, artwork.Id)
		} else {
			requested = append(requested, artwork)
		}
	}
	return UserArtworks{requested, newArtworks, deleted}, rows.Err()
}

type StreamData struct {
	Artworks    []ArtworkStreamPreview
	NewArtworks []ArtworkStreamPreview
	DeletedIds  []string
}

func (ar *Store) GetStream(userId string, since string, latest string) (data StreamData, err error) {

	// default artworks capacity set to default paginated size
	var (
		artworks    = make([]ArtworkStreamPreview, 0, 12)
		newArtworks = make([]ArtworkStreamPreview, 0)
		deletedIds  = make([]string, 0)
	)

	rows, err := ar.Connection.Query(`
		SELECT arts.id, title, alias, name, format, added, new, deleted,
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
			&artwork.Format, &artwork.Added, &recent, &deleted, &artwork.Comments, &artwork.Reactions); err != nil {
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
