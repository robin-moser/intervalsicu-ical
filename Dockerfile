FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY *.go ./

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o intervals-calendar .

FROM alpine:3.21

RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /app/intervals-calendar /usr/local/bin/intervals-calendar

ENTRYPOINT ["intervals-calendar"]
