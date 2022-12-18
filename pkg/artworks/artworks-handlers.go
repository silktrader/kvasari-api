package artworks

import (
	"errors"
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

	// comments
	engine.Post("/artworks/:artworkId/comments", addComment(ar), authenticated)
	engine.Delete("/artworks/:artworkId/comments/:commentId", deleteComment(ar), authenticated)
	engine.Get("/artworks/:artworkId/comments", getArtworkComments(ar), authenticated)

	// reactions
	engine.Put("/artworks/:artworkId/reactions/:alias", setReaction(ar), authenticated)
	engine.Delete("/artworks/:artworkId/reactions/:alias", removeReaction(ar), authenticated)
	engine.Get("/artworks/:artworkId/reactions", getArtworkReactions(ar), authenticated)

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

		switch response, err := ar.GetArtwork(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); {
		case errors.Is(err, ErrNotFound):
			JSON.NotFound(writer, "Artwork not found")
		case err == nil:
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

		// it's debatable whether 201 should be returned on first setting the reaction
		if err = ar.SetReaction(user.Id, GetParam(request, "artworkId"), date, data); err == nil {
			JSON.Ok(writer, struct {
				Status string
				Date   ntime.NTime
			}{"changed", date})
		} else if errors.Is(err, ErrNotModified) {
			JSON.Ok(writer, struct{ Status string }{"unchanged"})
		} else {
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

		if err := ar.RemoveReaction(user.Id, GetParam(request, "artworkId")); err == nil {
			JSON.NoContent(writer)
		} else if errors.Is(err, ErrNotFound) {
			JSON.NotFound(writer, "Reaction not found, or unauthorised action")
		} else {
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

		if err := ar.DeleteComment(auth.MustGetUser(request).Id, GetParam(request, "commentId")); err == nil {
			JSON.NoContent(writer)
		} else if errors.Is(err, ErrNotFound) {
			JSON.NotFound(writer, "Comment not found, or unauthorised action")
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// getArtworkComments handles the authenticated GET "/artworks/:artworkId/comments" route
func getArtworkComments(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		if comments, err := ar.GetArtworkComments(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); err == nil {
			JSON.Ok(writer, comments)
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

func getArtworkReactions(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		if reacts, err := ar.GetArtworkReactions(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); err == nil {
			JSON.Ok(writer, reacts)
		} else {
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

		if profile, err := ar.GetProfileData(user.Id); err == nil {
			JSON.Ok(writer, profile)
		} else {
			JSON.InternalServerError(writer, err) // could disambiguate errors given the elaborate query
		}
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
