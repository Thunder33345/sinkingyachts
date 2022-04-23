package sinking_yachts

import (
	"encoding/json"
	"fmt"
	"time"
)

//save is the on disk save format
type save struct {
	LastUpdated time.Time `json:"last_updated"`
	Domains     []string  `json:"domains"`
}

//DomainUpdate represent an update to the domains list,
//which depending on type, it could mean adding or deleting domains
type DomainUpdate struct {
	//Add defines if it is adding or removing domains
	Add bool
	//Domains is a slice of domains
	Domains []string
}

//modEntry is the api representation of a domain update
type modEntry struct {
	//Type is the method, should be "add" or "delete"
	Type string `json:"type"`
	//Domains is a slice of domains
	Domains []string `json:"domains"`
}

func (m *DomainUpdate) UnmarshalJSON(bytes []byte) error {
	var me modEntry
	err := json.Unmarshal(bytes, &me)
	if err != nil {
		return err
	}
	switch me.Type {
	case "add":
		m.Add = true
	case "delete":
		m.Add = false
	default:
		return fmt.Errorf(`expecting "add" or "delete" in modEntry.Type, received "%s"`, me.Type)
	}
	m.Domains = me.Domains
	return nil
}

type empty struct{}

type unexpectedStatusError struct {
	endpoint string
	status   int
}

func (err unexpectedStatusError) Error() string {
	return fmt.Sprintf(`unexpected status code: recieved "%d" on "%s"`, err.status, err.endpoint)
}
