package log

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"

	"github.com/TerrexTech/go-eventstore-models/model"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

// Logger provides convenient handling for log-messages.
type Logger interface {
	// D produces DEBUG logs, which will also produce INFO and ERROR.
	// Additional data provided will be marshalled and added to log-description.
	// If the data is one of EventStore-Models, the included data-elements, such as
	// "Data" in Event, "Result" and "Input" in KafkaResponse, are also converted
	// to plain-strings before log is produced.
	D(entry Entry, data ...interface{})
	// E produces ERROR logs which will discard INFO and DEBUG logs,
	// and produce only ERROR logs.
	E(entry Entry)
	// F produces ERROR logs which will discard INFO and DEBUG logs,
	// and produce only ERROR logs. This also exits the program using os.Exit after logging.
	F(entry Entry)
	// I produces INFO logs, which also include ERROR logs.
	// DEBUG logs are discarded from production.
	I(entry Entry)
	// DisableOutput disables writing to Output.
	// The logs are still sent to logsink. Output is enabled by default.
	DisableOutput()
	// EnableOutput enables writing to Output. This is the default.
	EnableOutput()
	// SetOutput sets the output to which the logs are written.
	// Default is Stdout.
	SetOutput(w io.Writer)
}

// Entry is a single log-entry.
type Entry struct {
	Description   string `json:"description,omitempty"`
	ErrorCode     int    `json:"errorCode,omitempty"`
	EventAction   string `json:"eventAction,omitempty"`
	ServiceAction string `json:"serviceAction,omitempty"`
	ServiceName   string `json:"serviceName,omitempty"`
}

// logger implements Logger interface
type logger struct {
	logChan      chan<- model.LogEntry
	enableOutput bool
	output       io.Writer
	svcName      string
}

func (l *logger) DisableOutput() {
	l.enableOutput = false
}

func (l *logger) EnableOutput() {
	l.enableOutput = true
}

func (l *logger) SetOutput(w io.Writer) {
	l.output = w
}

func (l *logger) D(entry Entry, data ...interface{}) {
	l.log(model.LogEntry{
		Description:   entry.Description,
		ErrorCode:     entry.ErrorCode,
		Level:         "DEBUG",
		EventAction:   entry.EventAction,
		ServiceAction: entry.ServiceAction,
		ServiceName:   entry.ServiceName,
	}, data...)
}

func (l *logger) E(entry Entry) {
	l.log(model.LogEntry{
		Description:   entry.Description,
		ErrorCode:     entry.ErrorCode,
		Level:         "ERROR",
		EventAction:   entry.EventAction,
		ServiceAction: entry.ServiceAction,
		ServiceName:   entry.ServiceName,
	})
}

func (l *logger) F(entry Entry) {
	l.log(model.LogEntry{
		Description:   entry.Description,
		ErrorCode:     entry.ErrorCode,
		Level:         "ERROR",
		EventAction:   entry.EventAction,
		ServiceAction: entry.ServiceAction,
		ServiceName:   entry.ServiceName,
	})
	os.Exit(1)
}

func (l *logger) I(entry Entry) {
	l.log(model.LogEntry{
		Description:   entry.Description,
		ErrorCode:     entry.ErrorCode,
		Level:         "INFO",
		EventAction:   entry.EventAction,
		ServiceAction: entry.ServiceAction,
		ServiceName:   entry.ServiceName,
	})
}
func (l *logger) log(entry model.LogEntry, data ...interface{}) {
	level := os.Getenv(LogLevelEnvVar)
	if level != "INFO" && level != "ERROR" && level != "DEBUG" && level != "NONE" {
		log.Println(
			"LogLevelEnvVar environment variable missing or set to invalid value. " +
				"Valid levels are: ERROR, INFO and DEBUG.",
		)
		log.Println("INFO level will be used")
		level = "INFO"
	}

	switch level {
	case "NONE":
		return
	case "INFO":
		if entry.Level == "DEBUG" {
			return
		}
	case "ERROR":
		if entry.Level != "ERROR" {
			return
		}
	}
	if entry.Level == "NONE" {
		log.Println(
			`LogENtry contains "NONE" Log-Level which is invalid. LogEntry will be ignored.`,
		)
		return
	}

	if entry.ServiceName == "" {
		entry.ServiceName = l.svcName
	}

	if level == "DEBUG" {
		desc, err := fmtDebug(entry.Description, data...)
		if err != nil {
			err = errors.Wrap(err, "Error while formatting log for Debug-level")
			log.Println(err)
			entry.Description += " " + err.Error()
		} else {
			entry.Description = desc
		}
	}

	if l.enableOutput {
		l.output.Write([]byte(entry.Description))
	}

	l.logChan <- entry
}

// fmtDebug adds the provided additional data to log-description if the level is DEBUG.
func fmtDebug(description string, data ...interface{}) (string, error) {
	if data == nil {
		return description, nil
	}

	outStr := "\n------------------------\n"
	outStr += description + "\n"

	for i, d := range data {
		if reflect.TypeOf(d).Kind() == reflect.Ptr {
			d = reflect.ValueOf(d).Elem().Interface()
		}

		outStr += fmt.Sprintf("==> Data %d: ", i)
		dd, err := fmtDebugData(d)
		if err != nil {
			err = errors.Wrapf(err, "Error while formatting DEBUG data at index: %d", i)
			return "", err
		}
		outStr += dd + "\n"
	}

	outStr += "------------------------\n"
	return outStr, nil
}

func fmtDebugData(d interface{}) (string, error) {
	if reflect.TypeOf(d).Kind() == reflect.Ptr {
		d = reflect.ValueOf(d).Elem().Interface()
	}

	switch t := d.(type) {
	case model.Event:
		eventData := string(t.Data)
		parsedData, ok := gjson.Parse(eventData).Value().(interface{})
		if !ok {
			parsedData = eventData
		}
		tm := map[string]interface{}{
			"aggregateID":   t.AggregateID,
			"eventAction":   t.EventAction,
			"serviceAction": t.ServiceAction,
			"correlationID": t.CorrelationID,
			"data":          parsedData,
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
		return fmt.Sprintf("%s:\n%s", reflect.TypeOf(t).String(), string(mm)), nil

	case model.KafkaResponse:
		krResult := string(t.Result)
		result, ok := gjson.Parse(krResult).Value().(interface{})
		if !ok {
			result = krResult
		}
		krInput := string(t.Input)
		input, ok := gjson.Parse(krInput).Value().(interface{})
		if !ok {
			input = krInput
		}
		tm := map[string]interface{}{
			"aggregateID":   t.AggregateID,
			"error":         t.Error,
			"errorCode":     t.ErrorCode,
			"topic":         t.Topic,
			"eventAction":   t.EventAction,
			"serviceAction": t.ServiceAction,
			"correlationID": t.CorrelationID,
			"result":        result,
			"input":         input,
			"uuid":          t.UUID.String(),
		}
		mm, err := json.Marshal(tm)
		if err != nil {
			err = errors.Wrap(err, "Error marshalling KafkaResponse")
			return "", err
		}
		return fmt.Sprintf("%s:\n%s", reflect.TypeOf(t).String(), string(mm)), nil

	case model.EventMeta:
		mm, err := json.Marshal(t)
		if err != nil {
			err = errors.Wrap(err, "Error marshalling EventMeta")
			return "", err
		}
		return "EventMeta:\n" + string(mm), nil

	case model.EventStoreQuery:
		mm, err := json.Marshal(t)
		if err != nil {
			err = errors.Wrap(err, "Error marshalling EventStoreQuery")
			return "", err
		}
		return fmt.Sprintf("%s:\n%s", reflect.TypeOf(t).String(), string(mm)), nil

	default:
		dataKind := reflect.ValueOf(d).Kind()
		switch dataKind {
		case reflect.Struct, reflect.Map:
			mm, err := json.Marshal(t)
			if err != nil {
				err = errors.Wrap(err, "Error marshalling Unknown Type")
				return "", err
			}
			return fmt.Sprintf("%s:\n%s", reflect.TypeOf(t).String(), string(mm)), nil

		case reflect.Slice, reflect.Array:
			dataType := reflect.TypeOf(t).String()
			outStr := fmt.Sprintf("%s:\n", dataType)

			v := reflect.ValueOf(t)
			for i := 0; i < v.Len(); i++ {
				outStr += fmt.Sprintf("=> Index %d: ", i)

				dd, err := fmtDebugData(v.Index(i).Interface())
				if err != nil {
					err = errors.Wrapf(err, `Error formatting %s at index: "%d"`, dataType, i)
					return "", err
				}
				outStr += dd + "\n"
			}
			return outStr, nil

		default:
			return fmt.Sprintf("%v", d), nil
		}
	}
}
