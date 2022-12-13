package artworks

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/silktrader/kvasari/pkg/ntime"
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
	AuthorName  string
	AuthorAlias string
	Title       string
	Description string
	PictureUrl  string
	Location    string
	Year        int
	Type        ArtworkType
	Created     ntime.NTime
	Added       ntime.NTime
	Updated     ntime.NTime
	Comments    int
	Reactions   int
}

type AddArtworkData struct {
	AuthorId    string
	Title       string
	Description string
	PictureURL  string
	Location    string
	Year        int
	Type        ArtworkType
	Created     ntime.NTime
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

type AddReactionRequest struct {
	Reaction ReactionType
}

func (data AddReactionRequest) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.Reaction,
		validation.Required,
		validation.In(reactions...),
	))
}

type ReactionResponse struct {
	AuthorAlias string
	AuthorName  string
	Reaction    ReactionType
	Date        ntime.NTime
}

// Comments

type AddCommentData struct {
	Comment string
}

func (data AddCommentData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.Comment,
		validation.Required,
		validation.Length(10, 3000),
	))
}

type CommentResponse struct {
	Id          string
	AuthorAlias string
	AuthorName  string
	Comment     string
	Date        ntime.NTime
}

// Profile Response DTOs

type ProfileData struct {
	TotalArtworks int
	Artworks      []ArtworkProfilePreview
	Followers     []users.RelationData
	FollowedUsers []users.RelationData
}

type ArtworkProfilePreview struct {
	Id         string
	Title      string
	PictureURL string // ideally, a server generated preview
	Added      ntime.NTime
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

// Stream Responses

type ArtworkStreamPreview struct {
	Id          string
	Title       string
	AuthorAlias string
	AuthorName  string
	PictureURL  string
	Reactions   int
	Comments    int
	Added       ntime.NTime
}
