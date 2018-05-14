package model

import "encoding/base64"

type Company struct {
	WikidataURI  string
	Name         string
	Homepage     string
	LogoURL      string
	ExecutiveURI string
	PeopleURIs   []string
}

func (c Company) ID() string {
	return base64.StdEncoding.EncodeToString([]byte(c.WikidataURI))
}
