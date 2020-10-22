# Reducer

This is the Reducer component of the [bwNetFlow][bwNetFlow] platform. It
supports taking protobuf-encoded [flow messages][protobuf] from a specified
Kafka topic, reducing the message to some specified minimum, and writing the
result back into another Kafka topic.

The intention is to enable a Kafka topic of anonymous Flow messages within the
bwNetFlow project. The default subset of fields this processor limits flow
messages to are just `Bytes`, `Packets`, `Etype`, and `Proto` of the flow,
which can be considered fully anonymous and is quite sufficient to demonstrate
the API.

It also supports some experimental, subnet-based anonymisation.

## Usage

The simplest call could look like this, which would start the reducer process
with TLS encryption and SASL auth enabled and all outputs working.

```
export KAFKA_SASL_USER=prod-reducer
export KAFKA_SASL_PASS=somesecurepass`
./reducer \
        --kafka.brokers=kafka.local:9093 \
        --kafka.in.topic=flows-enriched \
        --kafka.out.topic=flows-anon \
        --kafka.consumer_group=reducer-prod
```

Check `--help` for a full list of options and also see our Dockerfile for some
more examples.

[bwNetFlow]: https://github.com/bwNetFlow/bwNetFlow
[protobuf]: https://github.com/bwNetFlow/protobuf
