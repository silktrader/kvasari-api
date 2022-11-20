package artworks

import (
	"github.com/julienschmidt/httprouter"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"github.com/silktrader/kvasari/pkg/users"
	"net/http"
	"time"
)

func RegisterHandlers(engine rest.Engine, ar ArtworkRepository, ur users.UserRepository) {
	engine.Post("/artworks", addArtwork(ar), auth.Auth(ur))
	engine.Delete("/artworks/:id", deleteArtwork(ar), auth.Auth(ur))
}

func addArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the artwork data
		var data AddArtworkData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		var userId = auth.GetUserId(request)

		id, updated, err := ar.AddArtwork(data, userId)
		if err != nil {
			JSON.InternalServerError(writer, err)
		}

		JSON.Created(writer, struct {
			Id      string
			Updated time.Time
		}{
			Id:      id,
			Updated: updated,
		})
	}
}

func deleteArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		var userId = auth.GetUserId(request)
		var artworkId = httprouter.ParamsFromContext(request.Context()).ByName("id")

		if artworkId == "" {
			JSON.BadRequest(writer)
			return
		}

		// issues a bad request regardless of authorisation issues to deny information about existing resources
		if deleted := ar.DeleteArtwork(artworkId, userId); deleted {
			JSON.NoContent(writer)
		} else {
			JSON.BadRequest(writer)
		}
	}
}
