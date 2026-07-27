package utils

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"

	"github.com/google/uuid"
)

func GetUserUuid(r *http.Request) (uuid.UUID, error) {
	userId := r.Context().Value(consts.ContextUserId)
	if userId == nil {
		return uuid.Nil, errors.New("no id found in context")
	}
	userUuid, err := uuid.Parse(userId.(string))

	return userUuid, err
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func Cast[T interface{}, K interface{}](object T, cast *K) error {
	temporaryVariable, err := json.Marshal(object)
	err = json.Unmarshal(temporaryVariable, &cast)
	return err
}
