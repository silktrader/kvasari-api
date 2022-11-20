package artworks

import (
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/rest"
	"github.com/silktrader/kvasari/pkg/users"
	"net/http"
	"time"
)

func RegisterHandlers(engine rest.Engine, ar ArtworkRepository, ur users.UserRepository) {
	engine.Post("/artworks", addArtwork(ar), auth.Auth(ur))
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
