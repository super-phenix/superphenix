ARG PROJECT=superphenix-api

FROM golang:1.25-bookworm AS build

ARG PROJECT
ARG USER=$PROJECT
ARG UID=1000

RUN adduser               \
  --disabled-password     \
  --gecos ""              \
  --home "/nonexistent"   \
  --shell "/sbin/nologin" \
  --no-create-home        \
  --uid $UID              \
  $USER

WORKDIR $GOPATH/src/$PROJECT/

# Copy workspace and module definitions
COPY go.work* go.mod* go.sum* ./
COPY internal/argo-controller/go.mod* internal/argo-controller/go.sum* ./internal/argo-controller/
COPY internal/superphenix-api/go.mod* internal/superphenix-api/go.sum* ./internal/superphenix-api/
COPY internal/superphenix-controller/go.mod* internal/superphenix-controller/go.sum* ./internal/superphenix-controller/
COPY pkg/chi-helper/go.mod* pkg/chi-helper/go.sum* ./pkg/chi-helper/
COPY pkg/permify-wrapper/go.mod* pkg/permify-wrapper/go.sum* ./pkg/permify-wrapper/
COPY pkg/superphenix-id/go.mod* pkg/superphenix-id/go.sum* ./pkg/superphenix-id/
COPY pkg/utils/go.mod* pkg/utils/go.sum* ./pkg/utils/

RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s' -o /bin/main ./cmd/superphenix-api

FROM scratch

ARG PROJECT
ARG USER=$PROJECT
WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /bin/main /bin/$PROJECT

USER $USER:$USER

EXPOSE 80

ENTRYPOINT ["/bin/superphenix-api"]
