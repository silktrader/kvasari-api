package users

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"net/http"
)

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
			// tk add 304 response, Not Modified?
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

func unbanUser(ur UserRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the user taking action is authorised
		var source = auth.GetUser(request)
		if source.Alias != httprouter.ParamsFromContext(request.Context()).ByName("alias") {
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
		if source.Alias == targetAlias {
			JSON.BadRequestWithMessage(writer, "Narcissistic request: can't ban oneself")
			return
		}

		switch err := ur.Unban(source.Id, targetAlias); err {
		case nil:
			JSON.NoContent(writer)
		case ErrNotFound:
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("User %s isn't banned", targetAlias))
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

func (ur *userRepository) GetUserRelations(userId string) ([]RelationData, []RelationData, error) {

	var followers, followed = make([]RelationData, 0), make([]RelationData, 0)

	rows, err := ur.Connection.Query(`
		SELECT id, is_follower, alias, name, date
		FROM (
		    SELECT follower as id, TRUE as is_follower, date
		    FROM   followers
		    WHERE  target = ?
		    UNION
		    SELECT target as id, FALSE as is_follower, date
		    FROM   followers
		    WHERE  follower = ?
		) as x
		JOIN users USING (id)
		ORDER BY date DESC`,
		userId,
		userId,
	)

	if err != nil {
		return followers, followed, err
	}

	var isFollower bool
	for rows.Next() {
		var relation RelationData
		if err = rows.Scan(&relation.Id, &isFollower, &relation.Alias, &relation.Name, &relation.Date); err != nil {
			return followers, followed, err
		}

		// append the relation to either followers or followed slices
		if isFollower {
			followers = append(followers, relation)
		} else {
			followed = append(followed, relation)
		}
	}

	if err = rows.Err(); err != nil {
		return followers, followed, err
	}

	if err = rows.Close(); err != nil {
		return followers, followed, err
	}

	return followers, followed, err
}
