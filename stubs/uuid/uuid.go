package uuid

import (
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type UUID [16]byte
type NullUUID struct {
	UUID  UUID
	Valid bool
}

var Nil UUID

func New() UUID {
	return UUID{}
}

func Parse(s string) (UUID, error) {
	var u UUID
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return Nil, errors.New("invalid UUID length")
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Nil, err
	}
	copy(u[:], b)
	return u, nil
}

func (u UUID) String() string {
	hexStr := hex.EncodeToString(u[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

func (u UUID) Value() (driver.Value, error) {
	return u.String(), nil
}

func (u *UUID) Scan(src any) error {
	switch v := src.(type) {
	case string:
		parsed, err := Parse(v)
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	case []byte:
		parsed, err := Parse(string(v))
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	default:
		return errors.New("unsupported UUID scan type")
	}
}

func (nu *NullUUID) Scan(src any) error {
	if src == nil {
		nu.UUID = Nil
		nu.Valid = false
		return nil
	}
	var u UUID
	if err := (&u).Scan(src); err != nil {
		return err
	}
	nu.UUID = u
	nu.Valid = true
	return nil
}

func (nu NullUUID) Value() (driver.Value, error) {
	if !nu.Valid {
		return nil, nil
	}
	return nu.UUID.Value()
}
