package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"time"

	"github.com/TerrexTech/go-eventstore-models/model"
	"github.com/pkg/errors"
)

// Logger provides convenient handling for log-messages.
// Additional data can be provided to log-levels and will be marshalled and added to log.
// If the data is one of EventStore-Models, the included data-elements, such as
// "Data" in Event, "Result" and "Input" in KafkaResponse, are also attempted to be
// parsed and converted to JSON before the log is produced.
// DEBUG is most performance-intensive level, and should only be used for development.
type Logger interface {
	// D produces DEBUG logs, which will also produce INFO and ERROR.
	D(entry Entry, data ...interface{})
	// E produces ERROR logs which will discard INFO and DEBUG logs,
	// and produce only ERROR logs.
	E(entry Entry, data ...interface{})
	// F produces ERROR logs which will discard INFO and DEBUG logs,
	// and produce only ERROR logs. This also exits the program using os.Exit after logging.
	F(entry Entry, data ...interface{})
	// I produces INFO logs, which also include ERROR logs.
	// DEBUG logs are discarded from production.
	I(entry Entry, data ...interface{})
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

	svcName       string
	eventAction   string
	serviceAction string
}

func (l *logger) SetEventAction(action string) {
	l.eventAction = action
}

func (l *logger) SetServiceAction(action string) {
	l.serviceAction = action
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

func (l *logger) E(entry Entry, data ...interface{}) {
	l.log(model.LogEntry{
		Description:   entry.Description,
		ErrorCode:     entry.ErrorCode,
		Level:         "ERROR",
		EventAction:   entry.EventAction,
		ServiceAction: entry.ServiceAction,
		ServiceName:   entry.ServiceName,
	})
}

func (l *logger) F(entry Entry, data ...interface{}) {
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

func (l *logger) I(entry Entry, data ...interface{}) {
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
	invalidConfig := false
	if level != "INFO" && level != "ERROR" && level != "DEBUG" && level != "NONE" {
		invalidConfig = true
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
	if entry.EventAction == "" {
		entry.EventAction = l.eventAction
	}
	if entry.ServiceAction == "" {
		entry.ServiceAction = l.serviceAction
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
		if invalidConfig {
			l.output.Write([]byte(
				"LogLevelEnvVar environment variable missing or set to invalid value. " +
					"Valid levels are: ERROR, INFO and DEBUG. " + "INFO level will be used.",
			))
		}
		l.output.Write([]byte(entry.Description))
	}

	l.logChan <- entry
}

// fmtDebug adds the provided additional data to log-description if the level is DEBUG.
func fmtDebug(description string, data ...interface{}) (string, error) {
	now := time.Now()
	year, month, day := now.Date()
	hour, min, sec := now.Clock()
	timeFMT := fmt.Sprintf(
		"%02d/%02d/%02d %02d:%02d:%02d",
		year, month, day, hour, min, sec,
	)
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = -1
	}
	description = fmt.Sprintf("%s %s:%d:\n%s", timeFMT, file, line, description)

	if data == nil {
		return description + "\n", nil
	}

	outStr := "\n========================\n"
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
		outStr += "--------------\n"
	}

	outStr += "========================\n"
	return outStr, nil
}
