package users

import (
	"github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"time"
)

var nameRules = []validation.Rule{validation.Required, validation.Length(5, 50)}
var aliasRules = []validation.Rule{validation.Required, validation.Length(5, 16), is.UTFLetterNumeric}

type User struct {
	Id      string
	Alias   string
	Name    string
	Email   string
	Created time.Time
	Updated time.Time
}

type AddUserData struct {
	Alias    string
	Name     string
	Email    string
	Password string
}

func (data *AddUserData) Validate() error {
	return validation.ValidateStruct(data,
		validation.Field(&data.Name, nameRules...),
		validation.Field(&data.Alias, aliasRules...),
		validation.Field(&data.Email, validation.Required, is.Email),
		validation.Field(&data.Password, validation.Required, validation.Length(8, 50)),
	)
}

type UpdateNameData struct {
	Name string
}

func (data *UpdateNameData) Validate() error {
	return validation.ValidateStruct(data, validation.Field(&data.Name, nameRules...))
}

type UpdateAliasData struct {
	Alias string
}

func (data *UpdateAliasData) Validate() error {
	return validation.ValidateStruct(data, validation.Field(&data.Alias, aliasRules...))
}

func ValidateUserAlias(alias string) error {
	return validation.Validate(alias, aliasRules...)
}

// Bans

type BanUserData struct {
	TargetAlias string
}

func (data *BanUserData) Validate() error {
	return validation.ValidateStruct(data, validation.Field(&data.TargetAlias, aliasRules...))
}

// Followers

type Follower struct {
	ID       string
	Alias    string
	Name     string
	Email    string
	Followed time.Time
}

type FollowUserData struct {
	TargetAlias string
}

func (data *FollowUserData) Validate() error {
	return validation.ValidateStruct(data, validation.Field(&data.TargetAlias, aliasRules...))
}
