package handlers

import (
	"io/ioutil"
	"math"
	"net/http"
	"strconv"

	"github.com/vardius/gorouter/v4/context"

	"github.com/vardius/go-api-boilerplate/cmd/user/internal/domain/user"
	"github.com/vardius/go-api-boilerplate/cmd/user/internal/infrastructure/persistence"
	"github.com/vardius/go-api-boilerplate/pkg/commandbus"
	"github.com/vardius/go-api-boilerplate/pkg/errors"
	"github.com/vardius/go-api-boilerplate/pkg/http/response"
	"github.com/vardius/go-api-boilerplate/pkg/identity"
)

// BuildCommandDispatchHandler wraps user gRPC client with http.Handler
func BuildCommandDispatchHandler(cb commandbus.CommandBus) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var e error

		if r.Body == nil {
			response.MustJSONError(r.Context(), w, ErrEmptyRequestBody)
			return
		}

		params, ok := context.Parameters(r.Context())
		if !ok {
			response.MustJSONError(r.Context(), w, ErrInvalidURLParams)
			return
		}

		defer r.Body.Close()
		body, e := ioutil.ReadAll(r.Body)
		if e != nil {
			appErr := errors.Wrap(e, errors.INTERNAL, "Invalid request body")

			response.MustJSONError(r.Context(), w, appErr)
			return
		}

		c, e := user.NewCommandFromPayload(params.Value("command"), body)
		if e != nil {
			appErr := errors.Wrap(e, errors.INTERNAL, errors.ErrorMessage(e))

			response.MustJSONError(r.Context(), w, appErr)
			return
		}

		out := make(chan error, 1)
		defer close(out)

		go func() {
			cb.Publish(r.Context(), c, out)
		}()

		select {
		case <-r.Context().Done():
			appErr := errors.Wrap(r.Context().Err(), errors.TIMEOUT, "Request timeout")

			response.MustJSONError(r.Context(), w, appErr)
			return
		case e = <-out:
			if e != nil {
				appErr := errors.Wrap(e, errors.INTERNAL, "Command handler error")

				response.MustJSONError(r.Context(), w, appErr)
				return
			}
		}

		w.WriteHeader(http.StatusCreated)
		response.MustJSON(r.Context(), w, nil)
	}

	return http.HandlerFunc(fn)
}

// BuildMeHandler wraps user gRPC client with http.Handler
func BuildMeHandler(repository persistence.UserRepository) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var e error

		if r.Body == nil {
			response.MustJSONError(r.Context(), w, ErrEmptyRequestBody)
			return
		}

		i, _ := identity.FromContext(r.Context())

		u, e := repository.Get(r.Context(), i.ID.String())
		if e != nil {
			appErr := errors.Wrap(e, errors.NOTFOUND, "User not found")

			response.MustJSONError(r.Context(), w, appErr)
			return
		}

		w.WriteHeader(http.StatusOK)
		response.MustJSON(r.Context(), w, u)
	}

	return http.HandlerFunc(fn)
}

// BuildGetUserHandler wraps user gRPC client with http.Handler
func BuildGetUserHandler(repository persistence.UserRepository) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var e error

		if r.Body == nil {
			response.MustJSONError(r.Context(), w, ErrEmptyRequestBody)
			return
		}

		params, ok := context.Parameters(r.Context())
		if !ok {
			response.MustJSONError(r.Context(), w, ErrInvalidURLParams)
			return
		}

		u, e := repository.Get(r.Context(), params.Value("id"))
		if e != nil {
			appErr := errors.Wrap(e, errors.NOTFOUND, "User not found")

			response.MustJSONError(r.Context(), w, appErr)
			return
		}

		w.WriteHeader(http.StatusOK)
		response.MustJSON(r.Context(), w, u)
	}

	return http.HandlerFunc(fn)
}

// BuildListUserHandler wraps user gRPC client with http.Handler
func BuildListUserHandler(repository persistence.UserRepository) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var e error

		if r.Body == nil {
			response.MustJSONError(r.Context(), w, ErrEmptyRequestBody)
			return
		}

		pageInt, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32)
		limitInt, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 32)
		page := int32(math.Max(float64(pageInt), 1))
		limit := int32(math.Max(float64(limitInt), 20))

		totalUsers, e := repository.Count(r.Context())
		if e != nil {
			appErr := errors.New(errors.INTERNAL, http.StatusText(http.StatusInternalServerError))

			response.MustJSONError(r.Context(), w, appErr)
			return
		}

		offset := (page * limit) - limit

		paginatedList := struct {
			Users []persistence.User `json:"users"`
			Page  int32              `json:"page"`
			Limit int32              `json:"limit"`
			Total int32              `json:"total"`
		}{
			Page:  page,
			Limit: limit,
			Total: totalUsers,
		}

		if totalUsers < 1 || offset > (totalUsers-1) {
			w.WriteHeader(http.StatusOK)
			response.MustJSON(r.Context(), w, paginatedList)
			return
		}

		paginatedList.Users, e = repository.FindAll(r.Context(), limit, offset)
		if e != nil {
			appErr := errors.New(errors.INTERNAL, http.StatusText(http.StatusInternalServerError))

			response.MustJSONError(r.Context(), w, appErr)
			return
		}

		w.WriteHeader(http.StatusOK)
		response.MustJSON(r.Context(), w, paginatedList)
	}

	return http.HandlerFunc(fn)
}
