package crud

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
)

func FindAll[T interface{}, K interface{}](object T, preload ...string) ([]K, error) {
	query := db.Client.Model(&object).Where(&object)

	for _, s := range preload {
		query.Preload(s)
	}

	var list []T
	result := query.Find(&list)

	cast := make([]K, 0)
	if result.RowsAffected == 0 {
		return cast, result.Error
	}

	temporaryVariable, err := json.Marshal(list)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}

func Find[T interface{}, K interface{}](object T, preload ...string) (K, error) {
	query := db.Client.Model(&object).Where(&object)

	for _, s := range preload {
		query.Preload(s)
	}

	result := query.First(&object)

	var cast K
	if result.RowsAffected == 0 {
		return cast, result.Error
	}

	temporaryVariable, err := json.Marshal(object)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}

func FindUnscoped[T interface{}, K interface{}](object T, preload ...string) (K, error) {
	query := db.Client.Unscoped().Model(&object).Where(&object)

	for _, s := range preload {
		query.Preload(s)
	}

	result := query.First(&object)

	var cast K
	if result.RowsAffected == 0 {
		return cast, result.Error
	}

	temporaryVariable, err := json.Marshal(object)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}

func FindAllWhere[T interface{}, K interface{}](object T, where T, preload ...string) ([]K, error) {
	query := db.Client.Model(&object).Where(&where)

	for _, s := range preload {
		query.Preload(s)
	}

	var list []T
	result := query.Find(&list)

	cast := make([]K, 0)
	if result.RowsAffected == 0 {
		return cast, result.Error
	}

	temporaryVariable, err := json.Marshal(list)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}

func FindWhere[T interface{}, K interface{}](object T, where T, preload ...string) (K, error) {
	query := db.Client.Model(&object).Where(&where)

	for _, s := range preload {
		query.Preload(s)
	}

	result := query.First(&object)

	var cast K
	if result.RowsAffected == 0 {
		return cast, result.Error
	}

	temporaryVariable, err := json.Marshal(object)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}
