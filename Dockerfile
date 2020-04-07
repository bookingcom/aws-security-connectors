# Build
FROM golang:1 as build

ADD . /app
WORKDIR /app

RUN go test ./...

RUN CGO_ENABLED=0 GOOS=linux go build -o aws-security-connectors .

# Run
FROM alpine

RUN apk add --update ca-certificates && update-ca-certificates
RUN adduser -s /bin/false -S connectors

COPY --from=build /app/aws-security-connectors /usr/bin

USER connectors

ENTRYPOINT ["/usr/bin/aws-security-connectors"]