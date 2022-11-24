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
	engine.Post("/artworks", addArtwork(ar), auth.Auth(aur))
	engine.Delete("/artworks/:id", deleteArtwork(ar), auth.Auth(aur))
}

func addArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// parse and validate the artwork data
		var data AddArtworkData
		if err := JSON.DecodeValidate(request, &data); err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		id, updated, err := ar.AddArtwork(data, auth.GetUser(request).Id)
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
