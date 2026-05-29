package settings

import (
	"fmt"
	"strconv"
	"strings"
)

type Recipient string

type Recipients []Recipient

func (r Recipients) Int64s() ([]int64, error) {
	ids := make([]int64, 0, len(r))
	for i, raw := range r {
		value := strings.TrimSpace(string(raw))
		if value == "" {
			return nil, fmt.Errorf("recipient %d is empty", i)
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("recipient %d: %w", i, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r Recipients) Strings() []string {
	values := make([]string, 0, len(r))
	for _, raw := range r {
		value := strings.TrimSpace(string(raw))
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}
