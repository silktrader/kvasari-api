package ntime

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// NTime represents a nullable time.Time.
// It can be used a scan destination and can be marshalled to JSON.
type NTime struct {
	Time    time.Time
	IsValid bool // false when Time is null, possibly redundant
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

func (nt *NTime) MarshalJSON() ([]byte, error) {
	// for some obscure reason the quotes are necessary
	if nt.IsValid {
		return []byte(fmt.Sprintf("\"%s\"", nt.Time.UTC().Format(time.RFC3339))), nil
	}
	return []byte("null"), nil
}

// Scan implements the Scanner interface.
func (nt *NTime) Scan(value any) error {
	nt.Time, nt.IsValid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NTime) Value() (driver.Value, error) {
	// arguable choice, would yield poor results with full-fledged DBs tk
	if nt.IsValid {
		return driver.Value(nt.Time.UTC().Format(time.RFC3339)), nil
	}
	return nil, nil
}

func Now() NTime {
	return NTime{Time: time.Now().UTC(), IsValid: true}
}
