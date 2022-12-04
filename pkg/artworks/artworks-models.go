package artworks

import (
	"database/sql"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/silktrader/kvasari/pkg/users"
	"net/url"
	"time"
)

type ArtworkType string

const (
	Painting     ArtworkType = "Painting"
	Drawing      ArtworkType = "Drawing"
	Sculpture    ArtworkType = "Sculpture"
	Architecture ArtworkType = "Architecture"
	Photograph   ArtworkType = "Photograph"
)

// tk really odd issue with variadic arguments; can't specify ArtworkType[]
var artworkTypes = []interface{}{Painting, Drawing, Sculpture, Architecture, Photograph}

type Artwork struct {
	ID          string
	Title       string
	Description string
	PictureURL  string
	AuthorID    string
	Location    string
	Year        int
	Type        ArtworkType
	Created     time.Time
	Added       time.Time
	Updated     time.Time
}

type AddArtworkData struct {
	AuthorId    string
	Title       string
	Description string
	PictureURL  string
	Location    string
	Year        int
	Type        ArtworkType
	Created     sql.NullTime
}

func (data AddArtworkData) Validate() error {
	return validation.ValidateStruct(&data,
		validation.Field(&data.Type, validation.Required, validation.In(artworkTypes...)),
		validation.Field(&data.Title, validation.Required),
		validation.Field(&data.PictureURL, validation.Required, is.URL),
		validation.Field(&data.Year, validation.Min(-10000), validation.Max(10000)),
		validation.Field(&data.Created, validation.Date(time.RFC3339)),
	)
}

// Reactions

type ReactionType string

const (
	Like      ReactionType = "Like"
	Perplexed ReactionType = "Perplexed"
)

var reactions = []interface{}{Like, Perplexed}

type ReactionData struct {
	Reaction ReactionType
}

func (data ReactionData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.Reaction,
		validation.Required,
		validation.In(reactions...),
	))
}

// Comments

type CommentData struct {
	Comment string
}

func (data CommentData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.Comment,
		validation.Required,
		validation.Length(10, 3000),
	))
}

// Profile Response DTOs

type ProfileData struct {
	Artworks  []ArtworkProfilePreview
	Followers []users.RelationData
	Followed  []users.RelationData
}

type ArtworkProfilePreview struct {
	ID         string
	Title      string
	PictureURL string // ideally, a server generated preview
	Added      time.Time
}

// I wasted one hour of my life attempting to find out why my custom format wouldn't work
// only to realise time.Parse expects specific numbers for hours, minutes, etc.
var datesRule = validation.Date(time.RFC3339)

func getStreamParams(streamParams url.Values) (since string, latest string, err error) {
	since = streamParams.Get("since")
	if err = validation.Validate(since, validation.Required, datesRule); err != nil {
		return since, latest, err
	}

	latest = streamParams.Get("latest")
	if err = validation.Validate(latest, validation.Required, datesRule); err != nil {
		return since, latest, err
	}

	return since, latest, err
}
