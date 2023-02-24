package users

import (
	"errors"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/julienschmidt/httprouter"
	"github.com/silktrader/kvasari/pkg/ntime"
	"net/http"
	"net/url"
	"time"
)

const (
	maxNameLength  = 50
	maxAliasLength = 16
)

// validation rules reused throughout the package
var (
	nameRules     = []validation.Rule{validation.Required, validation.Length(5, maxNameLength)}
	aliasRules    = []validation.Rule{validation.Required, validation.Length(5, maxAliasLength), is.UTFLetterNumeric}
	passwordRules = []validation.Rule{validation.Required, validation.Length(8, 50)}
)

var ErrMissingAlias = errors.New("missing `alias` parameter")

type User struct {
	Id      string
	Alias   string
	Name    string
	Email   string
	Created ntime.NTime
	Updated ntime.NTime
}

// SessionData provides the data to log in
type SessionData struct {
	Alias    string
	Password string
}

func (data SessionData) Validate() error {
	return validation.ValidateStruct(&data,
		validation.Field(&data.Alias, aliasRules...),
		validation.Field(&data.Password, passwordRules...))
}

type AddUserData struct {
	Alias    string
	Name     string
	Email    string
	Password string
}

func (data AddUserData) Validate() error {
	return validation.ValidateStruct(&data,
		validation.Field(&data.Name, nameRules...),
		validation.Field(&data.Alias, aliasRules...),
		validation.Field(&data.Password, passwordRules...),
		validation.Field(&data.Email, validation.Required, is.Email),
	)
}

type UpdateNameData struct {
	Name string
}

func (data UpdateNameData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.Name, nameRules...))
}

type UpdateAliasData struct {
	Alias string
}

func (data UpdateAliasData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.Alias, aliasRules...))
}

// ValidateUserAlias takes a candidate user alias and validates it against a set of text patterns.
func ValidateUserAlias(alias string) error {
	return validation.Validate(alias, aliasRules...)
}

// getValidateAlias verifies the `alias` request parameter before proceeding with queries;
// possibly returns a missing alias error, or a validation one
func getValidateAlias(request *http.Request) (alias string, err error) {
	if alias = httprouter.ParamsFromContext(request.Context()).ByName("alias"); alias == "" {
		return alias, ErrMissingAlias
	}
	return alias, ValidateUserAlias(alias)
}

// UserDetails describes data returned by the getDetails() handler and repository method.
// Note that Comments and Reactions refer to feedback received, rather than emitted.
type UserDetails struct {
	Name           string
	Email          string
	Followers      int
	Following      int
	ArtworksAdded  int
	Comments       int
	Reactions      int
	FollowedByUser bool
	FollowsUser    bool
	BlockedByUser  bool
	Created        ntime.NTime
	Updated        ntime.NTime
}

// filtered users GET query parameters validation

// getStreamParams returns the values of query parameters `filter` and `requesterAlias`, after validating them
func getFilteredUsersParams(params url.Values) (filter string, requesterAlias string, err error) {
	// there's no need to check for both parameters when one fails
	filter = params.Get("filter")
	if err = validation.Validate(filter, validation.Required, validation.Length(3, maxNameLength)); err != nil {
		return filter, requesterAlias, err
	}

	requesterAlias = params.Get("requester")
	if err = validation.Validate(requesterAlias, aliasRules...); err != nil {
		return filter, requesterAlias, err
	}

	return filter, requesterAlias, err
}

// Bans

type BannedUser struct {
	Id     string
	Alias  string
	Name   string
	Banned ntime.NTime
}

type BanUserData struct {
	TargetAlias string
}

func (data BanUserData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.TargetAlias, aliasRules...))
}

// Followers

type Follower struct {
	Id       string
	Alias    string
	Name     string
	Email    string
	Followed ntime.NTime
}

type FollowUserData struct {
	TargetAlias string
}

func (data FollowUserData) Validate() error {
	return validation.ValidateStruct(&data, validation.Field(&data.TargetAlias, aliasRules...))
}

type RelationData struct {
	Id    string // debatable inclusion
	Alias string
	Name  string
	Date  time.Time
}
