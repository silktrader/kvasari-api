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

type userKey string

const keyUser userKey = "user"

var (
	errBadAuth     = errors.New("missing or malformed authorisation header")
	errParsingAuth = errors.New("unable to parse authorisation header")
)

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
			next.ServeHTTP(w, request.WithContext(context.WithValue(request.Context(), keyUser, user)))
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
	return "", errParsingAuth
}

// MustGetUser fetches the user's ID from the authorisation header, assuming the handler includes auth middleware.
func MustGetUser(request *http.Request) User {
	// one could return an error to detect a possibly missing auth middleware
	// but this is an exceptional occurrence stemming from careless auth configuration
	var userValue = request.Context().Value(keyUser)
	if userValue == nil {
		panic(errBadAuth)

	}

	if user, ok := userValue.(User); !ok {
		panic(errBadAuth)
	} else {
		return user
	}
}

func reportUnauthorised(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
}
