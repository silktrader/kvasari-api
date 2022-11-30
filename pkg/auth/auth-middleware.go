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

const userKey = "user"

var authError = errors.New("missing or malformed authorisation header")

// Auth performs ridiculously simple checks on routes, ensuring that requests include a valid user ID, as per project's specifications.
func Auth(ar Repository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {

			var id, err = parseBearer(request)
			if err != nil {
				reportUnauthorised(w)
				return
			}

			// in the (temporary) absence of actual authorisation, verify the user exists
			// tk return pointer instead of user?
			user, err := ar.GetUserById(id)
			if err != nil {
				reportUnauthorised(w)
				return
			}

			// create a new context, stemming from the original one, adding the user's details for future reference
			next.ServeHTTP(w, request.WithContext(context.WithValue(request.Context(), userKey, user)))
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

// MustGetUser fetches the user's ID from the authorisation header, assuming the handler includes auth middleware.
func MustGetUser(request *http.Request) User {
	// one could return an error to detect a possibly missing auth middleware
	// but this is an exceptional occurrence stemming from careless auth configuration
	var userValue = request.Context().Value(userKey)
	if userValue == nil {
		panic(authError)

	}

	user, ok := userValue.(User)
	if !ok {
		panic(authError)
	}
	return user
}

func reportUnauthorised(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	return
}
