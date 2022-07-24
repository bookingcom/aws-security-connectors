# Build
FROM golang:1 as build

ADD . /app
WORKDIR /app

RUN go test ./...

RUN CGO_ENABLED=0 GOOS=linux go build -o aws-security-connectors .

# Run
FROM alpine

LABEL org.opencontainers.image.authors="Dmitry Verkhoturov <paskal.07@gmail.com>" \
      org.opencontainers.image.description="AWS Security Connectors" \
      org.opencontainers.image.documentation="https://github.com/bookingcom/aws-security-connectors" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.source="https://github.com/bookingcom/aws-security-connectors.git" \
      org.opencontainers.image.title="aws-security-connectors" \
      org.opencontainers.image.vendor="Booking.com"

RUN apk add --update ca-certificates && update-ca-certificates
RUN adduser -s /bin/false -S connectors

COPY --from=build /app/aws-security-connectors /usr/bin

USER connectors

ENTRYPOINT ["/usr/bin/aws-security-connectors"]