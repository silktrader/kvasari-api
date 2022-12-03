package users

import (
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

func RegisterHandlers(engine rest.Engine, ur UserRepository, ar auth.Repository) {

	var authenticated = auth.Auth(ar)

	engine.Post("/sessions", login(ur))
	engine.Get("/users", getUsers(ur))
	engine.Post("/users", registerUser(ur))

	// followers
	engine.Get("/users/:alias/followers", getFollowers(ur))
	engine.Post("/users/:alias/followed", followUser(ur), authenticated)
	engine.Delete("/users/:alias/followed/:target", unfollowUser(ur), authenticated)

	// bans
	engine.Get("/users/:alias/bans", getBans(ur), authenticated)
	engine.Post("/users/:alias/bans", banUser(ur), authenticated)
	engine.Delete("/users/:alias/bans/:target", unbanUser(ur), authenticated)

	// user details
	engine.Put("/users/:alias/name", updateName(ur), authenticated)
	engine.Put("/users/:alias/alias", updateAlias(ur), authenticated)

	// doesn't return a handler, as it's already present in the original scope
}

// getUsers handles the GET "/users" route and fetches all existing users.
// Neither authorisation nor authentication are required for development purposes.
func getUsers(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var users, err = ur.GetAll()
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}
		JSON.Ok(writer, users)
	}
}

// registerUser handles the POST "/users" route
func registerUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the user data
		data, err := JSON.DecodeValidate[AddUserData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		newUser, err := ur.Register(data)
		switch err {
		case nil:
			JSON.Created(writer, newUser)
		case ErrDupUser:
			JSON.BadRequestWithMessage(writer, "Email or alias already registered")
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// updateName handles the PUT "/users/:alias/name" route, allowing users to change their full name
func updateName(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// validate data first
		data, err := JSON.DecodeValidate[UpdateNameData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// can't change others names
		var user = auth.MustGetUser(request)
		if user.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		if err = ur.UpdateName(user.Id, data.Name); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.NoContent(writer)
	}
}

// updateAlias handles the PUT "/users/:alias/alias" route, allowing users to change aliases provided they're unique
func updateAlias(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// validate data first
		data, err := JSON.DecodeValidate[UpdateAliasData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// check authorisation
		var user = auth.MustGetUser(request)
		if user.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		err = ur.UpdateAlias(user.Id, data.Alias)
		switch err {
		case nil:
			JSON.NoContent(writer)
		case ErrAliasTaken:
			JSON.BadRequestWithMessage(writer, "Alias already taken")
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// login performs a simplistic authentication attempt, returning the user ID and status on success.
// No passwords are checked, only mere user existence.
func login(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		const message = "Authentication failed due to wrong credentials."
		sessionData, err := JSON.DecodeValidate[SessionData](request)
		if err != nil {
			// debatable status code choice; 401 is inappropriate without HTTP auth and 403 misses the point
			JSON.BadRequestWithMessage(writer, message)
			return
		}

		// a mere user existence check
		user, err := ur.GetUserByAlias(sessionData.Alias)
		if err != nil {
			JSON.BadRequestWithMessage(writer, message)
			return
		}

		// one would set refresh and access tokens in the response but for the moment a status suffices
		JSON.Ok(writer, struct {
			Id     string
			Status string
		}{
			Id:     user.Id,
			Status: "authenticated",
		})
	}
}
