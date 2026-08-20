FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/controller ./cmd/controller
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agent ./cmd/agent
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/certgen ./cmd/certgen

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/controller /controller
COPY --from=build /out/agent /agent
COPY --from=build /out/certgen /certgen
EXPOSE 8080 8081
USER nonroot:nonroot
ENTRYPOINT ["/controller"]
