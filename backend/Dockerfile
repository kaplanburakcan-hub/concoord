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
USER ipks
WORKDIR /app
COPY --from=build /out/api /out/worker /app/
EXPOSE 8080
ENTRYPOINT ["/bin/sh", "-c", "/app/api -seed && /app/api"]