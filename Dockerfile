# Builder image
FROM golang:1.20.2 as builder

# Set workspace
WORKDIR /src/jainishshah17/tugger/

# Copy source
COPY ./ /src/jainishshah17/tugger/

# Build microservices
RUN cd cmd/tugger && go install

# Runnable image
FROM gcr.io/distroless/base-debian11

# Copy microservice executable from builder image
COPY --from=builder /go/bin/tugger /

# Set Entrypoint
CMD ["/tugger"]
