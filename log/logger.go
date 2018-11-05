package log

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"

	"github.com/TerrexTech/go-eventstore-models/model"
	"github.com/pkg/errors"
)

// Logger provides convenient interface for logging messages.
type Logger struct {
	logChan chan<- model.LogEntry
	svcName string
}

// Log enhances the provided LogEntry, filters it based on LogLevelEnvVar,
// and produces the log using Kafka-Producer.
// LogLevelEnvVar is an environment variable and can contain possible values from DEBUG, ERROR, INFO, NONE.
// If the environment variable is unset or is not one of above, INFO level is used.
// Following are preferences for each log-level:
//  * DEBUG: Produces INFO and ERROR, and also produces provided data.
//    If the provides data is one of EventStore-Models, its included data parts such as
//    "Data" in Event, and "Result" and "Input" in KafkaResponse, are also converted
//    to string before being produced.
//  * ERROR: Produces only ERROR log-levels.
//  * INFO: Produces INFO and ERROR log-levels.
//  * NONE: Doesn't produce any log-level.
func (l *Logger) Log(entry model.LogEntry, data ...interface{}) {
	level := os.Getenv(LogLevelEnvVar)
	if level != "INFO" && level != "ERROR" && level != "DEBUG" && level != "NONE" {
		log.Println(
			"LogLevelEnvVar environment variable missing or set to invalid value. " +
				"Valid levels are: ERROR, INFO and DEBUG.",
		)
		log.Println("INFO level will be used")
		level = "INFO"
	}

	if level == "NONE" {
		return
	}
	// Only INFO and ERROR logs
	if level == "INFO" && entry.Level == "DEBUG" {
		return
	}
	// Only ERROR logs
	if level == "ERROR" && entry.Level != "ERROR" {
		return
	}

	if entry.ServiceName == "" {
		entry.ServiceName = l.svcName
	}

	if level == "DEBUG" {
		desc, err := l.fmtDebug(entry.Description, data...)
		if err != nil {
			err = errors.Wrap(err, "Error while formatting log for Debug-level")
			log.Println(err)
			entry.Description += " " + err.Error()
		} else {
			entry.Description = desc
		}
	}

	l.logChan <- entry
}

func (*Logger) fmtDebug(description string, data ...interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	outStr := "\n------------------------\n"
	outStr += description + "\n"

	for i, d := range data {
		if reflect.TypeOf(d).Kind() == reflect.Ptr {
			d = reflect.ValueOf(d).Elem().Interface()
		}

		outStr += fmt.Sprintf("==> Data %d: ", i)
		switch t := d.(type) {
		case model.Event:
			tm := map[string]interface{}{
				"aggregateID":   t.AggregateID,
				"eventAction":   t.EventAction,
				"serviceAction": t.ServiceAction,
				"correlationID": t.CorrelationID,
				"data":          string(t.Data),
				"nanoTime":      t.NanoTime,
				"userUUID":      t.UserUUID.String(),
				"uuid":          t.UUID.String(),
				"version":       t.Version,
				"yearBucket":    t.YearBucket,
			}
			mm, err := json.Marshal(tm)
			if err != nil {
				err = errors.Wrap(err, "Error marshalling Event")
				return "", err
			}
			outStr += "Event: \n" + string(mm)

		case model.KafkaResponse:
			tm := map[string]interface{}{
				"aggregateID":   t.AggregateID,
				"error":         t.Error,
				"errorCode":     t.ErrorCode,
				"topic":         t.Topic,
				"eventAction":   t.EventAction,
				"serviceAction": t.ServiceAction,
				"correlationID": t.CorrelationID,
				"result":        string(t.Result),
				"input":         string(t.Input),
				"uuid":          t.UUID.String(),
			}
			mm, err := json.Marshal(tm)
			if err != nil {
				err = errors.Wrap(err, "Error marshalling KafkaResponse")
				return "", err
			}
			outStr += "KafkaResponse: \n" + string(mm)

		case model.EventMeta:
			mm, err := json.Marshal(t)
			if err != nil {
				err = errors.Wrap(err, "Error marshalling EventMeta")
				return "", err
			}
			outStr += "EventMeta: \n" + string(mm)

		case model.EventStoreQuery:
			mm, err := json.Marshal(t)
			if err != nil {
				err = errors.Wrap(err, "Error marshalling EventStoreQuery")
				return "", err
			}
			outStr += "EventStoreQuery: \n" + string(mm)

		default:
			dataKind := reflect.ValueOf(d).Kind()
			if dataKind == reflect.Struct || dataKind == reflect.Map {
				mm, err := json.Marshal(t)
				if err != nil {
					err = errors.Wrap(err, "Error marshalling Unknown Type")
					return "", err
				}
				outStr += fmt.Sprintf("Unknown: %s:\n%s", dataKind.String(), string(mm))
			} else {
				outStr += fmt.Sprintf("%v", d)
			}
		}
		outStr += "\n"
	}

	outStr += "------------------------\n"
	return outStr, nil
}
