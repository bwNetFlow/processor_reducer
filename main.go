package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"

	kafka "github.com/bwNetFlow/kafkaconnector"
	flow "github.com/bwNetFlow/protobuf/go"
)

var (
	logFile = flag.String("log", "./processor_reducer.log", "Location of the log file.")

	kafkaBroker        = flag.String("kafka.brokers", "127.0.0.1:9092,[::1]:9092", "Kafka brokers list separated by commas")
	kafkaConsumerGroup = flag.String("kafka.consumer_group", "reducer_debug", "Kafka Consumer Group")
	kafkaInTopic       = flag.String("kafka.in.topic", "flow-messages", "Kafka topic to consume from")
	kafkaOutTopic      = flag.String("kafka.out.topic", "flows-messages-anon", "Kafka topic to produce to")

	kafkaUser        = flag.String("kafka.user", "", "Kafka username to authenticate with")
	kafkaPass        = flag.String("kafka.pass", "", "Kafka password to authenticate with")
	kafkaAuthAnon    = flag.Bool("kafka.auth_anon", false, "Set Kafka Auth Anon")
	kafkaDisableTLS  = flag.Bool("kafka.disable_tls", false, "Whether to use tls or not")
	kafkaDisableAuth = flag.Bool("kafka.disable_auth", false, "Whether to use auth or not")

	limitFields = flag.String("fields.limit", "Bytes,Packets,Etype,Proto", "Fields which will be kept")
	anonFields  = flag.String("fields.anon", "", "Fields which will be anonymized, if kept by fields.limit")
)

func main() {
	flag.Parse()
	var err error

	// initialize logger
	logfile, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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

	// disable TLS if requested
	if *kafkaDisableTLS {
		log.Println("kafkaDisableTLS ...")
		kafkaConn.DisableTLS()
	}
	if *kafkaDisableAuth {
		log.Println("kafkaDisableAuth ...")
		kafkaConn.DisableAuth()
	} else { // set Kafka auth
		if *kafkaAuthAnon {
			kafkaConn.SetAuthAnon()
		} else if *kafkaUser != "" {
			kafkaConn.SetAuth(*kafkaUser, *kafkaPass)
		} else {
			log.Println("No explicit credentials available, trying env.")
			err = kafkaConn.SetAuthFromEnv()
			if err != nil {
				log.Println("No credentials available, using 'anon:anon'.")
				kafkaConn.SetAuthAnon()
			}
		}
	}
	err = kafkaConn.StartConsumer(*kafkaBroker, []string{*kafkaInTopic}, *kafkaConsumerGroup, -1) // offset -1 is the most recent flow
	if err != nil {
		log.Println("StartConsumer:", err)
		// sleep to make auto restart not too fast and spamming connection retries
		time.Sleep(5 * time.Second)
		return
	}
	err = kafkaConn.StartProducer(*kafkaBroker)
	if err != nil {
		log.Println("StartProducer:", err)
		// sleep to make auto restart not too fast and spamming connection retries
		time.Sleep(5 * time.Second)
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
			for _, fieldname := range strings.Split(*limitFields, ",") {
				if fieldname == "" {
					log.Println("fields.limit unset, terminating...")
					return // case limitFields == ''
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
			for _, fieldname := range strings.Split(*anonFields, ",") {
				if fieldname == "" {
					continue // case anonFields == ''
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
			kafkaConn.ProducerChannel(*kafkaOutTopic) <- &reduced
		case <-signals:
			return
		}
	}
}
