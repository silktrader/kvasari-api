package artworks

import (
	"github.com/julienschmidt/httprouter"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"net/http"
	"time"
)

func RegisterHandlers(engine rest.Engine, ar ArtworkRepository, aur auth.Repository) {

	var authenticated = auth.Auth(aur)

	engine.Post("/users/:alias/artworks", addArtwork(ar), authenticated) // tk review path
	engine.Get("/users/:alias/profile", getProfile(ar), authenticated)
	engine.Get("/users/:alias/stream", getStream(ar), authenticated)

	engine.Delete("/artworks/:id", deleteArtwork(ar), authenticated) // tk review path

	engine.Post("/artworks/:id/comments", addComment(ar), authenticated)
	engine.Delete("/artworks/:id/comments/:commentId", deleteComment(ar), authenticated)

	engine.Put("/artworks/:artworkId/reactions/:alias", setReaction(ar), authenticated)
}

// addArtwork handles the authenticated POST "/users/:alias/artworks" route
func addArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the artwork data
		data, err := JSON.DecodeValidate[AddArtworkData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// ensure that the follower's alias matches the authenticated user's
		var user = auth.MustGetUser(request)
		if user.Alias != rest.GetParam(request, "alias") {
			JSON.Unauthorised(writer)
			return
		}

		// return a JSON with the artwork's ID and time of creation
		id, updated, err := ar.AddArtwork(data, user.Id)
		switch err {
		case nil:
			JSON.Created(writer, struct {
				Id      string
				Updated string
			}{
				Id:      id,
				Updated: updated,
			})
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

func deleteArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var artworkId = httprouter.ParamsFromContext(request.Context()).ByName("id")

		if artworkId == "" {
			JSON.BadRequest(writer)
			return
		}

		// issues a bad request regardless of authorisation issues to deny information about existing resources
		if deleted := ar.DeleteArtwork(artworkId, auth.MustGetUser(request).Id); deleted {
			JSON.NoContent(writer)
		} else {
			JSON.BadRequest(writer)
		}
	}
}

func setReaction(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// the path user must match the authorised one
		var user = auth.MustGetUser(request)
		if user.Alias != rest.GetParam(request, "alias") {
			JSON.Unauthorised(writer)
			return
		}

		// validate
		data, err := JSON.DecodeValidate[ReactionData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		var date = time.Now()

		if err = ar.SetReaction(user.Id, rest.GetParam(request, "artworkId"), date, data); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Ok(writer, struct {
			Reaction ReactionType
			Date     time.Time
		}{
			data.Reaction,
			date,
		})
	}
}

func addComment(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		data, err := JSON.DecodeValidate[CommentData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		var artworkId = rest.GetParam(request, "id")
		id, date, err := ar.AddComment(auth.MustGetUser(request).Id, artworkId, data)

		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Ok(writer, struct {
			Id   string
			Date time.Time
		}{
			Id:   id,
			Date: date,
		})
	}
}

func deleteComment(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		switch err := ar.DeleteComment(auth.MustGetUser(request).Id, rest.GetParam(request, "commentId")); err {
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
		if user.Alias != rest.GetParam(request, "alias") {
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
		if user.Alias != rest.GetParam(request, "alias") {
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
