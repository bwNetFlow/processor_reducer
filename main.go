package main

import (
	"flag"
	kafka "github.com/bwNetFlow/kafkaconnector"
	flow "github.com/bwNetFlow/protobuf/go"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strings"
)

var (
	LogFile = flag.String("log", "./processor_reducer.log", "Location of the log file.")

	KafkaConsumerGroup = flag.String("kafka.consumer_group", "reducer_debug", "Kafka Consumer Group")
	KafkaInTopic       = flag.String("kafka.in.topic", "flow-messages", "Kafka topic to consume from")
	KafkaOutTopic      = flag.String("kafka.out.topic", "flows-messages-anon", "Kafka topic to produce to")
	KafkaBroker        = flag.String("kafka.brokers", "127.0.0.1:9092,[::1]:9092", "Kafka brokers list separated by commas")
	LimitFields        = flag.String("fields.limit", "Bytes,Packets,Etype,Proto", "Fields which will be kept")
	AnonFields         = flag.String("fields.anon", "", "Fields which will be anonymized, if kept by fields.limit")
)

func main() {
	flag.Parse()
	var err error

	// initialize logger
	logfile, err := os.OpenFile(*LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		println("Error opening file for logging: %v", err)
		return
	}
	defer logfile.Close()
	mw := io.MultiWriter(os.Stdout, logfile)
	log.SetOutput(mw)
	log.Println("-------------------------- Started.")

	// trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// connect to the Kafka cluster using bwNetFlow/kafkaconnector
	var kafkaConn = kafka.Connector{}
	err = kafkaConn.SetAuthFromEnv()
	if err != nil {
		log.Println(err)
		log.Println("Resuming as user anon.")
		kafkaConn.SetAuthAnon()
	}
	err = kafkaConn.StartConsumer(*KafkaBroker, []string{*KafkaInTopic}, *KafkaConsumerGroup, -1) // offset -1 is the most recent flow
	if err != nil {
		log.Println("StartConsumer:", err)
		return
	}
	err = kafkaConn.StartProducer(*KafkaBroker)
	if err != nil {
		log.Println("StartProducer:", err)
		return
	}
	defer kafkaConn.Close()

	// receive flows in a loop
	for {
		select {
		case original, ok := <-kafkaConn.ConsumerChannel():
			if !ok {
				log.Println("Reducer ConsumerChannel closed. Investigate this!")
				return
			}

			reflected_original := reflect.ValueOf(original) // immutable
			reduced := flow.FlowMessage{}
			reflected_reduced := reflect.ValueOf(&reduced) // mutable
			// limit stuff
			for _, fieldname := range strings.Split(*LimitFields, ",") {
				if fieldname == "" {
					log.Println("fields.limit unset, terminating...")
					return // case LimitFields == ''
				}
				original_field := reflect.Indirect(reflected_original).FieldByName(fieldname)
				reduced_field := reflected_reduced.Elem().FieldByName(fieldname)
				if original_field.IsValid() && reduced_field.IsValid() {
					reduced_field.Set(original_field)
				} else {
					log.Printf("Flow messages do not have a field named '%s'", fieldname)
				}
			}
			// anon stuff
			for _, fieldname := range strings.Split(*AnonFields, ",") {
				if fieldname == "" {
					continue // case AnonFields == ''
				}
				reduced_field := reflected_reduced.Elem().FieldByName(fieldname)
				if reduced_field.IsValid() {
					if reduced_field.Type() == reflect.TypeOf([]uint8{}) {
						raw := reduced_field.Interface().([]uint8)
						address := net.IP(raw)
						raw[len(raw)-1] = 0
						if address.To4() == nil {
							for i := 2; i <= 8; i++ {
								raw[len(raw)-i] = 0
							}
						}
						reduced_field.Set(reflect.ValueOf(raw))
					} else {
						log.Printf("Field '%s' has type '%s'. Anonymization is only supported for IP types.", fieldname, reduced_field.Type())
					}
				} else {
					log.Printf("The reduced flow message did not have a field named '%s' to anonymize.", fieldname)
				}
			}
			kafkaConn.ProducerChannel(*KafkaOutTopic) <- &reduced
		case <-signals:
			return
		}
	}
}
