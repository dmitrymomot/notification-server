package main

import "encoding/json"

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "    ")
	return string(s)
}
