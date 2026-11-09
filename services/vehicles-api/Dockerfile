FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/vehicles-api ./cmd/vehicles-api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/vehicles-api /vehicles-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/vehicles-api"]
