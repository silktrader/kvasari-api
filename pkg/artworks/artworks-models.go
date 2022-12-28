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

type ImageFormat string

const (
	PNG  ImageFormat = "png"
	JPG  ImageFormat = "jpg"
	WEBP ImageFormat = "webp"
)

// tk really odd issue with variadic arguments; can't specify ArtworkType[]
var artworkTypes = []interface{}{Painting, Drawing, Sculpture, Architecture, Photograph}

// var imageFormats = []interface{}{PNG, JPG, WEBP}

type Artwork struct {
	AuthorName  string
	AuthorAlias string
	Title       *string
	Description *string
	Format      string
	Location    *string
	Year        *int
	Type        ArtworkType
	Created     ntime.NTime
	Added       ntime.NTime
	Updated     ntime.NTime
	Comments    int
	Reactions   int
}

type AddArtworkData struct {
	Id       string
	AuthorId string
	Format   ImageFormat
	Type     ArtworkType
}

// tk check whether this is actually needed
func (data AddArtworkData) Validate() error {
	return validation.ValidateStruct(&data,
		validation.Field(&data.Type, validation.Required, validation.In(artworkTypes...)),
		validation.Field(&data.Format, validation.Required, is.URL),
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
	// TotalArtworks refers to the total number of artworks uploaded by a user, as opposed to the ones sent in the resp.
	TotalArtworks int
	Artworks      []ArtworkProfilePreview
	Followers     []users.RelationData
	FollowedUsers []users.RelationData
}

type ArtworkProfilePreview struct {
	Id     string
	Title  *string // the alternative is to use sql.NullString and a custom marshaller
	Format string
	Added  ntime.NTime
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
	Format      string
	Reactions   int
	Comments    int
	Added       ntime.NTime
}

// Images metadata, to be expanded

type ImageMetadata struct {
	Format string
}
