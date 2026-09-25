package controller

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func azResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestResolveProductResponse(t *testing.T) {
	productID := uuid.MustParse("1b6a6d5c-9f7a-4c2e-8f3a-2a5f8c1d4e7b")
	dbProduct := model.Product{
		Model:         model.Model{ID: productID},
		EffectiveID:   "spx-4f2c",
		ProductName:   "my-cluster",
		ProductTypeId: model.ProductTypeKaaS.Name,
	}

	azBody := `{"id":"` + productID.String() + `","eid":"kaas-spx-4f2c","productName":"az-name","gitops":"true","cluster":{"name":"c1"}}`

	tests := []struct {
		name        string
		resp        *http.Response
		dbProduct   model.Product
		dbErr       error
		wantOutcome MergeOutcome
		want        ProductResponse
		wantPayload bool
	}{
		{
			name:        "no AZ response and no db row",
			resp:        nil,
			dbErr:       gorm.ErrRecordNotFound,
			wantOutcome: MergeNotFound,
		},
		{
			name:        "AZ unreachable falls back to the db row",
			resp:        nil,
			dbProduct:   dbProduct,
			wantOutcome: MergeDbOnly,
			want: ProductResponse{
				ID:            productID.String(),
				EId:           "spx-4f2c",
				ProductName:   "my-cluster",
				CodeAZ:        "az1",
				ProductTypeId: model.ProductTypeKaaS.Name,
				Gitops:        "false",
			},
		},
		{
			name:        "AZ answers 404 and we have a db row",
			resp:        azResponse(http.StatusNotFound, `{"message":"Resource not found"}`),
			dbProduct:   dbProduct,
			wantOutcome: MergeDbOnly,
			want: ProductResponse{
				ID:            productID.String(),
				EId:           "spx-4f2c",
				ProductName:   "my-cluster",
				CodeAZ:        "az1",
				ProductTypeId: model.ProductTypeKaaS.Name,
				Gitops:        "false",
			},
		},
		{
			name:        "AZ answers 404 and there is no db row",
			resp:        azResponse(http.StatusNotFound, `{"message":"Resource not found"}`),
			dbErr:       gorm.ErrRecordNotFound,
			wantOutcome: MergeNotFound,
		},
		{
			name:        "gitops resource known only by the AZ",
			resp:        azResponse(http.StatusOK, azBody),
			dbErr:       gorm.ErrRecordNotFound,
			wantOutcome: MergeAzOnly,
			want: ProductResponse{
				ID:          productID.String(),
				EId:         "kaas-spx-4f2c",
				ProductName: "az-name",
				CodeAZ:      "az1",
				Gitops:      "true",
			},
			wantPayload: true,
		},
		{
			name:        "db row and AZ resource agree",
			resp:        azResponse(http.StatusOK, azBody),
			dbProduct:   dbProduct,
			wantOutcome: MergeBoth,
			want: ProductResponse{
				ID:            productID.String(),
				EId:           "kaas-spx-4f2c",
				ProductName:   "my-cluster",
				CodeAZ:        "az1",
				ProductTypeId: model.ProductTypeKaaS.Name,
				Gitops:        "true",
			},
			wantPayload: true,
		},
		{
			// The AZ resource was never created (failed gitops sync): the AZ used to answer 200
			// with an empty object, which blanked out the whole product response.
			name:        "AZ resource identity does not match the db row",
			resp:        azResponse(http.StatusOK, `{"id":"","eid":"","productName":"","gitops":"","cluster":{}}`),
			dbProduct:   dbProduct,
			wantOutcome: MergeDbOnly,
			want: ProductResponse{
				ID:            productID.String(),
				EId:           "spx-4f2c",
				ProductName:   "my-cluster",
				CodeAZ:        "az1",
				ProductTypeId: model.ProductTypeKaaS.Name,
				Gitops:        "false",
			},
		},
		{
			name:        "AZ returns another resource than the one requested",
			resp:        azResponse(http.StatusOK, `{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","eid":"kaas-spx-other","gitops":"false"}`),
			dbProduct:   dbProduct,
			wantOutcome: MergeDbOnly,
			want: ProductResponse{
				ID:            productID.String(),
				EId:           "spx-4f2c",
				ProductName:   "my-cluster",
				CodeAZ:        "az1",
				ProductTypeId: model.ProductTypeKaaS.Name,
				Gitops:        "false",
			},
		},
		{
			name:        "unparsable AZ body falls back to the db row",
			resp:        azResponse(http.StatusOK, "<html>502 Bad Gateway</html>"),
			dbProduct:   dbProduct,
			wantOutcome: MergeDbOnly,
			want: ProductResponse{
				ID:            productID.String(),
				EId:           "spx-4f2c",
				ProductName:   "my-cluster",
				CodeAZ:        "az1",
				ProductTypeId: model.ProductTypeKaaS.Name,
				Gitops:        "false",
			},
		},
		{
			name:        "unparsable AZ body and no db row",
			resp:        azResponse(http.StatusOK, "<html>502 Bad Gateway</html>"),
			dbErr:       gorm.ErrRecordNotFound,
			wantOutcome: MergeNotFound,
		},
		{
			name:        "AZ fields of an unexpected type are read as empty strings",
			resp:        azResponse(http.StatusOK, `{"id":42,"eid":null,"gitops":true,"cluster":{"name":"c1"}}`),
			dbErr:       gorm.ErrRecordNotFound,
			wantOutcome: MergeAzOnly,
			want:        ProductResponse{CodeAZ: "az1"},
			wantPayload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, payload, outcome := ResolveProductResponse(context.Background(), "az1", tt.resp, tt.dbProduct, tt.dbErr)

			assert.Equal(t, tt.wantOutcome, outcome)
			assert.Equal(t, tt.want, got)
			if tt.wantPayload {
				assert.NotNil(t, payload)
			} else {
				assert.Nil(t, payload)
			}
		})
	}
}
