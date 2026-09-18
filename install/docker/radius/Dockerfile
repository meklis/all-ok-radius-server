# Собирается с контекстом = корень репозитория (см. install/docker/docker-compose.yml):
# docker build -f install/docker/radius/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/all-ok-radius-server ./server

FROM alpine:3.20
LABEL maintainer="Max Boyar <max.boyar.a@gmail.com>"
# ca-certificates - для HTTPS-запросов к script.database (clientdb) и api.addresses
RUN apk add --no-cache ca-certificates
COPY --from=build /out/all-ok-radius-server /opt/all-ok-radius-server
COPY --from=build /src/script/examples /opt/scripts
COPY install/docker/radius/radius.conf.yml /opt/radius.conf.yml
ENTRYPOINT ["/opt/all-ok-radius-server", "-c", "/opt/radius.conf.yml"]
