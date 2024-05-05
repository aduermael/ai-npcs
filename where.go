package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Operator string

const (
	Equal              Operator = "$eq"  // string, int, float
	NotEqual           Operator = "$ne"  // string, int, float
	GreaterThan        Operator = "$gt"  // int, float
	GreaterThanOrEqual Operator = "$gte" // int, float
	LessThan           Operator = "$lt"  // int, float
	LessThanOrEqual    Operator = "$lte" // int, float
	In                 Operator = "$in"  // int, float
	NotIn              Operator = "nin"
)

type Where struct{}

type WhereField struct {
	Where
	Name     string
	Operator Operator
	Value    any
}

func (w WhereField) MarshalJSON() ([]byte, error) {

	valueString, ok := w.Value.(string)
	if ok {
		return []byte(fmt.Sprintf("{ \"%s\": { \"%s\": \"%s\" }}", w.Name, w.Operator, valueString)), nil
	}

	valueInt, ok := w.Value.(int)
	if ok {
		return []byte(fmt.Sprintf("{ \"%s\": { \"%s\": %d }}", w.Name, w.Operator, valueInt)), nil
	}

	valueFloat32, ok := w.Value.(float32)
	if ok {
		return []byte(fmt.Sprintf("{ \"%s\": { \"%s\": %f }}", w.Name, w.Operator, valueFloat32)), nil
	}

	valueFloat64, ok := w.Value.(float64)
	if ok {
		return []byte(fmt.Sprintf("{ \"%s\": { \"%s\": %f }}", w.Name, w.Operator, valueFloat64)), nil
	}

	return nil, errors.New("value should be string, int or float")
}

type WhereOr struct {
	Where
	Entries []Where
}

func (w WhereOr) MarshalJSON() ([]byte, error) {
	s := map[string][]Where{
		"$or": w.Entries,
	}
	return json.Marshal(s)
}

type WhereAnd struct {
	Where
	Entries []Where
}

func (w WhereAnd) MarshalJSON() ([]byte, error) {
	s := map[string][]Where{
		"$and": w.Entries,
	}
	return json.Marshal(s)
}

type DocumentOperator string

const (
	Contains       DocumentOperator = "$contains"
	DoesNotContain DocumentOperator = "$not_contains"
)

type WhereDocument struct {
	Operator DocumentOperator
	Value    string
}

func (w WhereDocument) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("{ \"%s\": \"%s\" }", w.Operator, w.Value)), nil
}
