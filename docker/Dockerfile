FROM golang:1.25-alpine AS build

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /kerobot ./cmd/kerobot

FROM alpine:3.20

RUN adduser -D -g '' kerobot

WORKDIR /app

COPY --from=build /kerobot /usr/local/bin/kerobot
COPY migrations ./migrations

ENV TZ=UTC
USER kerobot

CMD ["kerobot"]
