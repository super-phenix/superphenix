package dbaas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	argodbaas "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app/dbaas"

	"github.com/google/uuid"
)

func TestCombineListResult(t *testing.T) {
	product := model.Product{EffectiveID: "spx-db", ProductName: "orders", CodeAZ: "az1", ProductTypeId: "dbaas"}
	product.ID = uuid.New()
	cluster := map[string]any{"status": map[string]any{"phase": "Cluster in healthy state"}}
	azEntry := func(id, eid, name, gitops string) map[string]any {
		return map[string]any{"id": id, "eid": eid, "productName": name, "gitops": gitops, "cluster": cluster, "endpoints": []any{}}
	}

	tests := []struct {
		name      string
		az        map[string][]any
		products  []model.Product
		want      []ProductResponse
		wantState []bool // whether the entry carries the AZ cluster
	}{
		{
			name:      "merged",
			az:        map[string][]any{"az1": {azEntry(product.ID.String(), "spx-db", "local-name", "false")}},
			products:  []model.Product{product},
			want:      []ProductResponse{{ID: product.ID.String(), EId: "spx-db", ProductName: "orders", CodeAZ: "az1", ProductTypeId: "dbaas", Gitops: "false"}},
			wantState: []bool{true},
		},
		{
			name:      "db only",
			az:        map[string][]any{},
			products:  []model.Product{product},
			want:      []ProductResponse{{ID: product.ID.String(), EId: "spx-db", ProductName: "orders", CodeAZ: "az1", ProductTypeId: "dbaas", Gitops: "false"}},
			wantState: []bool{false},
		},
		{
			name:      "gitops only",
			az:        map[string][]any{"az2": {azEntry("my-db", "spx-git", "my-db", "true")}},
			want:      []ProductResponse{{ID: "my-db", EId: "spx-git", ProductName: "my-db", CodeAZ: "az2", Gitops: "true"}},
			wantState: []bool{true},
		},
		{
			name:      "AZ only and not gitops is being deleted",
			az:        map[string][]any{"az1": {azEntry(uuid.NewString(), "spx-del", "local-name", "false")}},
			want:      []ProductResponse{},
			wantState: []bool{},
		},
		{
			name:      "malformed AZ entry is skipped",
			az:        map[string][]any{"az1": {"not-an-object"}},
			want:      []ProductResponse{},
			wantState: []bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := combineListResult(tt.az, tt.products)
			if len(got) != len(tt.want) {
				t.Fatalf("combineListResult() returned %d entries, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].ProductResponse != tt.want[i] {
					t.Errorf("entry %d = %+v, want %+v", i, got[i].ProductResponse, tt.want[i])
				}
				if (got[i].Cluster != nil) != tt.wantState[i] {
					t.Errorf("entry %d cluster = %v, want present %v", i, got[i].Cluster, tt.wantState[i])
				}
			}
		})
	}
}

func TestResolveSubnet(t *testing.T) {
	tests := []struct {
		name      string
		subnetEId string
		wantPath  string
		want      argodbaas.Subnet
		wantErr   bool
	}{
		{
			name:      "subnet of the project",
			subnetEId: "spx-subnet",
			wantPath:  "/org/project/subnet/spx-subnet",
			want:      argodbaas.Subnet{LocalID: "my-subnet", VPC: "my-vpc", CIDR: "10.0.0.0/24"},
		},
		{
			name:      "dot segments stay in one segment",
			subnetEId: "../../other/mark",
			wantPath:  "/org/project/subnet/..%2F..%2Fother%2Fmark",
			want:      argodbaas.Subnet{LocalID: "my-subnet", VPC: "my-vpc", CIDR: "10.0.0.0/24"},
		},
		{name: "missing subnet", subnetEId: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var subnetPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.EscapedPath(), "/org/project/subnet/"):
					subnetPath = r.URL.EscapedPath()
					_, _ = w.Write([]byte(`{"id":"my-subnet","subnet":{"spec":{"cidrBlock":"10.0.0.0/24","vpc":"spx-vpc"}}}`))
				case r.URL.EscapedPath() == "/org/project/vpc/spx-vpc":
					_, _ = w.Write([]byte(`{"id":"my-vpc"}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			azDb := config.AZConfig{Code: "az1", ControllerUrl: srv.URL}
			got, err := resolveSubnet(context.Background(), azDb, "org", "project", tt.subnetEId)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveSubnet() error = %v, wantErr %v", err, tt.wantErr)
			}
			if subnetPath != tt.wantPath {
				t.Errorf("subnet path = %q, want %q", subnetPath, tt.wantPath)
			}
			if got != tt.want {
				t.Errorf("resolveSubnet() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
