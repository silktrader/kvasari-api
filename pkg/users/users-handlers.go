package users

import (
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

func RegisterHandlers(engine rest.Engine, ur UserRepository) {
	// doesn't return a handler, as it's already present in the original scope
	engine.Get("/users", getUsers(ur), auth.Auth(ur))
	engine.Post("/users", addUser(ur))

	engine.Put("/profile/name", updateName(ur), auth.Auth(ur))
}

// getUsers fetches all existing users. As neither authorisation nor authentication are required; this is clearly a temporary
// expedient to facilitate development.
func getUsers(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var users, err = ur.GetUsers()
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}
		JSON.Ok(writer, users)
	}
}

func addUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the user data
		var userData AddUserData
		if err := JSON.DecodeValidate(request, &userData); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		newUser, err := ur.AddUser(userData)
		if err != nil {
			JSON.InternalServerError(writer, err)
		}

		JSON.Created(writer, newUser)
	}
}

func updateName(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var userId = auth.GetUserId(request)

		var data UpdateUserNameData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		if err := ur.UpdateUserName(userId, data); err != nil {
			JSON.InternalServerError(writer, err)
		}

		// tk why is this marked as superflous?
		JSON.NoContent(writer)
	}
}
