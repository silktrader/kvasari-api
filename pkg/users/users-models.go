package users

import (
	"github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"time"
)

var nameRules = []validation.Rule{validation.Required, validation.Length(5, 50)}

type User struct {
	ID      string
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
		validation.Field(&data.Alias, validation.Required, validation.Length(5, 16), is.UTFLetterNumeric),
		validation.Field(&data.Name, nameRules...),
		validation.Field(&data.Email, validation.Required, is.Email),
		validation.Field(&data.Password, validation.Required, validation.Length(8, 50)),
	)
}

type UpdateUserNameData struct {
	Name string
}

func (data *UpdateUserNameData) Validate() error {
	return validation.ValidateStruct(data, validation.Field(&data.Name, nameRules...))
}
