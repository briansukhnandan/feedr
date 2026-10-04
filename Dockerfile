FROM golang:1.24-alpine AS build

ENV PATH=/usr/local/go/bin:$PATH

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/feedr ./cmd/feedr

FROM alpine:3.22

COPY --from=build /out/feedr /usr/local/bin/feedr
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV FEEDR_HOME=/feedr
VOLUME ["/feedr"]
ENTRYPOINT ["/usr/local/bin/feedr"]
