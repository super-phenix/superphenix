module github.com/super-phenix/superphenix/internal/superphenix-api

go 1.25.7

require (
	github.com/go-chi/chi/v5 v5.3.1
	github.com/go-chi/cors v1.2.2
	github.com/ory/kratos-client-go v1.3.8
	github.com/prometheus/client_golang v1.23.2
	github.com/rs/zerolog v1.35.1
	github.com/spf13/viper v1.21.0
	github.com/super-phenix/superphenix/pkg/chi-helper v0.0.0
	github.com/super-phenix/superphenix/pkg/permify-wrapper v0.0.0
	github.com/super-phenix/superphenix/pkg/superphenix-id v0.0.0
	github.com/super-phenix/superphenix/pkg/utils v0.0.0
	github.com/swaggo/swag v1.16.6
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.40.0
)

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/go-gormigrate/gormigrate/v2 v2.1.5
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.1-0.20241114170450-2d3c2a9cc518
	github.com/stretchr/testify v1.11.1
	github.com/swaggo/http-swagger v1.3.4
	golang.org/x/crypto v0.54.0
	gopkg.in/yaml.v2 v2.4.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.1
	k8s.io/apimachinery v0.35.0
	k8s.io/client-go v0.34.2
)

require (
	buf.build/gen/go/envoyproxy/protoc-gen-validate/protocolbuffers/go v1.34.2-20221025150516-6607b10f00ed.2 // indirect
	buf.build/gen/go/grpc-ecosystem/grpc-gateway/protocolbuffers/go v1.34.2-20221127060915-a1ecdc58eccd.2 // indirect
	buf.build/gen/go/permifyco/permify/grpc/go v1.5.1-20240722150440-5ee7aa4c5fb5.1 // indirect
	buf.build/gen/go/permifyco/permify/protocolbuffers/go v1.34.2-20240722150440-5ee7aa4c5fb5.2 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/Permify/permify-go v0.4.9 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.4 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/swaggo/files v0.0.0-20220610200504-28940afbdbfe // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260317180543-43fb72c5454a // indirect
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/super-phenix/superphenix/pkg/utils => ../../pkg/utils

replace github.com/super-phenix/superphenix/pkg/permify-wrapper => ../../pkg/permify-wrapper

replace github.com/super-phenix/superphenix/pkg/superphenix-id => ../../pkg/superphenix-id

replace github.com/super-phenix/superphenix/pkg/chi-helper => ../../pkg/chi-helper
