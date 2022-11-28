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

	engine.Post("/users/:alias/artworks", addArtwork(ar), authenticated)
	engine.Delete("/artworks/:id", deleteArtwork(ar), authenticated)

	engine.Put("/artworks/:artworkId/reactions/:alias", setArtworkReaction(ar), authenticated)
}

func addArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the artwork data
		data, err := JSON.DecodeValidate[AddArtworkData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// ensure that the follower's alias matches the authenticated user's
		var user = auth.GetUser(request)
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
				Updated time.Time
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
		if deleted := ar.DeleteArtwork(artworkId, auth.GetUser(request).Id); deleted {
			JSON.NoContent(writer)
		} else {
			JSON.BadRequest(writer)
		}
	}
}

func setArtworkReaction(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// the path user must match the authorised one
		var user = auth.GetUser(request)
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

		switch err = ar.SetReaction(user.Id, rest.GetParam(request, "artworkId"), date, data); err {
		case nil:
			JSON.Ok(writer, struct {
				Reaction ReactionType
				Date     time.Time
			}{
				data.Reaction,
				date,
			})
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}
