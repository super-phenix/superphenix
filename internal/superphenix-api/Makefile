.PHONY: all prebuild build install-deps lint fmt vet clean sdk

all: run build

prebuild: lint fmt vet

run:
	go run cmd/main.go

build: prebuild
	@echo "+ $@"
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o bin/main cmd/main.go
	go install github.com/swaggo/swag/cmd/swag
	(swag fmt -d . && swag init --pd --pdl 3 --parseInternal  -d ./pkg/api,./internal -g ./api.go -o api)

install-deps:
	@echo "+ $@"
	go get ./...

lint:
	@echo "+ $@"
	go get -u golang.org/x/lint/golint
	go list -f {{.Target}} golang.org/x/lint/golint ./...;

fmt:
	@echo "+ $@"
	gofmt -l -w .

vet:
	@echo "+ $@"
	CGO_ENABLED=0 go vet ./...

clean:
	rm -rf bin/main

sdk:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	oapi-codegen -generate types,client,spec -package sdk api/openapi3.yaml > pkg/sdk.go
