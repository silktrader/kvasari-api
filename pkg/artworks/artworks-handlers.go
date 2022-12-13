package artworks

import (
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/ntime"
	. "github.com/silktrader/kvasari/pkg/rest"
	"net/http"
)

func RegisterHandlers(engine Engine, ar ArtworkRepository, aur auth.Repository) {

	var authenticated = auth.Auth(aur)

	// artworks management
	engine.Post("/artworks", addArtwork(ar), authenticated)
	engine.Delete("/artworks/:artworkId", deleteArtwork(ar), authenticated)
	engine.Get("/artworks/:artworkId", getArtwork(ar), authenticated)

	// feedback
	engine.Post("/artworks/:artworkId/comments", addComment(ar), authenticated)
	engine.Delete("/artworks/:artworkId/comments/:commentId", deleteComment(ar), authenticated)
	engine.Put("/artworks/:artworkId/reactions/:alias", setReaction(ar), authenticated)
	engine.Delete("/artworks/:artworkId/reactions/:alias", removeReaction(ar), authenticated)

	// user specific aggregates
	engine.Get("/users/:alias/profile", getProfile(ar), authenticated)
	engine.Get("/users/:alias/stream", getStream(ar), authenticated)
}

// addArtwork handles the authenticated POST "/artworks" route
func addArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the artwork data
		data, err := JSON.DecodeValidate[AddArtworkData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// ensure that the author's ID matches the authenticated user's one
		if auth.MustGetUser(request).Id != data.AuthorId {
			JSON.Forbidden(writer)
			return
		}

		// return a JSON with the artwork's ID and time of creation
		id, updated, err := ar.AddArtwork(data)
		switch err {
		case nil:
			JSON.Created(writer, struct {
				Id      string
				Updated ntime.NTime
			}{
				Id:      id,
				Updated: updated,
			})
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// deleteArtwork handles the authenticated DELETE "/artworks/:artworkId" route
func deleteArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// issues a bad request regardless of authorisation issues to deny information about existing resources
		if deleted := ar.DeleteArtwork(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); deleted {
			JSON.NoContent(writer)
		} else {
			JSON.BadRequest(writer)
		}
	}
}

// getArtwork handles the authenticated GET "/artworks/:artworkId" route and provides an artwork's metadata
func getArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		switch response, err := ar.GetArtwork(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); err {
		case ErrNotFound:
			JSON.NotFound(writer, "Artwork not found")
		case nil:
			JSON.Ok(writer, response)
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// setReaction handles the authenticated PUT "/artworks/:artworkId/reactions/:alias" route
func setReaction(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// the path user must match the authorised one
		var user = auth.MustGetUser(request)
		if user.Alias != GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		// validate
		data, err := JSON.DecodeValidate[AddReactionRequest](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		var date = ntime.Now()
		err = ar.SetReaction(user.Id, GetParam(request, "artworkId"), date, data)

		// it's debatable whether 201 should be returned on first setting the reaction
		switch err {
		case nil:
			JSON.Ok(writer, struct {
				Status string
				Date   ntime.NTime
			}{"changed", date})
		case ErrNotModified:
			JSON.Ok(writer, struct{ Status string }{"unchanged"})
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// removeReaction handles the DELETE "/artworks/:artworkId/reactions/:alias" route
func removeReaction(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// the alias must match the authorised one
		var user = auth.MustGetUser(request)
		if user.Alias != GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		switch err := ar.RemoveReaction(user.Id, GetParam(request, "artworkId")); err {
		case nil:
			JSON.NoContent(writer)
		case ErrNotFound:
			JSON.NotFound(writer, "Reaction not found, or unauthorised action")
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// addComment handles the POST "/artworks/:artworkId/comments route
func addComment(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		data, err := JSON.DecodeValidate[AddCommentData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		var artworkId = GetParam(request, "artworkId")
		id, date, err := ar.AddComment(auth.MustGetUser(request).Id, artworkId, data)

		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Created(writer, struct {
			Id   string
			Date ntime.NTime
		}{
			Id:   id,
			Date: date,
		})
	}
}

// deleteComment handles the authenticated DELETE "/artworks/:artworkId/comments/:commentId" route
func deleteComment(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		switch err := ar.DeleteComment(auth.MustGetUser(request).Id, GetParam(request, "commentId")); err {
		case nil:
			JSON.NoContent(writer)
		case ErrNotFound:
			JSON.NotFound(writer, "Comment not found, or unauthorised action")
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// getProfile handles the authenticated GET "/users/:alias/profile" route
func getProfile(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// check whether the user has legitimate access to the route
		var user = auth.MustGetUser(request)
		if user.Alias != GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		profile, err := ar.GetProfileData(user.Id)
		if err != nil {
			JSON.InternalServerError(writer, err) // tk disambiguate
			return
		}

		JSON.Ok(writer, profile)
	}
}

// getStream handles the authenticated GET "/users/:alias/stream?since=date&latest=date" route
func getStream(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// check whether the user has legitimate access to the route
		var user = auth.MustGetUser(request)
		if user.Alias != GetParam(request, "alias") {
			JSON.Forbidden(writer)
			return
		}

		// get and validate the two required parameters from the URL query
		var since, latest, err = getStreamParams(request.URL.Query())
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		stream, err := ar.GetStream(user.Id, since, latest)
		if err != nil {
			JSON.InternalServerError(writer, err) // tk handle
			return
		}

		JSON.Ok(writer, stream)
	}
}
