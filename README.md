reducer
==

This is a small Kafka Processor, i.e. a Consumer and a Producer, which supports
reducing Protobuf messages to a specific subset of fields. The intention is to
enable a Kafka topic of anonymous Flow messages within the bwNetFlow project.
The default subset of fields this Processor limits Flow messages to reflects
this: `Bytes,Packets,Etype,Proto`

It also supports some experimental, subnet-based anonymisation.

Usage
====
First, have the proper environment variables set for authenticating with your Kafka Cluster:
`KAFKA_SASL_USER=reducer KAFKA_SASL_PASS=somesecurepass`

As standalone command:
`./reducer --kafka.brokers "broker01:9093,broker02:9093" --kafka.in.topic flow-messages --kafka.out.topic flow-messages-anon --kafka.consumer_group reducer-dev`

Using Systemd:
```
[Unit]
Description=Default kafka-processor Reducer process
After=network.target

[Service]
EnvironmentFile=/opt/kafka-processor-reducer/config/authdata
User=kafka-processor-reducer
WorkingDirectory=/opt/kafka-processor-reducer
ExecStart=/opt/kafka-processor-reducer/reducer --kafka.brokers "broker01:9093,broker02:9093" --kafka.in.topic flow-messages --kafka.out.topic flow-messages-anon --kafka.consumer_group processor-reducer
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

TODOs
====

 * make anonymisation work better, provide different patterns
