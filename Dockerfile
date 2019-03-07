FROM golang:1 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /sbapi -ldflags "-w -s" ./sbapi \
    && go build -o /twchat -ldflags "-w -s" ./twchat \
    && go build -o /gann -ldflags "-w -s" ./gann

FROM alpine
RUN apk add --no-cache ca-certificates
RUN adduser -D thorium-salty && adduser -D thorium-twitch && adduser -D thorium-gann
COPY --from=builder /sbapi /twchat /gann /usr/bin/
