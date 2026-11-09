FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/drivers-service ./cmd/drivers-service

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/drivers-service /drivers-service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/drivers-service"]
