package log

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/TerrexTech/go-common-models/model"
	"github.com/TerrexTech/go-commonutils/commonutil"
)

// TODO: Refactor the formatting-code and improve tests.

// fmtDebug adds the provided additional data to log-description if the level is DEBUG.
func fmtDebug(
	description string, arrThreshold int, data ...interface{},
) (string, error) {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		file = "???"
		line = -1
	}
	outStr := fmt.Sprintf("%s:%d: ===> %s", file, line, description)

	if data == nil {
		outStr += "\n========================"
		return outStr, nil
	}

	outStr += "\n========================\n"

	now := time.Now()
	year, month, day := now.Date()
	hour, min, sec := now.Clock()
	timeFMT := fmt.Sprintf(
		"%02d/%02d/%02d %02d:%02d:%02d",
		year, month, day, hour, min, sec,
	)
	outStr = fmt.Sprintf("\n%s %s", timeFMT, outStr)

	for i, d := range data {
		outStr += "--------------\n"
		if reflect.TypeOf(d).Kind() == reflect.Ptr {
			d = reflect.ValueOf(d).Elem().Interface()
		}

		outStr += fmt.Sprintf("==> Data %d: ", i)
		dd := fmtDebugData(d, arrThreshold)
		outStr += dd + "\n"
	}
	outStr += "--------------\n"
	outStr += "========================"
	return outStr, nil
}

// fmtDebugData performs left-fold on data and gets results from fmtKnownTypes.
func fmtDebugData(d interface{}, arrThreshold int) string {
	dataType := reflect.TypeOf(d)
	if dataType.Kind() == reflect.Ptr {
		d = reflect.ValueOf(d).Elem().Interface()
	}

	dataKind := reflect.ValueOf(d).Kind()
	switch dataKind {
	case reflect.Slice, reflect.Array:
		outStr := fmt.Sprintf("%s:\n", dataType.String())

		v := reflect.ValueOf(d)
		arrLength := v.Len()

		if arrThreshold > 0 && arrLength > arrThreshold {
			outStr = fmt.Sprintf(
				"Array (length: %d) exceeds array-length threshold of %d.\n",
				arrLength,
				arrThreshold,
			)
		}

		if arrLength > arrThreshold {
			arrLength = arrThreshold
		}
		for i := 0; i < arrLength; i++ {
			outStr += "-----\n"
			outStr += fmt.Sprintf("=> Index %d: ", i)

			dd := fmtDebugData(v.Index(i).Interface(), arrThreshold)
			outStr += dd + "\n"
		}
		outStr += "-----"
		return outStr

	default:
		result := fmtKnownTypes(d, arrThreshold)
		mr, err := json.Marshal(result)
		if err != nil {
			return fmt.Sprintf("%s:\n%+v", dataType.String(), d)
		}
		return fmt.Sprintf("%s:\n%s", dataType.String(), string(mr))
	}
}

// fmtKnownTypes check if data-type is one of common-models,
// and tries to parse nested Data if it is.
func fmtKnownTypes(d interface{}, arrThreshold int) interface{} {
	if reflect.TypeOf(d).Kind() == reflect.Ptr {
		d = reflect.ValueOf(d).Elem().Interface()
	}

	switch t := d.(type) {
	case model.Command:
		dd := fmtKnownTypes(parseESModels(t.Data, arrThreshold), arrThreshold)
		tm := map[string]interface{}{
			"action":        t.Action,
			"correlationId": t.CorrelationID,
			"data":          dd,
			"responseTopic": t.ResponseTopic,
			"source":        t.Source,
			"sourceTopic":   t.SourceTopic,
			"timestamp":     t.Timestamp,
			"ttlSec":        t.TTLSec,
			"uuid":          t.UUID,
		}
		return tm

	case model.Document:
		dd := fmtKnownTypes(parseESModels(t.Data, arrThreshold), arrThreshold)
		tm := map[string]interface{}{
			"correlationId": t.CorrelationID,
			"data":          dd,
			"error":         t.Error,
			"errorCode":     t.ErrorCode,
			"source":        t.Source,
			"topic":         t.Topic,
			"uuid":          t.UUID,
		}
		return tm

	case model.Event:
		dd := fmtKnownTypes(parseESModels(t.Data, arrThreshold), arrThreshold)
		tm := map[string]interface{}{
			"action":        t.Action,
			"aggregateID":   t.AggregateID,
			"correlationID": t.CorrelationID,
			"data":          dd,
			"nanoTime":      t.NanoTime,
			"source":        t.Source,
			"userUUID":      t.UserUUID,
			"uuid":          t.UUID,
			"version":       t.Version,
			"year":          t.YearBucket,
		}
		return tm

	default:
		return t
	}
}

// parseESModels tries to parse provided json-bytes
// to a Common-Model (from go-common-models package).
func parseESModels(jsonBytes []byte, arrThreshold int) interface{} {
	m := map[string]interface{}{}
	err := json.Unmarshal(jsonBytes, &m)
	if err != nil {
		arr := []interface{}{}
		err = json.Unmarshal(jsonBytes, &arr)
		if err != nil || len(arr) == 0 {
			return string(jsonBytes)
		}

		results := []interface{}{}
		for _, v := range arr {
			dd := fmtKnownTypes(v, arrThreshold)
			results = append(results, dd)
		}
		return results
	}

	doc := &model.Document{}
	cmd := &model.Command{}
	event := &model.Event{}

	docTags := getTags(doc)
	cmdTags := getTags(cmd)
	eventTags := getTags(event)

	isDoc := true
	isCmd := true
	isEvent := true

	containsKey := commonutil.IsElementInSlice
	for k := range m {
		if isDoc && !containsKey(docTags, k) {
			isDoc = false
		}
		if isCmd && !containsKey(cmdTags, k) {
			isCmd = false
		}
		if isEvent && !containsKey(eventTags, k) {
			isEvent = false
		}
	}

	// Start from smallest struct, and validate till largest
	switch true {
	case isDoc:
		err := json.Unmarshal(jsonBytes, doc)
		if err == nil {
			return doc
		}

	case isCmd:
		err := json.Unmarshal(jsonBytes, cmd)
		if err == nil {
			return cmd
		}

	case isEvent:
		err := json.Unmarshal(jsonBytes, event)
		if err == nil {
			return event
		}
	}

	fmtData := fmtKnownTypes(m, arrThreshold)
	return fmtData
}

// getTags gets json-tag field-names from a struct
func getTags(s interface{}) []string {
	structType := reflect.ValueOf(s).Elem().Type()
	tags := []string{}
	for i := 0; i < structType.NumField(); i++ {
		fieldTags := structType.Field(i).Tag.Get("json")
		// Extract the name from the bson tag
		tagName := strings.Split(fieldTags, ",")[0]
		tags = append(tags, tagName)
	}
	return tags
}
