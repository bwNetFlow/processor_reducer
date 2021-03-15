package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	kafka "github.com/bwNetFlow/kafkaconnector"
	"github.com/bwNetFlow/processor_reducer/reducer"
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

	red := reducer.Reducer{
		LimitFields: *limitFields,
		AnonFields:  *anonFields,
	}

	// receive flows in a loop
	for {
		select {
		case original, ok := <-kafkaConn.ConsumerChannel():
			if !ok {
				log.Println("Reducer ConsumerChannel closed. Investigate this!")
				return
			}

			kafkaConn.ProducerChannel(*kafkaOutTopic) <- red.Process(original)
		case <-signals:
			return
		}
	}
}
