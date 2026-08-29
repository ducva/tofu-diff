package domain

import (
	"bytes"
	"encoding/json"
	"sort"
)

// AttributeDiff is a presentation-neutral comparison of one top-level
// attribute. Presenters decide how to mask, truncate, style, or wrap values.
type AttributeDiff struct {
	Key             string
	BeforeRaw       json.RawMessage
	AfterRaw        json.RawMessage
	BeforeSensitive bool
	AfterSensitive  bool
	IsUnknownAfter  bool
}

// DiffAttributes computes a stable attribute-level diff for a Change.
func DiffAttributes(c Change) []AttributeDiff {
	keySet := make(map[string]struct{})
	for key := range c.Before {
		keySet[key] = struct{}{}
	}
	for key := range c.After {
		keySet[key] = struct{}{}
	}
	for key := range c.AfterUnknown {
		keySet[key] = struct{}{}
	}

	diffs := make([]AttributeDiff, 0, len(keySet))
	for key := range keySet {
		if c.AfterUnknown[key] {
			diffs = append(diffs, AttributeDiff{
				Key:             key,
				BeforeRaw:       c.Before[key],
				AfterRaw:        c.After[key],
				BeforeSensitive: c.BeforeSensitive[key],
				AfterSensitive:  c.AfterSensitive[key],
				IsUnknownAfter:  true,
			})
			continue
		}
		if bytes.Equal(c.Before[key], c.After[key]) {
			continue
		}
		diffs = append(diffs, AttributeDiff{
			Key:             key,
			BeforeRaw:       c.Before[key],
			AfterRaw:        c.After[key],
			BeforeSensitive: c.BeforeSensitive[key],
			AfterSensitive:  c.AfterSensitive[key],
		})
	}

	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Key < diffs[j].Key })
	return diffs
}
