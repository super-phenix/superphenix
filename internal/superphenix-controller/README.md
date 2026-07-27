# Superphenix Controller

This project is a Rest API to control a SPX environment in kubernetes.

## Requirements

- Go : 1.23


## Endpoints

All endpoints start by `/{orgId}/{projectId}` to indicate which organisation and which project 
is concerned by the request.

They are describe in `api/`


## Build

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

