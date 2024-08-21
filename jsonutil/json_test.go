package jsonutil

import (
	"fmt"
	"testing"
)

func TestJSON(t *testing.T) {
	json := `{
	"employees": [{
		"id": 1,
		"name": "John Smith",
		"age": 34,
		"position": "Software Engineer",
		"department": "IT",
		"email": "john.smith@example.com",
		"address": {
			"street": "1234 Elm Street",
			"city": "Springfield",
			"state": "IL",
			"zip": "62701"
		},
		"phone": "555-1234"
	}, {
		"id": 2,
		"name": "Jane Doe",
		"age": 29,
		"position": "Project Manager",
		"department": "Operations",
		"email": "jane.doe@example.com",
		"address": {
			"street": "5678 Oak Street",
			"city": "Springfield",
			"state": "IL",
			"zip": "62701"
		},
		"phone": "555-5678"
	}],
	"departments": [{
		"id": 1,
		"name": "IT",
		"head": "John Smith",
		"budget": 1000000
	}, {
		"id": 2,
		"name": "Operations",
		"head": "Jane Doe",
		"budget": 500000
	}]
}`
	jsonObj, err := FromString(json)
	if err != nil {
		fmt.Println(err)
	}
	subJSON, err := jsonObj.GetSubJSON("departments")
	if err != nil {
		fmt.Println(err)
	}
	s, err := subJSON.String()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(s)

	fromString, err := GetJSONFromString(json, "departments", 6, "id")

	if err != nil {
		fmt.Println(err)
	}
	if fromString != nil {
		fmt.Println(fromString.String())
	}
}
