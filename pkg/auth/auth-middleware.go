package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

/* There are two solutions to avoiding cyclic imports between `auth` and `user` packages:
1. merge the two in the user package
2. adopt and maintain an interface as a dependency in the auth package
*/

const userIdKey = "userId"

type userChecker interface {
	ExistsUserId(id string) bool
}

// Auth performs ridiculously simple checks on routes, ensuring that requests include a valid user ID, as per project's specifications.
func Auth(ur userChecker) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {

			var id, err = parseBearer(request)

			if err != nil {
				reportUnauthorised(w)
				return
			}

			// verify the user exists
			if ur.ExistsUserId(id) {
				// create a new context, stemming from the original one, adding the user's id for future reference
				next.ServeHTTP(w, request.WithContext(context.WithValue(request.Context(), userIdKey, id)))
			} else {
				reportUnauthorised(w)
			}

		})
	}
}

// parseBearer extracts the user id from the authorization header.
func parseBearer(request *http.Request) (string, error) {
	var header = request.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		var userId = header[7:]
		if len(userId) == 36 {
			return userId, nil
		}
	}
	return "", errors.New("bad authorization header")
}

func GetUserId(request *http.Request) (string, error) {
	var id = request.Context().Value(userIdKey)
	// return an error to detect a possibly missing auth middleware
	if id == nil {
		return "", errors.New("missing user ID authorization header")
	}
	return id.(string), nil
}

func reportUnauthorised(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	return
}
