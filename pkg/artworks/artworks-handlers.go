package artworks

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/silktrader/kvasari/pkg/auth"
	JSON "github.com/silktrader/kvasari/pkg/json-utilities"
	"github.com/silktrader/kvasari/pkg/ntime"
	. "github.com/silktrader/kvasari/pkg/rest"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

// maxFileUploadSize determines the maximum incoming file size; set to ~40MiB
const maxFileUploadSize = 41943040

// acceptableFileTypes describes which file types can be uploaded by users
var acceptableFileTypes = [...]string{"image/jpeg", "image/png", "image/webp"}

func RegisterHandlers(engine Engine, ar ArtworkRepository, aur auth.IRepository) {

	var authenticated = auth.Auth(aur)

	// artworks management
	engine.Post("/artworks", addArtwork(ar), authenticated)
	engine.Delete("/artworks/:artworkId", deleteArtwork(ar), authenticated)
	engine.Get("/artworks/:artworkId", getArtwork(ar), authenticated)
	engine.Get("/artworks/:artworkId/image", getArtworkImage(ar), authenticated)

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

func closeFile(file multipart.File) {
	_ = file.Close()
}

func getFormat(imageType string) ImageFormat {
	switch imageType {
	case "image/png":
		return PNG
	case "image/webp":
		return WEBP
	default:
		return JPG
	}
}

func addArtwork(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		// ensure that the uploader alias matches the authenticated user's one
		var user = auth.MustGetUser(request)
		if user.Alias != request.FormValue("alias") {
			JSON.Forbidden(writer)
			return
		}

		// ParseMultipartForm argument refers to a memory limit, additional bytes will be cached on disk
		// 100 << 20 is a bitwise equivalent of 100 ** 2 ^ 20
		if err := request.ParseMultipartForm(maxFileUploadSize); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		uploadedFile, header, err := request.FormFile("image")
		if err != nil {
			JSON.BadRequestWithMessage(writer, "Malformed image upload")
			return
		}

		defer closeFile(uploadedFile)

		// ensure files are sized appropriately
		if header.Size > maxFileUploadSize {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("%s is too large; limit file sizes to 15MB", header.Filename))
			return
		}

		var buffer = make([]byte, 512)
		_, err = uploadedFile.Read(buffer)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		filetype := http.DetectContentType(buffer)
		var invalidFileType = true
		for _, acceptableType := range acceptableFileTypes {
			if filetype == acceptableType {
				invalidFileType = false
				break
			}
		}
		if invalidFileType {
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("%s isn't a valid file type; choose among: %v",
				header.Filename,
				strings.Trim(fmt.Sprintf("%v", acceptableFileTypes), "[]")))
			return
		}

		// end filetype detection, seek to start to avoid parsing issues
		if _, err = uploadedFile.Seek(0, io.SeekStart); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		// hash the image, to identify it and ensure it's not a duplicate
		hash := sha256.New()
		if _, err = io.Copy(hash, uploadedFile); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}
		var checksum = hex.EncodeToString(hash.Sum(nil))

		// write image file named after its hash and detected file extension
		var fileFormat = getFormat(filetype)
		storedFile, err := os.Create(fmt.Sprintf("./images/%s.%s", checksum, fileFormat))
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		defer closeFile(storedFile)

		// seek to start after checksum calculations
		if _, err = uploadedFile.Seek(0, io.SeekStart); err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		if written, e := io.Copy(storedFile, uploadedFile); e != nil || written == 0 {
			JSON.InternalServerError(writer, e)
			return
		}

		// finally, attempt to update the database
		if date, e := ar.AddArtwork(AddArtworkData{
			Id:       checksum,
			AuthorId: user.Id,
			Format:   fileFormat,
			Type:     Painting, // default for the moment tk change to default in DB
		}); e != nil {
			JSON.InternalServerError(writer, e)
		} else {
			JSON.Created(writer, struct {
				Id      string
				Updated ntime.NTime
			}{
				Id:      checksum,
				Updated: date,
			})
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

func getArtworkImage(ar ArtworkRepository) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var artworkId = GetParam(request, "artworkId")
		if metadata, err := ar.GetImageMetadata(artworkId, auth.MustGetUser(request).Id); err == nil {
			http.ServeFile(writer, request, fmt.Sprintf("images/%s.%s", artworkId, metadata.Format))
		} else {
			JSON.NotFound(writer, "Image not found, or forbidden access")
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
		// tk return 201
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
