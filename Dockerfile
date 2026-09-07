# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/bookdb ./cmd/bookdb

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/bookdb /usr/local/bin/bookdb
COPY config/ ./config/

EXPOSE 8080 9090

ENTRYPOINT ["bookdb"]
CMD ["api"]