package data

import (
	"context"
	"fmt"
	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"
	"slices"

	"github.com/rs/zerolog/log"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

	permifyPayload "buf.build/gen/go/permifyco/permify/protocolbuffers/go/base/v1"
)

func Write(ctx context.Context, tenantId string, tuples []*v1.Tuple) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	if len(tuples) <= 0 {
		return fmt.Errorf("no tuples to write")
	}

	batchSize := 99
	var err error
	for batch := range slices.Chunk(tuples, batchSize) {
		_, err = client.Data.WriteRelationships(ctx, &permifyPayload.RelationshipWriteRequest{
			TenantId: tenantId,
			Metadata: &permifyPayload.RelationshipWriteRequestMetadata{SchemaVersion: ""},
			Tuples:   batch,
		})

		if err != nil {
			log.Error().Err(err).Any("batch", batch).Msg("Failed to write a batch of tuples")
		}
	}

	return err
}

func Read(ctx context.Context, tenantId string, filter *v1.TupleFilter) (*permifyPayload.RelationshipReadResponse, error) {
	if !client.IsConnected() {
		return &permifyPayload.RelationshipReadResponse{}, fmt.Errorf("client not connected")
	}

	relationships, err := client.Data.ReadRelationships(ctx, &permifyPayload.RelationshipReadRequest{
		TenantId: tenantId,
		Metadata: &permifyPayload.RelationshipReadRequestMetadata{SnapToken: ""},
		Filter: &permifyPayload.TupleFilter{
			Entity: &permifyPayload.EntityFilter{
				Type: filter.Entity.Type,
				Ids:  filter.Entity.Ids,
			},
			Relation: filter.Relation,
			Subject: &permifyPayload.SubjectFilter{
				Type: filter.Subject.Type,
				Ids:  filter.Subject.Ids,
			},
		},
	})
	if err != nil {
		log.Error().Err(err).Any("filter", filter).Msg("Failed to read tuples")
		return &permifyPayload.RelationshipReadResponse{}, err
	}

	return relationships, nil
}

func Delete(ctx context.Context, tenantId string, filter *v1.TupleFilter) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Data.DeleteRelationships(ctx, &permifyPayload.RelationshipDeleteRequest{
		TenantId: tenantId,
		Filter:   filter,
	})

	if err != nil {
		log.Error().Err(err).Any("filter", filter).Msg("Failed to delete tuples")
	}
	return err
}
