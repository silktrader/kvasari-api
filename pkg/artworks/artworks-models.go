package artworks

import (
	"errors"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
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

func (t ArtworkType) IsValid() bool {
	// would be preferable to have an automagically populated slice of values and check for inclusion
	switch t {
	case Painting, Drawing, Sculpture, Architecture, Photograph:
		return true
	}
	return false
}

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
	Title       string
	Description string
	PictureURL  string
	Location    string
	Year        int
	Type        ArtworkType
	Created     time.Time
}

func (data AddArtworkData) Validate() error {
	if !data.Type.IsValid() {
		return errors.New("invalid artwork type")
	}
	// tk test validation.In() to check "enums"
	return validation.ValidateStruct(data,
		validation.Field(&data.Title, validation.Required),
		validation.Field(&data.PictureURL, is.URL),
		validation.Field(&data.Year, validation.Min(-10000), validation.Max(10000)),
		validation.Field(&data.Created, validation.Date("2000-12-25")),
	)
}
