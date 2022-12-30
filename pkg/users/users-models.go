package users

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/silktrader/kvasari/pkg/ntime"
	"net/url"
	"time"
)

var nameRules = []validation.Rule{validation.Required, validation.Length(5, 50)}
var aliasRules = []validation.Rule{validation.Required, validation.Length(5, 16), is.UTFLetterNumeric}
var passwordRules = []validation.Rule{validation.Required, validation.Length(8, 50)}

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

func ValidateUserAlias(alias string) error {
	return validation.Validate(alias, aliasRules...)
}

// filtered users GET query parameters validation

// getStreamParams returns the values of query parameters `since` and `latest`, after validating them
func getFilteredUsersParams(params url.Values) (filter string, requesterAlias string, err error) {
	// there's no need to check for both parameters when one fails
	filter = params.Get("filter")
	if err = validation.Validate(filter, nameRules...); err != nil {
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
