package users

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

func RegisterHandlers(engine rest.Engine, ur UserRepository, ar auth.Repository) {
	// doesn't return a handler, as it's already present in the original scope
	engine.Get("/users", getUsers(ur), auth.Auth(ar))
	engine.Post("/users", addUser(ur))

	// followers
	engine.Get("/users/:alias/followers", getFollowers(ur)) // unauthorised
	engine.Get("/me/followers", getSelfFollowers(ur), auth.Auth(ar))

	engine.Post("/users/:alias/followed", followUser(ur), auth.Auth(ar))
	engine.Delete("/users/:target/followers/:follower", unfollowUser(ur), auth.Auth(ar))

	// bans
	engine.Post("/users/:alias/bans", banUser(ur), auth.Auth(ar))

	// user details
	engine.Put("/profile/name", updateName(ur), auth.Auth(ar))
	engine.Put("/profile/alias", updateAlias(ur), auth.Auth(ar))
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

		followers, err := ur.GetFollowersById(auth.GetUser(request).Id)
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

		// validate data first
		var data UpdateNameData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// then attempt to perform the operation
		var user = auth.GetUser(request)

		if err := ur.UpdateName(user.Id, data.Name); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.NoContent(writer)
	}
}

func updateAlias(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var authUser = auth.GetUser(request)

		var data UpdateAliasData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// the database ensures uniqueness of aliases, but a specific error would be useful for the frontend
		existingUser, err := ur.GetUserByAlias(data.Alias)

		// authUser alias not found, proceed with the change
		if err != nil {
			err = ur.UpdateAlias(authUser.Alias, data.Alias)
			if err != nil {
				JSON.InternalServerError(writer, err)
				return
			}
			JSON.NoContent(writer)
			return
		}

		// the authUser is attempting to change his own alias to its old alias
		if existingUser.Id == authUser.Id {
			JSON.BadRequestWithMessage(writer, "New and old aliases coincide")
			return
		}

		JSON.BadRequestWithMessage(writer, "Alias already taken")
	}
}

func followUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the follower's alias matches the authenticated user's
		var follower = auth.GetUser(request)
		if follower.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		// validate target's alias
		var data FollowUserData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// short circuit handler when the target and the source match
		if follower.Alias == data.TargetAlias {
			JSON.BadRequestWithMessage(writer, "Narcissistic request: can't follow oneself")
			return
		}

		// attempt to follow the user and fail when:
		// - the follower already follows the target (ErrFollowDuplicate)
		// - no user matches the target alias (ErrNotFound)
		// - the target is banning the requester (a debatable ErrNotFound)
		switch err := ur.FollowAlias(follower.Id, data.TargetAlias); err {
		case nil:
			JSON.NoContent(writer)
		case ErrFollowDuplicate:
			JSON.BadRequestWithMessage(writer, err.Error())
		case ErrNotFound:
			JSON.NotFound(writer, err.Error())
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

func unfollowUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var followerId = auth.GetUser(request).Id
		var targetAlias = httprouter.ParamsFromContext(request.Context()).ByName("target")

		// tk attempt to sanitise input
		// check whether alias exists and return its id
		targetUser, err := ur.GetUserByAlias(targetAlias)
		if err != nil {
			JSON.NotFound(writer, fmt.Sprintf("User %s not found", targetAlias))
			return
		}

		// the (debatable) alternative to making an extra round trip to the DB is to rely on a rows count when deleting
		unfollowed, err := ur.Unfollow(followerId, targetUser.Id)

		if unfollowed {
			JSON.NoContent(writer)
		} else if err != nil {
			JSON.InternalServerError(writer, err)
		} else {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("You can't unfollow %s", targetAlias))
		}
	}
}

func banUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var source = auth.GetUser(request)

		// ensure that the banning user matches the authenticated one
		if source.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		// validate target user alias
		var data BanUserData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// avoid self bans
		if source.Alias == data.TargetAlias {
			JSON.BadRequestWithMessage(writer, "Can't ban oneself")
			return
		}

		// ensure the target exists to provide specific errors
		targetUser, err := ur.GetUserByAlias(data.TargetAlias)
		if err != nil {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s doesn't exist", data.TargetAlias))
			return
		}

		// attempt to ban, which will result in targets following the source to stop doing so
		banned, err := ur.Ban(source.Id, targetUser.Id)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		if banned {
			JSON.NoContent(writer)
		} else {
			JSON.BadRequest(writer)
		}

	}
}
