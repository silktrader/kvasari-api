package users

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

func RegisterHandlers(engine rest.Engine, ur UserRepository) {
	// doesn't return a handler, as it's already present in the original scope
	engine.Get("/users", getUsers(ur), auth.Auth(ur))
	engine.Post("/users", addUser(ur))

	// followers
	engine.Get("/users/:alias/followers", getFollowers(ur)) // unauthorised
	engine.Get("/me/followers", getSelfFollowers(ur), auth.Auth(ur))
	
	engine.Post("/users/:alias/followers", followUser(ur), auth.Auth(ur))
	engine.Delete("/users/:target/followers/:follower", unfollowUser(ur), auth.Auth(ur))

	// user details
	engine.Put("/profile/name", updateName(ur), auth.Auth(ur))
	engine.Put("/profile/alias", updateAlias(ur), auth.Auth(ur))
}

// getUsers fetches all existing users. As neither authorisation nor authentication are required; this is clearly a temporary
// expedient to facilitate development.
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

func getFollowers(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var targetAlias = httprouter.ParamsFromContext(request.Context()).ByName("alias")

		// check whether the user exists for gracious feedback
		var targetExists = ur.ExistsUserAlias(targetAlias)
		if !targetExists {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s doesn't exist", targetAlias))
			return
		}

		// populate the slice of followers
		followers, err := ur.GetFollowers(targetAlias)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Ok(writer, followers)
	}
}

func getSelfFollowers(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		followers, err := ur.GetFollowersById(auth.GetUserId(request))
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Ok(writer, followers)
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

		newUser, err := ur.Register(userData)
		if err != nil {
			JSON.InternalServerError(writer, err)
		}

		JSON.Created(writer, newUser)
	}
}

func updateName(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var userId = auth.GetUserId(request)

		var data UpdateNameData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		if err := ur.UpdateName(userId, data.Name); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.NoContent(writer)
	}
}

func updateAlias(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var userId = auth.GetUserId(request)

		var data UpdateAliasData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// the database ensures uniqueness of aliases, but a specific error would be useful for the frontend
		existingUser, err := ur.GetUserByAlias(data.Alias)

		// user alias not found, proceed with the change
		if err != nil {
			err = ur.UpdateAlias(userId, data.Alias)
			if err != nil {
				JSON.InternalServerError(writer, err)
				return
			}
			JSON.NoContent(writer)
			return
		}

		// the user is attempting to change his own alias to its old alias
		if existingUser.ID == userId {
			JSON.BadRequestWithMessage(writer, "New and old aliases coincide")
			return
		}

		JSON.BadRequestWithMessage(writer, "Alias already taken")
	}
}

func followUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var followerId = auth.GetUserId(request)
		var targetAlias = httprouter.ParamsFromContext(request.Context()).ByName("alias")

		// tk attempt to sanitise input

		// check whether alias exists and return its id
		targetUser, err := ur.GetUserByAlias(targetAlias)
		if err != nil {
			JSON.NotFound(writer, fmt.Sprintf("User %s not found", targetAlias))
			return
		}

		if targetUser.ID == followerId {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("Narcissistic request: You can't follow yourself"))
			return
		}

		// check whether the user already follows the target to disambiguate between errors
		// requires one more trip to the database
		// tk CHECK WHETHER USER BANS OTHER USER
		if ur.IsFollowing(followerId, targetUser.ID) {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("You already follow %s", targetAlias))
			return
		}

		// attempt to follow the user
		err = ur.Follow(followerId, targetUser.ID)
		if err != nil {
			JSON.InternalServerError(writer, fmt.Errorf("error while following %s: %w", targetAlias, err))
			return
		}

		JSON.NoContent(writer)
	}
}

func unfollowUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var followerId = auth.GetUserId(request)
		var targetAlias = httprouter.ParamsFromContext(request.Context()).ByName("target")

		// tk attempt to sanitise input
		// check whether alias exists and return its id
		targetUser, err := ur.GetUserByAlias(targetAlias)
		if err != nil {
			JSON.NotFound(writer, fmt.Sprintf("User %s not found", targetAlias))
			return
		}

		// the (debatable) alternative to making an extra round trip to the DB is to rely on a rows count when deleting
		unfollowed, err := ur.Unfollow(followerId, targetUser.ID)

		if unfollowed {
			JSON.NoContent(writer)
		} else if err != nil {
			JSON.InternalServerError(writer, err)
		} else {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("You can't unfollow %s", targetAlias))
		}
	}
}
