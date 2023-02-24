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
	"github.com/silktrader/kvasari/pkg/users"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// maxFileUploadSize determines the maximum incoming file size; set to ~40MiB
const maxFileUploadSize = 41943040

// acceptableFileTypes describes which file types can be uploaded by users
var acceptableFileTypes = [...]string{"image/jpeg", "image/png", "image/webp"}

func RegisterHandlers(engine Engine, ar Storer, aur auth.IRepository) {
	var authenticated = auth.Auth(aur)

	// artworks management
	engine.Post("/artworks", addArtwork(ar), authenticated)
	engine.Delete("/artworks/:artworkId", deleteArtwork(ar), authenticated)
	engine.Get("/artworks/:artworkId/data", getArtworkData(ar), authenticated)
	engine.Get("/artworks/:artworkId/image", getArtworkImage(ar), authenticated)
	engine.Get("/artworks", getArtworks(ar), authenticated)
	engine.Put("/artworks/:artworkId/title", setTitle(ar), authenticated)

	// comments
	engine.Post("/artworks/:artworkId/comments", addComment(ar), authenticated)
	engine.Delete("/artworks/:artworkId/comments/:commentId", deleteComment(ar), authenticated)
	engine.Get("/artworks/:artworkId/comments", getArtworkComments(ar), authenticated)

	// reactions
	engine.Put("/artworks/:artworkId/reactions/:alias", setReaction(ar), authenticated)
	engine.Delete("/artworks/:artworkId/reactions/:alias", removeReaction(ar), authenticated)
	engine.Get("/artworks/:artworkId/reactions", getArtworkReactions(ar), authenticated)

	// user specific aggregates
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

func addArtwork(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// ensure that the uploader alias matches the authenticated user's one
		var user = auth.MustGetUser(request)
		if user.Alias != request.FormValue("alias") {
			JSON.Forbidden(writer)
			return
		}

		// ParseMultipartForm's argument refers to a memory limit, additional bytes will be cached on disk
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
			JSON.BadRequestWithMessage(writer, fmt.Sprintf("%s is too large; limit file sizes to 40MiB", header.Filename))
			return
		}

		var buffer = make([]byte, 512)
		_, err = uploadedFile.Read(buffer)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		// format detection by reading a file's header avoid extension renaming issues
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

		// detect file extension or format
		var fileFormat = getFormat(filetype)

		// guard against duplicate uploads, before an existing file is truncated
		// attempt to clean a previously soft-deleted artwork and related comments, reactions
		existsImage, err := ar.CleanDeletedArtwork(checksum, user.Id)
		if err != nil {
			JSON.InternalServerError(writer, err)
			return
		}

		// attempt to update the database
		date, err := ar.AddArtwork(AddArtworkData{
			Id:       checksum,
			AuthorId: user.Id,
			Format:   fileFormat,
			Type:     Painting, // default for the moment
		})

		if err != nil {
			if errors.Is(err, ErrDupArtwork) {
				JSON.BadRequestWithMessage(writer, "The artwork image is already present.")
				return
			}
			JSON.InternalServerError(writer, err)
			return
		}

		// write image file named after its hash and detected file extension
		// there's no need to perform the operation if the file hasn't been deleted yet
		if !existsImage {
			if err = writeImage(uploadedFile, checksum, string(fileFormat), ar.GetImagesPath()); err != nil {
				JSON.InternalServerError(writer, err)
				// in the unlikely case an error occurs while writing to disk, attempt to clean the related DB entry
				_ = ar.CleanArtwork(checksum, user.Id)
				return
			}
		}

		JSON.Created(writer, struct {
			Id      string
			Updated ntime.NTime
			Format  string
		}{
			Id:      checksum,
			Updated: date,
			Format:  string(fileFormat),
		})
	}
}

// writeImage creates (or truncates) an image file named after its hash and header-detected file extension.
func writeImage(file multipart.File, checksum, format, path string) error {
	storedFile, err := os.Create(fmt.Sprintf("%s/%s.%s", path, checksum, format))
	if err != nil {
		return err
	}
	defer closeFile(storedFile)

	// seek to file start, to offset previous checksum calculations or header reads
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if written, e := io.Copy(storedFile, file); e != nil {
		return e
	} else if written == 0 {
		return errors.New("no bytes written")
	}
	return nil
}

// deleteArtwork handles the authenticated DELETE "/artworks/:artworkId" route
func deleteArtwork(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// issues a bad request regardless of authorisation issues to deny information about existing resources
		if err := ar.DeleteArtwork(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); err == nil {
			JSON.NoContent(writer)
		} else {
			JSON.BadRequest(writer)
		}
	}
}

// getArtworkData handles the authenticated GET "/artworks/:artworkId/data" route and provides an artwork's metadata
func getArtworkData(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		switch response, err := ar.GetArtworkData(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); {
		case errors.Is(err, ErrNotFound):
			JSON.NotFound(writer, "Artwork not found")
		case err == nil:
			JSON.Ok(writer, response)
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// getArtworkImage handles the authenticated GET "/artworks/:artworkId/image" route and serves binary data.
func getArtworkImage(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var artworkId = GetParam(request, "artworkId")
		switch metadata, err := ar.GetImageMetadata(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); {
		case err == nil:
			http.ServeFile(writer, request, fmt.Sprintf("%s/%s.%s", ar.GetImagesPath(), artworkId, metadata.Format))
		case errors.Is(err, ErrNotFound):
			JSON.NotFound(writer, "Image not found, or forbidden access")
		default:
			JSON.InternalServerError(writer, err)
		}
	}
}

// getArtworks handles the authenticated GET "/artworks" route, with parameters: "alias", "since" and "latest"
func getArtworks(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// fetch and validate required parameters
		author, since, latest, err := getValidateArtworkParameters(request.URL.Query())
		if err != nil {
			JSON.ValidationError(writer, err)
		} else if artworks, e := ar.GetUserArtworks(author, auth.MustGetUser(request).Id, PageData{12, since, latest}); e != nil {
			JSON.InternalServerError(writer, e)
		} else {
			JSON.Ok(writer, artworks)
		}
	}
}

// getValidateArtworkParameters ensures that all required parameters are present and abide to validation rules.
// There's no need to check for the remaining parameters when one fails.
func getValidateArtworkParameters(params url.Values) (alias, since, latest string, err error) {
	alias = params.Get("artist")
	if err = users.ValidateUserAlias(alias); err != nil {
		return alias, since, latest, err
	}
	since = params.Get("since")
	if err = ValidateDate(since); err != nil {
		return alias, since, latest, err
	}
	latest = params.Get("latest")
	return alias, since, latest, ValidateDate(latest)
}

// setReaction handles the authenticated PUT "/artworks/:artworkId/reactions/:alias" route
func setReaction(ar Storer) http.HandlerFunc {
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
func removeReaction(ar Storer) http.HandlerFunc {
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
func addComment(ar Storer) http.HandlerFunc {
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
func deleteComment(ar Storer) http.HandlerFunc {
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
func getArtworkComments(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if comments, err := ar.GetArtworkComments(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); err == nil {
			JSON.Ok(writer, comments)
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// getArtworkReactions handles the authenticated GET "/artworks/:artworkId/reactions" route
func getArtworkReactions(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if reacts, err := ar.GetArtworkReactions(GetParam(request, "artworkId"), auth.MustGetUser(request).Id); err == nil {
			JSON.Ok(writer, reacts)
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}

// getStream handles the authenticated GET "/users/:alias/stream?since=date&latest=date" route
func getStream(ar Storer) http.HandlerFunc {
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
			JSON.InternalServerError(writer, err)
			return
		}

		JSON.Ok(writer, stream)
	}
}

// setTitle handles the authenticated PUT "/artworks/:artworkId/title" route
func setTitle(ar Storer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// fetch and validate parameters
		var artworkId = GetParam(request, "artworkId")
		if !isValidArtworkId(artworkId) {
			JSON.BadRequestWithMessage(writer, "invalid artwork ID provided")
			return
		}

		// fetch and validate the new title's data
		data, err := JSON.DecodeValidate[UpdateArtworkTitleData](request)
		if err != nil {
			JSON.ValidationError(writer, err)
			return
		}

		// attempt to change the title
		if err = ar.SetArtworkTitle(artworkId, auth.MustGetUser(request).Id, data.Title); err == nil {
			JSON.NoContent(writer)
		} else if errors.Is(err, ErrNotModified) {
			JSON.NotFound(writer, "Unauthorised action or resource not found")
		} else {
			JSON.InternalServerError(writer, err)
		}
	}
}
