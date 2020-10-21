FROM golang:1.15 AS builder

# Add your files into the container
ADD . /opt/build
WORKDIR /opt/build

# build the binary
RUN CGO_ENABLED=0 go build -o reducer -v
FROM alpine:3.12
WORKDIR /

# COPY binary from previous stegae to your desired location
COPY --from=builder /opt/build/reducer .
ENTRYPOINT /reducer --kafka.brokers=${KAFKA_BROKERS} --kafka.in.topic=${KAFKA_IN_TOPIC} --kafka.out.topic=${KAFKA_OUT_TOPIC} --kafka.consumer_group=${CONSUMER_GROUP} --kafka.disable_auth=${DISABLE_AUTH} --kafka.disable_tls=${DISABLE_TLS} --kafka.auth_anon=${AUTH_ANON}
