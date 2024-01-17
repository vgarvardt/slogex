package observer

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"
)

// ObservedLogs is a concurrency-safe, ordered collection of observed logs.
type ObservedLogs struct {
	mu   sync.RWMutex
	logs []LoggedRecord
}

// Len returns the number of items in the collection.
func (o *ObservedLogs) Len() int {
	o.mu.RLock()
	n := len(o.logs)
	o.mu.RUnlock()
	return n
}

// All returns a copy of all the observed logs.
func (o *ObservedLogs) All() []LoggedRecord {
	o.mu.RLock()
	ret := make([]LoggedRecord, len(o.logs))
	copy(ret, o.logs)
	o.mu.RUnlock()
	return ret
}

// TakeAll returns a copy of all the observed logs, and truncates the observed slice.
func (o *ObservedLogs) TakeAll() []LoggedRecord {
	o.mu.Lock()
	ret := o.logs
	o.logs = nil
	o.mu.Unlock()
	return ret
}

// AllUntimed returns a copy of all the observed logs, but overwrites the
// observed timestamps with time.Time's zero value. This is useful when making
// assertions in tests.
func (o *ObservedLogs) AllUntimed() []LoggedRecord {
	ret := o.All()
	for i := range ret {
		ret[i].Record.Time = time.Time{}
	}
	return ret
}

// FilterLevelExact filters entries to those logged at exactly the given level.
func (o *ObservedLogs) FilterLevelExact(level slog.Level) *ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return r.Record.Level == level
	})
}

// FilterMessage filters entries to those that have the specified message.
func (o *ObservedLogs) FilterMessage(msg string) *ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return r.Record.Message == msg
	})
}

// FilterMessageSnippet filters entries to those that have a message containing the specified snippet.
func (o *ObservedLogs) FilterMessageSnippet(snippet string) *ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		return strings.Contains(r.Record.Message, snippet)
	})
}

// FilterAttr filters entries to those that have the specified attribute.
func (o *ObservedLogs) FilterAttr(attr slog.Attr) *ObservedLogs {
	return o.Filter(func(e LoggedRecord) bool {
		return filterAttr(e.Attrs, attr)
	})
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

// FilterFieldKey filters entries to those that have the specified key.
func (o *ObservedLogs) FilterFieldKey(key string) *ObservedLogs {
	return o.Filter(func(r LoggedRecord) bool {
		for _, a := range r.Attrs {
			if a.Key == key {
				return true
			}
		}
		return false
	})
}

// Filter returns a copy of this ObservedLogs containing only those entries
// for which the provided function returns true.
func (o *ObservedLogs) Filter(keep func(LoggedRecord) bool) *ObservedLogs {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var filtered []LoggedRecord
	for _, entry := range o.logs {
		if keep(entry) {
			filtered = append(filtered, entry)
		}
	}
	return &ObservedLogs{logs: filtered}
}

// add stores log record to the collection. Expects a record that is already prepared for storing:
// - has no attributes
// - attributes collection is passed alongside
func (o *ObservedLogs) add(record slog.Record, attrs []slog.Attr) {
	o.mu.Lock()
	o.logs = append(o.logs, LoggedRecord{Record: record, Attrs: attrs})
	o.mu.Unlock()
}

var _ slog.Handler = (*contextObserver)(nil)

type contextObserver struct {
	opts   slog.HandlerOptions
	logs   *ObservedLogs
	attrs  []slog.Attr
	groups []slog.Attr
}

// New creates new slog.Handler that buffers logs in memory.
// It's particularly useful in tests.
func New(opts *slog.HandlerOptions) (slog.Handler, *ObservedLogs) {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	ol := &ObservedLogs{}
	return &contextObserver{
		opts: *opts,
		logs: ol,
	}, ol
}

// Enabled implements slog.Handler: reports whether the handler handles records at the given level.
func (c contextObserver) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if c.opts.Level != nil {
		minLevel = c.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle implements slog.Handler: handles the Record.
func (c contextObserver) Handle(_ context.Context, record slog.Record) error {
	rc := slog.NewRecord(record.Time, record.Level, record.Message, 0)
	attrs := c.attrs[:len(c.attrs):len(c.attrs)]

	recordAttrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		recordAttrs = append(recordAttrs, attr)
		return true
	})

	if len(c.groups) > 0 {
		if len(recordAttrs) > 0 {
			currentGroupIdx := len(c.groups) - 1
			c.groups[currentGroupIdx].Value = slog.GroupValue(append(c.groups[currentGroupIdx].Value.Group(), recordAttrs...)...)
		}

		for i := len(c.groups) - 1; i >= 1; i-- {
			c.groups[i-1].Value = slog.GroupValue(append(c.groups[i-1].Value.Group(), c.groups[i])...)
		}
		attrs = append(attrs, c.groups[0])
	} else {
		attrs = append(recordAttrs, attrs...)
	}

	c.logs.add(rc, attrs)
	return nil
}

// WithAttrs implements slog.Handler: returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
func (c contextObserver) WithAttrs(attrs []slog.Attr) slog.Handler {
	co := contextObserver{
		opts:   c.opts,
		logs:   c.logs,
		groups: c.groups[:len(c.groups):len(c.groups)],
		attrs:  c.attrs[:len(c.attrs):len(c.attrs)],
	}

	if len(c.groups) == 0 {
		co.attrs = append(co.attrs, attrs...)
	} else {
		currentGroupIdx := len(co.groups) - 1
		co.groups[currentGroupIdx].Value = slog.GroupValue(append(co.groups[currentGroupIdx].Value.Group(), attrs...)...)
	}

	return &co
}

// WithGroup implements slog.Handler: returns a new Handler with the given group appended to
// the receiver's existing groups.
func (c contextObserver) WithGroup(name string) slog.Handler {
	co := contextObserver{
		opts:   c.opts,
		logs:   c.logs,
		attrs:  c.attrs[:len(c.attrs):len(c.attrs)],
		groups: append(c.groups[:len(c.groups):len(c.groups)], slog.Group(name)),
	}

	return &co
}
