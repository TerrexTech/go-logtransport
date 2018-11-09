package log

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/TerrexTech/go-commonutils/commonutil"
	"github.com/TerrexTech/go-eventstore-models/model"
	"github.com/pkg/errors"
)

// fmtDebugData attempts to create a human-readable formatted string from provided data.
func fmtDebugData(d interface{}) (string, error) {
	if reflect.TypeOf(d).Kind() == reflect.Ptr {
		d = reflect.ValueOf(d).Elem().Interface()
	}

	switch t := d.(type) {
	case model.Event:
		tm := map[string]interface{}{
			"aggregateID":   t.AggregateID,
			"eventAction":   t.EventAction,
			"serviceAction": t.ServiceAction,
			"correlationID": t.CorrelationID,
			"data":          parseESModels(t.Data),
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
		tm := map[string]interface{}{
			"aggregateID":   t.AggregateID,
			"error":         t.Error,
			"errorCode":     t.ErrorCode,
			"topic":         t.Topic,
			"eventAction":   t.EventAction,
			"serviceAction": t.ServiceAction,
			"correlationID": t.CorrelationID,
			"result":        parseESModels(t.Result),
			"input":         parseESModels(t.Input),
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

// parseESModels tries to parse provie json-bytes to an EventStore-Model.
func parseESModels(jsonBytes []byte) interface{} {
	m := map[string]interface{}{}
	err := json.Unmarshal(jsonBytes, &m)
	if err != nil {
		return string(jsonBytes)
	}

	em := &model.EventMeta{}
	esQuery := &model.EventStoreQuery{}
	kr := &model.KafkaResponse{}
	event := &model.Event{}

	emTags := getTags(em)
	esQueryTags := getTags(esQuery)
	krTags := getTags(kr)
	eventTags := getTags(event)

	isEM := true
	isESQuery := true
	isKR := true
	isEvent := true

	containsKey := commonutil.IsElementInSlice
	for k := range m {
		if isEM && !containsKey(emTags, k) {
			isEM = false
		}
		if isESQuery && !containsKey(esQueryTags, k) {
			isESQuery = false
		}
		if isKR && !containsKey(krTags, k) {
			isKR = false
		}
		if isEvent && !containsKey(eventTags, k) {
			isEvent = false
		}
	}

	// Start from smallest struct, and validate till largest
	switch true {
	case isEM:
		err := json.Unmarshal(jsonBytes, em)
		if err == nil {
			return em
		}

	case isESQuery:
		err := json.Unmarshal(jsonBytes, esQuery)
		if err == nil {
			return esQuery
		}

	case isKR:
		err := json.Unmarshal(jsonBytes, kr)
		if err == nil {
			return kr
		}

	case isEvent:
		err := json.Unmarshal(jsonBytes, event)
		if err == nil {
			return event
		}
	}

	return string(jsonBytes)
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
