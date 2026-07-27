# Argo Controller

This project is a Rest API to interact with Argo CD cluster.

## Requirements

- Go : 1.25

## Endpoints

All endpoints start by `/{orgId}/{projectId}` to indicate which organization and which project
is concerned by the request.

They are describe in `api/`

## Build

### Local

At the project root, launch `build.sh` :
```shell
# Run go mod tidy command and the go build for linux AMD64
./build.sh
```


### Docker

You can use the Dockerfile to build an image of the project.  
Or directly using the Makefile at root project.

## Documentations

### Swagger

To install `swag` :
```bash
go install github.com/swaggo/swag/cmd/swag
```

To update swagger information :
```bash
swag fmt -d ./internal/api && swag init --pd --pdl 1 --parseInternal -d ./internal/api -g ./api.go -o api
```

