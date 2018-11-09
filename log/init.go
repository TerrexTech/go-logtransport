package log

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/TerrexTech/go-eventstore-models/model"
	"github.com/TerrexTech/go-kafkautils/kafka"
	"github.com/pkg/errors"
)

// LogLevelEnvVar is the environment-variable from which the log-level is read.
const LogLevelEnvVar = "LOG_LEVEL"

// Init creates a new Logger for handling log-messages.
func Init(
	ctx context.Context,
	// svcName is the default ServiceName to be used
	// when ServiceName is not provided in LogEntry model.
	svcName string,
	config *kafka.ProducerConfig,
	topic string,
) (Logger, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if topic == "" {
		return nil, errors.New("empty svcName provided")
	}
	if config == nil {
		return nil, errors.New("nil config provided")
	}
	if topic == "" {
		return nil, errors.New("empty topic provided")
	}

	producer, err := kafka.NewProducer(config)
	if err != nil {
		err = errors.Wrap(err, "Error creating LogTransport-Producer")
		return nil, err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				prodErr := errors.New("LogTransport-Producer: context closed")
				log.Println(prodErr)
				return
			case err := <-producer.Errors():
				if err != nil && err.Err != nil {
					parsedErr := errors.Wrap(err.Err, "Error in LogTransport-Producer")
					log.Println(parsedErr)
					log.Println(err)
				}
			}
		}
	}()

	logChan := make(chan model.LogEntry, 256)
	closeProducer := false
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("LogTransport: context closed")
				closeProducer = true
				err := producer.Close()
				if err != nil {
					err = errors.Wrap(err, "Error closing LogTransport-Producer")
					log.Println(err)
				}
				log.Println("--> Closed log-transporter")

			case l := <-logChan:
				ml, err := json.Marshal(l)
				if err != nil {
					err = errors.Wrap(err, "Error marshalling log-entry")
					log.Println(err)
				}
				msg := kafka.CreateMessage(topic, ml)
				if !closeProducer {
					producer.Input() <- msg
				}
			}
		}
	}()

	return &logger{
		arrThreshold: 15,
		logChan:      (chan<- model.LogEntry)(logChan),
		enableOutput: true,
		output:       os.Stdout,
		svcName:      svcName,
	}, nil
}
