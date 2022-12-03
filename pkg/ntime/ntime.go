package ntime

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// NTime represents a nullable time.Time.
// It can be used a scan destination and can be marshalled to JSON.
type NTime struct {
	time    time.Time
	isValid bool // false when Time is null, possibly redundant
}

// UnmarshalJSON parses a RFC3339 time string into a time.Time object
func (nt *NTime) UnmarshalJSON(b []byte) error {
	parsedTime, err := time.Parse(time.RFC3339, string(b))
	if err != nil {
		return err
	}
	*nt = NTime{parsedTime, true}
	return nil
}

// MarshalJSON implements the Marshaller interface and operates on values rather than pointers, given NTime's heft.
func (nt NTime) MarshalJSON() ([]byte, error) {
	// for some obscure reason the quotes are necessary
	if nt.isValid {
		return []byte(fmt.Sprintf("\"%s\"", nt.time.UTC().Format(time.RFC3339))), nil
	}
	return []byte("null"), nil
}

// Scan implements the Scanner interface.
func (nt *NTime) Scan(value any) error {
	nt.time, nt.isValid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NTime) Value() (driver.Value, error) {
	// arguable choice, would yield poor results with full-fledged DBs tk
	if nt.isValid {
		return driver.Value(nt.time.UTC().Format(time.RFC3339)), nil
	}
	return nil, nil
}

func Now() NTime {
	return NTime{time: time.Now().UTC(), isValid: true}
}

func (nt *NTime) Before(compared NTime) bool {
	return nt.time.Before(compared.time)
}
