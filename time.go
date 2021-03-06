package main

import (
	"strconv"
	"strings"
	"time"
)

type Time struct {
	time.Time
}

func (t *Time) UnmarshalJSON(b []byte) (err error) {
	unix, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSuffix(string(b), "\""), "\""), 10, 64)
	if err != nil {
		return
	}
	t.Time = time.Unix(unix, 0)
	return
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.Time == (time.Time{}) {
		return []byte("\"\""), nil
	}
	return []byte("\"" + strconv.FormatInt(t.Time.Unix(), 10) + "\""), nil
}
