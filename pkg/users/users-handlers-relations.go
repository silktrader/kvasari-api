package users

import (
	"errors"
	"fmt"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/ntime"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

// getFollowers handles the unauthenticated "/users/:alias/followers" route, currently used for debugging purposes.
func getFollowers(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// tk use preemptive validation to cut down DB queries
		var targetAlias = rest.GetParam(request, "alias")

		// check whether the user exists for gracious feedback
		if _, err := ur.GetUserByAlias(targetAlias); err != nil {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s doesn't exist", targetAlias))
			return
		}

		// populate the slice of followers
		if followers, err := ur.GetFollowers(targetAlias); err != nil {
			JSON.InternalServerError(writer, err)
		} else {
			JSON.Ok(writer, followers)
		}
	}
}

// followUser handles the POST "/users/:alias/followed" route
func followUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// ensure that the follower's alias matches the authenticated user's
		var follower = auth.MustGetUser(request)
		if follower.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
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

		var date = ntime.Now()
		// attempt to follow the user and fail when:
		// - the follower already follows the target (ErrDupFollower)
		// - no user matches the target alias (ErrNotFound)
		// - the target is banning the requester (a debatable ErrNotFound)
		if err = ur.Follow(follower.Id, data.TargetAlias, date); err == nil {
			JSON.Created(writer, struct {
				Alias    string
				Followed ntime.NTime
			}{data.TargetAlias, date})
		} else if errors.Is(err, ErrDupFollower) {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("You are already following user %s", data.TargetAlias))
		} else if errors.Is(err, ErrNotFound) {
			JSON.NotFound(writer, fmt.Sprintf("User %s not found", data.TargetAlias))
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// unfollowUser handles the DELETE "/users/:alias/followed/:target" route
func unfollowUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the follower's alias matches the authenticated user's
		var follower = auth.MustGetUser(request)
		if follower.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		// attempt to sanitise target alias before queries
		var targetAlias = rest.GetParam(request, "target")
		if err := ValidateUserAlias(targetAlias); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// short circuit handler when the target and the source match
		if follower.Alias == targetAlias {
			JSON.BadRequestWithMessage(writer, "Narcissistic request: can't unfollow oneself")
			return
		}

		if err := ur.Unfollow(follower.Id, targetAlias); err == nil {
			JSON.NoContent(writer)
		} else if errors.Is(err, ErrNotFound) {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s isn't followed", targetAlias))
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// banUser handles the POST "/users/:alias/bans" route
func banUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the banning user matches the authenticated one
		var source = auth.MustGetUser(request)
		if source.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
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
		var date = ntime.Now()
		if err = ur.Ban(source.Id, data.TargetAlias, date); err == nil {
			JSON.Created(writer, struct {
				Alias  string
				Banned ntime.NTime
			}{data.TargetAlias, date})
		} else if errors.Is(err, ErrDupBan) {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s is already banned", data.TargetAlias))
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// unbanUser handles the DELETE "/users/:alias/bans/:target" route
func unbanUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the user taking action is authorised
		var source = auth.MustGetUser(request)
		if source.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		// attempt to sanitise target alias before queries
		var targetAlias = rest.GetParam(request, "target")
		if err := ValidateUserAlias(targetAlias); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// short circuit handler when the target and the source match
		if source.Alias == targetAlias {
			JSON.BadRequestWithMessage(writer, "Narcissistic request: can't ban oneself")
			return
		}

		if err := ur.Unban(source.Id, targetAlias); err == nil {
			JSON.NoContent(writer)
		} else if errors.Is(err, ErrNotFound) {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s isn't banned", targetAlias))
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// getBans handles the GET "/users/:alias/bans" route
func getBans(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// check whether the user has legitimate access to the route
		var user = auth.MustGetUser(request)
		if user.Alias != rest.GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		if banned, err := ur.GetBans(user.Id); err == nil {
			JSON.Ok(writer, banned)
		} else {
			JSON.InternalServerError(writer, err)
		}

	}
}
