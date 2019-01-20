# Builder image
FROM golang:1.11.4-alpine

# Install dependencies
RUN apk update && apk add --update gcc git musl-dev curl

# Set workspace
WORKDIR /src/jainishshah17/tugger/

# Copy source
COPY ./ /src/jainishshah17/tugger/

# Build microservices
RUN cd cmd/tugger && go install

# Runnable image
FROM golang:1.11.4-alpine

# Set vars
# ENV TUGGER_USER_NAME=caddyshack \
#    TUGGER_USER_ID=1001

# Install dependencies
RUN apk update && apk add --update gcc git musl-dev curl

# Create user and create needed directories
# RUN addgroup -g ${TUGGER_USER_ID} -S ${TUGGER_USER_NAME} && \
#    adduser -u ${TUGGER_USER_ID} -S ${TUGGER_USER_NAME} -G ${TUGGER_USER_NAME}

# Copy microservice executable from builder image
COPY --from=0 /go/bin/tugger /go/bin/tugger

RUN mkdir -p /etc/admission-controller/tls
# RUN chown -R ${TUGGER_USER_NAME}:${TUGGER_USER_NAME} /src/jainishshah17/tugger/

# The user that will run the container
# USER ${TUGGER_USER_NAME}

# Set Entrypoint
CMD ["/go/bin/tugger"]