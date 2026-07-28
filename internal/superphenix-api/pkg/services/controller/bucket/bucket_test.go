package bucket

import (
	"encoding/json"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBuildCreateControllerBody(t *testing.T) {
	m := spxId.Metadata{}
	err := m.GenerateMetadata(
		"22222222-2222-2222-2222-222222222222",
		"11111111-1111-1111-1111-111111111111",
		"33333333-3333-3333-3333-333333333333",
	)
	assert.NoError(t, err)

	maxObjects := uint64(100)

	tests := []struct {
		name         string
		body         func() CreateBucketBody
		wantContains []string
		wantAbsent   []string
	}{
		{
			name: "full config carried over",
			body: func() CreateBucketBody {
				var b CreateBucketBody
				b.General.ProductName = "my-bucket"
				b.General.StorageClass = "standard"
				b.General.Config = BucketConfig{
					MaxObjects: &maxObjects,
					MaxSize:    "10Gi",
					Policy:     `{"Version":"2012-10-17"}`,
					Lifecycle:  `{"Rules":[]}`,
				}
				return b
			},
			wantContains: []string{
				`"storageClass":"standard"`,
				`"maxObjects":100`,
				`"maxSize":"10Gi"`,
				`"policy":"{\"Version\":\"2012-10-17\"}"`,
				`"lifecycle":"{\"Rules\":[]}"`,
				`"resourceLocalID":"33333333-3333-3333-3333-333333333333"`,
			},
		},
		{
			name: "unset optional fields omitted from JSON",
			body: func() CreateBucketBody {
				var b CreateBucketBody
				b.General.ProductName = "my-bucket"
				b.General.StorageClass = "standard"
				return b
			},
			wantContains: []string{`"storageClass":"standard"`},
			wantAbsent:   []string{"maxObjects", "maxSize", "policy", "lifecycle", "productName"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCreateControllerBody(tt.body(), m)
			assert.Equal(t, m, got.Metadata)

			marshal, err := json.Marshal(got)
			assert.NoError(t, err)
			for _, want := range tt.wantContains {
				assert.Contains(t, string(marshal), want)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, string(marshal), absent)
			}
		})
	}
}

func TestCombineListResult(t *testing.T) {
	dbId := uuid.New()
	dbProduct := model.Product{
		EffectiveID:   "spx-eid-db",
		ProductName:   "db-bucket",
		CodeAZ:        "az1",
		ProductTypeId: model.ProductTypeBucket,
	}
	dbProduct.ID = dbId

	controllerResult := map[string]interface{}{
		"id":          dbId.String(),
		"eid":         "spx-eid-db",
		"productName": "",
		"gitops":      "false",
		"bucket":      map[string]interface{}{"name": "spx-eid-db"},
	}
	gitopsResult := map[string]interface{}{
		"id":          "git-local-id",
		"eid":         "spx-eid-git",
		"productName": "git-bucket",
		"gitops":      "true",
		"bucket":      map[string]interface{}{"name": "spx-eid-git"},
	}

	tests := []struct {
		name          string
		concatResults map[string][]interface{}
		resources     []model.Product
		want          []BucketFullResponse
	}{
		{
			name:          "db-only product marked not gitops",
			concatResults: map[string][]interface{}{},
			resources:     []model.Product{dbProduct},
			want: []BucketFullResponse{{
				ProductResponse: ProductResponse{
					ID:            dbId.String(),
					EId:           "spx-eid-db",
					ProductName:   "db-bucket",
					CodeAZ:        "az1",
					ProductTypeId: model.ProductTypeBucket,
					Gitops:        "false",
				},
			}},
		},
		{
			name:          "controller-only gitops passthrough",
			concatResults: map[string][]interface{}{"az1": {gitopsResult}},
			resources:     []model.Product{},
			want: []BucketFullResponse{{
				ProductResponse: ProductResponse{
					ID:          "git-local-id",
					EId:         "spx-eid-git",
					ProductName: "git-bucket",
					CodeAZ:      "az1",
					Gitops:      "true",
				},
				Bucket: map[string]interface{}{"name": "spx-eid-git"},
			}},
		},
		{
			name:          "matched db and controller entries merged",
			concatResults: map[string][]interface{}{"az1": {controllerResult}},
			resources:     []model.Product{dbProduct},
			want: []BucketFullResponse{{
				ProductResponse: ProductResponse{
					ID:            dbId.String(),
					EId:           "spx-eid-db",
					ProductName:   "db-bucket",
					CodeAZ:        "az1",
					ProductTypeId: model.ProductTypeBucket,
					Gitops:        "false",
				},
				Bucket: map[string]interface{}{"name": "spx-eid-db"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapResourceCheck := make(map[uuid.UUID]bool)
			for _, p := range tt.resources {
				mapResourceCheck[p.ID] = false
			}

			got := combineListResult(tt.concatResults, tt.resources, mapResourceCheck)
			assert.Equal(t, tt.want, got)
		})
	}
}
