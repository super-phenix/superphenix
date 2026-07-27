package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	crud "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreateResponse struct {
	Eid string `json:"eid"`
}

func WriteCreateResponse(w http.ResponseWriter, eid string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(CreateResponse{Eid: eid})
}

func CreateIntoDb(ctx context.Context, productName, productType, azCode string, orgUuid, projectUuid uuid.UUID) (model.Product, spxId.Metadata, error) {
	log := logger.GetLogger(ctx)

	product, err := crud.Save(model.Product{
		ProductName:   productName,
		CodeAZ:        azCode,
		ProjectId:     projectUuid,
		ProductTypeId: productType,
	})
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		return model.Product{}, spxId.Metadata{}, err
	}

	productId := product.ID
	m := spxId.Metadata{}
	err = m.GenerateMetadata(projectUuid.String(), orgUuid.String(), productId.String())
	if err != nil {
		err := crud.DeleteById(productId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete product")
		}
		log.Err(err).Msg("Failed to generate metadata")
		return model.Product{}, spxId.Metadata{}, err
	}

	product.EffectiveID = m.GetResourceEffectiveID()
	product, err = crud.Save(product)
	if err != nil {
		err := crud.DeleteById(productId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete product")
		}
		log.Err(err).Msg("Failed to save product into database")
		return model.Product{}, spxId.Metadata{}, err
	}

	return product, m, nil
}

// UpdateIntoDb updates the product's name when a matching DB row exists. When
// no row exists at all (gitops / shared / desync), it returns an empty product
// and nil error so the caller can still proxy to the AZ controller — mirroring
// the soft semantics of CheckProductBelongsToProject. Cross-tenant cases
// (row exists in a different project / AZ) are expected to be filtered by the
// pre-flight CheckProductBelongsToProject; this function defends in depth by
// re-checking project / AZ on the found row.
func UpdateIntoDb(ctx context.Context, productEid, productName, productType, azCode string, projectUuid uuid.UUID) (model.Product, error) {
	log := logger.GetLogger(ctx)

	product, err := crud.FindByEId(productEid)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Product{}, nil
	}
	if err != nil {
		log.Err(err).Str("eid", productEid).Msg("Failed to look up product by Effective ID")
		return model.Product{}, err
	}

	if product.ProjectId != projectUuid || product.CodeAZ != azCode {
		log.Warn().Str("eid", productEid).Str("projectId", projectUuid.String()).Str("az", azCode).Msg("Cross-tenant update attempted")
		return model.Product{}, gorm.ErrRecordNotFound
	}

	if product.ProductTypeId != productType {
		err := fmt.Errorf("product %s is not of type %s", productEid, productType)
		log.Err(err).Msg(consts.SpxResourceUpdateFailure)
		return model.Product{}, err
	}

	product.ProductName = productName
	product, err = crud.Save(product)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save product in database")
		return model.Product{}, err
	}

	return product, nil
}
