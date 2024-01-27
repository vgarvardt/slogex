package observer

import (
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"
)

var _ ObservedLogs = (*ObservedLogsDefault)(nil)

// ObservedLogsDefault is a concurrency-safe, ordered implementation of ObservedLogs.
type ObservedLogsDefault struct {
	mu sync.RWMutex

	fixed bool
	size  int
	logs  []LoggedRecord
}

// NewObservedLogsDefault creates and initializes new ObservedLogsDefault.
// If maxLogs is zero then the number of logs stored is unlimited.
func NewObservedLogsDefault(maxLogs uint) *ObservedLogsDefault {
	ol := ObservedLogsDefault{fixed: maxLogs > 0}
	if maxLogs > 0 {
		ol.logs = make([]LoggedRecord, 0, maxLogs)
	}
	return &ol
}

// Len returns the number of items in the collection.
func (o *ObservedLogsDefault) Len() int {
	o.mu.RLock()
	n := len(o.logs)
	o.mu.RUnlock()
	return n
}

// All returns a copy of all the observed logs.
func (o *ObservedLogsDefault) All() []LoggedRecord {
	o.mu.RLock()
	ret := make([]LoggedRecord, len(o.logs))
	copy(ret, o.logs)
	o.mu.RUnlock()
	return ret
}

// TakeAll returns a copy of all the observed logs, and truncates the observed slice.
func (o *ObservedLogsDefault) TakeAll() []LoggedRecord {
	o.mu.Lock()
	ret := o.logs
	o.logs = nil
	o.mu.Unlock()
	return ret
}

// AllUntimed returns a copy of all the observed logs, but overwrites the
// observed timestamps with time.Time's zero value. This is useful when making
// assertions in tests.
func (o *ObservedLogsDefault) AllUntimed() []LoggedRecord {
	ret := o.All()
	for i := range ret {
		ret[i].Record.Time = time.Time{}
	}
	return ret
}

// FilterLevelExact filters entries to those logged at exactly the given level.
func (o *ObservedLogsDefault) FilterLevelExact(level slog.Level) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return r.Record.Level == level
	})
}

// FilterMessage filters entries to those that have the specified message.
func (o *ObservedLogsDefault) FilterMessage(msg string) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return r.Record.Message == msg
	})
}

// FilterMessageSnippet filters entries to those that have a message containing the specified snippet.
func (o *ObservedLogsDefault) FilterMessageSnippet(snippet string) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return strings.Contains(r.Record.Message, snippet)
	})
}

// FilterAttr filters entries to those that have the specified attribute.
func (o *ObservedLogsDefault) FilterAttr(attr slog.Attr) ObservedLogs {
	return o.Filter(func(e LoggedRecord) bool {
		return filterAttr(e.Attrs, attr)
	})
}

// FilterFieldKey filters entries to those that have the specified key.
func (o *ObservedLogsDefault) FilterFieldKey(key string) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		for _, a := range r.Attrs {
			if a.Key == key {
				return true
			}
		}
		return false
	})
}

// Filter returns a copy of this ObservedLogsDefault containing only those entries
// for which the provided function returns true.
func (o *ObservedLogsDefault) Filter(keep func(LoggedRecord) bool) ObservedLogs {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var filtered []LoggedRecord
	for _, entry := range o.logs {
		if keep(entry) {
			filtered = append(filtered, entry)
		}
	}
	return &ObservedLogsDefault{logs: filtered}
}

// Add stores log record to the collection. Expects a record that is already prepared for storing:
// - has no attributes
// - attributes collection is passed alongside
func (o *ObservedLogsDefault) Add(record slog.Record, attrs []slog.Attr) {
	o.mu.Lock()
	o.size++
	if o.fixed && o.size > cap(o.logs) {
		copy(o.logs[0:], o.logs[1:])
		o.size--
		o.logs[o.size-1] = LoggedRecord{Record: record, Attrs: attrs}
	} else {
		o.logs = append(o.logs, LoggedRecord{Record: record, Attrs: attrs})
	}
	o.mu.Unlock()
}

func filterAttr(attrs []slog.Attr, attr slog.Attr) bool {
	for _, a := range attrs {
		kind := a.Value.Kind()
		if kind == slog.KindGroup {
			if filterAttr(a.Value.Group(), attr) {
				return true
			}
			continue
		}

		// a.Equal(attr) does this as well, but we're going to compare Any kind using reflect to avoid
		// panicking when comparing complex types.
		if a.Key != attr.Key || kind != attr.Value.Kind() {
			continue
		}

		if (kind == slog.KindAny || kind == slog.KindLogValuer) && reflect.DeepEqual(a.Value.Any(), attr.Value.Any()) {
			return true
		}

		if a.Equal(attr) {
			return true
		}
	}
	return false
}
