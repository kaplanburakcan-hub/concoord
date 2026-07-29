# ---- migrate binary ----
FROM migrate/migrate:v4.17.1 AS migrate

# ---- build ----
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

# ---- runtime ----
FROM alpine:3.20
RUN adduser -D -u 10001 ipks && apk add --no-cache ca-certificates curl tzdata
COPY --from=migrate /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=build /out/api /out/worker /app/
COPY migrations /app/migrations
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh
USER ipks
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/start.sh"]