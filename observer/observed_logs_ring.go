package observer

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

var _ ObservedLogs = (*ObservedLogsRing)(nil)

// ObservedLogsRing is a concurrency-safe, ring buffer implementation of ObservedLogs.
type ObservedLogsRing struct {
	mu sync.RWMutex

	fixed bool
	size  int
	over  bool
	logs  []LoggedRecord
}

// NewObservedLogsRing creates and initializes new ObservedLogsRing.
// If maxLogs is zero then the number of logs stored is unlimited.
func NewObservedLogsRing(maxLogs uint) *ObservedLogsRing {
	ol := ObservedLogsRing{fixed: maxLogs > 0}
	if maxLogs > 0 {
		ol.logs = make([]LoggedRecord, maxLogs)
	}
	return &ol
}

// Len returns the number of items in the collection.
func (o *ObservedLogsRing) Len() int {
	o.mu.RLock()
	n := o.len()
	o.mu.RUnlock()
	return n
}

func (o *ObservedLogsRing) len() (n int) {
	if !o.fixed || !o.over {
		n = o.size
	} else {
		n = cap(o.logs)
	}
	return
}

// All returns a copy of all the observed logs.
func (o *ObservedLogsRing) All() []LoggedRecord {
	o.mu.RLock()
	ret := o.all()
	o.mu.RUnlock()
	return ret
}

func (o *ObservedLogsRing) all() []LoggedRecord {
	ret := make([]LoggedRecord, o.len())
	if !o.fixed {
		copy(ret, o.logs)
	} else {
		copy(ret, o.logs[o.size%cap(o.logs):])
		copy(ret[cap(o.logs)-o.size%cap(o.logs):], o.logs[:o.size%cap(o.logs)])
	}
	return ret
}

// TakeAll returns a copy of all the observed logs, and truncates the observed slice.
func (o *ObservedLogsRing) TakeAll() []LoggedRecord {
	o.mu.Lock()
	ret := o.all()
	o.size = 0
	o.over = false
	if !o.fixed {
		o.logs = nil
	} else {
		o.logs = make([]LoggedRecord, cap(o.logs))
	}
	o.mu.Unlock()
	return ret
}

// AllUntimed returns a copy of all the observed logs, but overwrites the
// observed timestamps with time.Time's zero value. This is useful when making
// assertions in tests.
func (o *ObservedLogsRing) AllUntimed() []LoggedRecord {
	ret := o.All()
	for i := range ret {
		ret[i].Record.Time = time.Time{}
	}
	return ret
}

// FilterLevelExact filters entries to those logged at exactly the given level.
func (o *ObservedLogsRing) FilterLevelExact(level slog.Level) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return r.Record.Level == level
	})
}

// FilterMessage filters entries to those that have the specified message.
func (o *ObservedLogsRing) FilterMessage(msg string) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return r.Record.Message == msg
	})
}

// FilterMessageSnippet filters entries to those that have a message containing the specified snippet.
func (o *ObservedLogsRing) FilterMessageSnippet(snippet string) ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return strings.Contains(r.Record.Message, snippet)
	})
}

// FilterAttr filters entries to those that have the specified attribute.
func (o *ObservedLogsRing) FilterAttr(attr slog.Attr) ObservedLogs {
	return o.Filter(func(e LoggedRecord) bool {
		return filterAttr(e.Attrs, attr)
	})
}

// FilterFieldKey filters entries to those that have the specified key.
func (o *ObservedLogsRing) FilterFieldKey(key string) ObservedLogs {
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
func (o *ObservedLogsRing) Filter(keep func(LoggedRecord) bool) ObservedLogs {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var filtered []LoggedRecord
	if !o.fixed {
		for _, entry := range o.logs {
			if keep(entry) {
				filtered = append(filtered, entry)
			}
		}
	} else {
		for _, entry := range o.logs[o.size%cap(o.logs):] {
			if keep(entry) {
				filtered = append(filtered, entry)
			}
		}
		for _, entry := range o.logs[:o.size%cap(o.logs)] {
			if keep(entry) {
				filtered = append(filtered, entry)
			}
		}
	}
	return &ObservedLogsRing{logs: filtered, size: len(filtered)}
}

// Add stores log record to the collection. Expects a record that is already prepared for storing:
// - has no attributes
// - attributes collection is passed alongside
func (o *ObservedLogsRing) Add(record slog.Record, attrs []slog.Attr) {
	o.mu.Lock()
	o.size++
	if !o.fixed {
		o.logs = append(o.logs, LoggedRecord{Record: record, Attrs: attrs})
	} else {
		idx := (o.size - 1) % cap(o.logs)
		o.logs[idx].Record = record
		o.logs[idx].Attrs = attrs
		o.over = o.size > cap(o.logs)
	}
	o.mu.Unlock()
}
