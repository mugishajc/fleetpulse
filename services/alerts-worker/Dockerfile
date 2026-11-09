FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/alerts-worker ./cmd/alerts-worker

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/alerts-worker /alerts-worker
USER nonroot:nonroot
EXPOSE 9090
ENTRYPOINT ["/alerts-worker"]
