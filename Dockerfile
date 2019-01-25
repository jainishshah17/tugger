# Builder image
FROM golang:1.11.4-alpine as builder

# Install dependencies
RUN apk update && apk add --update gcc git musl-dev curl

# Set workspace
WORKDIR /src/jainishshah17/tugger/

# Copy source
COPY ./ /src/jainishshah17/tugger/

# Build microservices
RUN cd cmd/tugger && go install

# Runnable image
FROM alpine:3.8

# Install dependencies
RUN apk update && apk add --update git curl

# Copy microservice executable from builder image
COPY --from=builder /go/bin/tugger /go/bin/tugger

# Directory for tls
RUN mkdir -p /etc/admission-controller/tls

# Set Entrypoint
CMD ["/go/bin/tugger"]