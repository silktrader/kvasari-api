package artworks

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/silktrader/kvasari/pkg/ntime"
	"github.com/silktrader/kvasari/pkg/users"
	"net/url"
	"regexp"
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

// Artwork describes all the publicly available metadata relevant to an artwork.
type Artwork struct {
	Author      ArtworkAuthor
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

// ArtworkAuthor holds data relevant for artwork data responses.
type ArtworkAuthor struct {
	Alias          string
	Name           string
	FollowsUser    bool
	FollowedByUser bool
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

// Edit and artwork title

type UpdateArtworkTitleData struct {
	Title string
}

func (data UpdateArtworkTitleData) Validate() error {
	return validation.ValidateStruct(&data,
		validation.Field(&data.Title, validation.Required, validation.Length(0, 150)),
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
	Artworks      []ArtworkData
	Followers     []users.RelationData
	FollowedUsers []users.RelationData
}

/*
UserArtworks contains:
  - requested artworks, matching the provided timestamps and target user
  - new artworks uploaded after the user's last request
  - the IDs of artworks deleted since the last request
*/
type UserArtworks struct {
	Requested []ArtworkData
	New       []ArtworkData
	Deleted   []string
}

// ArtworkData describes a preview of the metadata related to each artwork, including comment, reactions aggregates.
type ArtworkData struct {
	Id        string
	Title     *string // the alternative is to use sql.NullString and a custom marshaller
	Format    string
	Added     ntime.NTime
	Comments  int
	Reactions int
}

// PageData specifies pagination details for various endpoint handlers and store methods
type PageData struct {
	pageSize int
	since    string
	latest   string
}

// I wasted one hour of my life attempting to find out why my custom format wouldn't work
// only to realise time.Parse expects specific numbers for hours, minutes, etc.
var datesRules = []validation.Rule{validation.Required, validation.Date(time.RFC3339)}

func ValidateDate(date string) error {
	return validation.Validate(date, datesRules...)
}

// Stream Responses

type ArtworkStreamPreview struct {
	Id          string
	Title       *string
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

/* Route parameters validation.
The following functions ensure the correct format of route parameters, and catch possible errors without
having to resort to DB queries.*/

// getStreamParams returns the values of query parameters `since` and `latest`, after validating them.
// tk replace
func getStreamParams(streamParams url.Values) (since string, latest string, err error) {
	// there's no need to check for both parameters when one fails
	since = streamParams.Get("since")
	if err = validation.Validate(since, datesRules...); err != nil {
		return since, latest, err
	}

	latest = streamParams.Get("latest")
	if err = validation.Validate(latest, datesRules...); err != nil {
		return since, latest, err
	}

	return since, latest, err
}

// validateArtworkIdParam verified that the provided ID it's a valid SHA-256 hash.
func isValidArtworkId(artworkId string) bool {
	match, err := regexp.MatchString("^[a-f0-9]{64}$", artworkId)
	return err == nil && match
}
