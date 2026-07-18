# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /mon64 ./cmd/mon64

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /mon64 /usr/local/bin/mon64
EXPOSE 8080
ENTRYPOINT ["mon64"]
CMD ["-config", "/config.yaml"]
