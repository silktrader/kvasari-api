package users

import (
	"errors"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

func RegisterHandlers(engine rest.Engine, ur UserRepository, ar auth.IRepository) {

	var authenticated = auth.Auth(ar)

	engine.Post("/sessions", login(ur))
	engine.Get("/users", getUsers(ur), authenticated)
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
	engine.Get("/users/:alias", getDetails(ur), authenticated)
	engine.Put("/users/:alias/name", updateName(ur), authenticated)
	engine.Put("/users/:alias/alias", updateAlias(ur), authenticated)

	// doesn't return a handler, as it's already present in the original scope
}

// getUsers handles the GET "/users" route and fetches all the existing users, matching a name or alias pattern,
// that the requester has access to.
func getUsers(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// ensure the requester alias matches the request's credentials
		var filter, requesterAlias, err = getFilteredUsersParams(request.URL.Query())
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		var user = auth.MustGetUser(request)
		if requesterAlias != user.Alias {
			JSON.Forbidden(writer)
			return
		}

		if users, e := ur.GetFilteredUsers(filter, user.Id); e != nil {
			JSON.InternalServerError(writer, e)
		} else {
			JSON.Ok(writer, users)
		}
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

		if newUser, e := ur.Register(data); e == nil {
			JSON.Created(writer, newUser)
		} else if errors.Is(e, ErrDupUser) {
			JSON.BadRequestWithMessage(writer, "Email or alias already registered")
		} else {
			JSON.InternalServerError(writer, e)
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

		if err = ur.UpdateName(user.Id, data.Name); err == nil {
			JSON.NoContent(writer)
		} else {
			JSON.InternalServerError(writer, err)
		}
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

		if err = ur.UpdateAlias(user.Id, data.Alias); err == nil {
			JSON.NoContent(writer)
		} else if errors.Is(err, ErrAliasTaken) {
			JSON.BadRequestWithMessage(writer, "Alias already taken")
		} else {
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
		// return basic user details for the front-end's consumption
		JSON.Created(writer, struct {
			Id     string
			Name   string
			Alias  string
			Status string
		}{
			Id:     user.Id,
			Name:   user.Name,
			Alias:  user.Alias,
			Status: "authenticated",
		})
	}
}

// getDetails handles the GET "/users/:alias" route and returns basic user details, including a mere count of
// uploaded artworks, followers and following
func getDetails(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// validate the alias
		var alias, err = getValidateAlias(request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// fetch user details or return a not found message, even in case of bans
		if details, e := ur.GetDetails(alias, auth.MustGetUser(request).Id); e == nil {
			JSON.Ok(writer, details)
		} else if errors.Is(e, ErrNotFound) {
			JSON.NotFound(writer, "user not found or unavailable")
		} else {
			JSON.InternalServerError(writer, e)
		}
	}
}
