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

	var authenticated = auth.Auth(ar)

	engine.Get("/users", getUsers(ur), authenticated)
	engine.Post("/users", addUser(ur))

	// followers
	engine.Get("/users/:alias/followers", getFollowers(ur))
	engine.Get("/me/followers", getSelfFollowers(ur), authenticated)
	engine.Post("/users/:alias/followed", followUser(ur), authenticated)
	engine.Delete("/users/:alias/followed/:target", unfollowUser(ur), authenticated)

	// bans
	engine.Get("/users/:alias/bans", getBans(ur), authenticated)
	engine.Post("/users/:alias/bans", banUser(ur), authenticated)

	// user details
	engine.Put("/users/:alias/name", updateName(ur), authenticated)
	engine.Put("/users/:alias/alias", updateAlias(ur), authenticated)

	// doesn't return a handler, as it's already present in the original scope
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
		data, err := JSON.DecodeValidate[AddUserData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		newUser, err := ur.Register(data)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Created(writer, newUser)
	}
}

func updateName(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// validate data first
		data, err := JSON.DecodeValidate[UpdateNameData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// authorise or fail
		var user = auth.GetUser(request)
		if user.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		if err = ur.UpdateName(user.Id, data.Name); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.NoContent(writer)
	}
}

func updateAlias(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// validate data first
		data, err := JSON.DecodeValidate[UpdateAliasData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// authorise or fail
		var user = auth.GetUser(request)
		if user.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
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

func followUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the follower's alias matches the authenticated user's
		var follower = auth.GetUser(request)
		if follower.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		// validate target's alias
		data, err := JSON.DecodeValidate[FollowUserData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// short circuit handler when the target and the source match
		if follower.Alias == data.TargetAlias {
			JSON.BadRequestWithMessage(writer, "Narcissistic request: can't follow oneself")
			return
		}

		// attempt to follow the user and fail when:
		// - the follower already follows the target (ErrDupFollower)
		// - no user matches the target alias (ErrNotFound)
		// - the target is banning the requester (a debatable ErrNotFound)
		switch err = ur.Follow(follower.Id, data.TargetAlias); err {
		case nil:
			JSON.NoContent(writer)
		case ErrDupFollower:
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

		// ensure that the follower's alias matches the authenticated user's
		var follower = auth.GetUser(request)
		if follower.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		// attempt to sanitise target alias before queries
		var targetAlias = httprouter.ParamsFromContext(request.Context()).ByName("target")
		if err := ValidateUserAlias(targetAlias); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// short circuit handler when the target and the source match
		if follower.Alias == targetAlias {
			JSON.BadRequestWithMessage(writer, "Narcissistic request: can't unfollow oneself")
			return
		}

		switch err := ur.Unfollow(follower.Id, targetAlias); err {
		case nil:
			JSON.NoContent(writer)
		case ErrNotFound:
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s isn't followed", targetAlias))
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

func banUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the banning user matches the authenticated one
		var source = auth.GetUser(request)
		if source.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		// validate target user alias
		data, err := JSON.DecodeValidate[BanUserData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// avoid self bans
		if source.Alias == data.TargetAlias {
			JSON.BadRequestWithMessage(writer, "Can't ban oneself")
			return
		}

		// attempt to ban, which will also result in targets following the source to stop doing so
		switch err = ur.Ban(source.Id, data.TargetAlias); err {
		case nil:
			JSON.NoContent(writer)
		case ErrDupBan:
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s is already banned", data.TargetAlias))
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

func getBans(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// check whether the user has legitimate access to the route
		var user = auth.GetUser(request)
		if user.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
			JSON.Unauthorised(writer)
			return
		}

		banned, err := ur.GetBans(user.Id)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Ok(writer, banned)
	}
}
