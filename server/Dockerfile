FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/burn-server .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && mkdir -p /data
COPY --from=build /out/burn-server /usr/local/bin/burn-server
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["burn-server"]
