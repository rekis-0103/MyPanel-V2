# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/controller ./cmd/controller && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agent ./cmd/agent && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/certgen ./cmd/certgen

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/controller /controller
COPY --from=build /out/agent /agent
COPY --from=build /out/certgen /certgen
EXPOSE 8080 8081
USER nonroot:nonroot
ENTRYPOINT ["/controller"]
