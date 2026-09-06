FROM golang:1.26-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/trippy-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/trippy-migrate ./cmd/migrate

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/trippy-server /usr/local/bin/trippy-server
COPY --from=build /out/trippy-migrate /usr/local/bin/trippy-migrate
COPY migrations ./migrations

EXPOSE 8080

CMD ["sh", "-c", "trippy-migrate -dir /app/migrations && exec trippy-server"]
