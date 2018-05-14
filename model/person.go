package model

import "encoding/base64"

// Person represents a human
type Person struct {
	WikidataURI string
	Name        string
	PhotoURLs   []string
	EmployerURI string
}

// ID returns a GraphQL-friendly ID for a person
func (p Person) ID() string {
	return base64.StdEncoding.EncodeToString([]byte(p.WikidataURI))
}
