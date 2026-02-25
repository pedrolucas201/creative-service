FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /bin/api /bin/api
COPY --from=build /app/internal/storage/migrations /app/internal/storage/migrations
EXPOSE 8080
CMD ["/bin/api"]
