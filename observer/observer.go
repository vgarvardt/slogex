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

// TakeAll returns a copy of all the observed logs, and truncates the observed
// slice.
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

func (o *ObservedLogs) add(record slog.Record) {
	o.mu.Lock()

	r := slog.NewRecord(record.Time, record.Level, record.Message, 0)
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	o.logs = append(o.logs, LoggedRecord{Record: r, Attrs: attrs})

	o.mu.Unlock()
}

var _ slog.Handler = (*contextObserver)(nil)

type contextObserver struct {
	opts   slog.HandlerOptions
	logs   *ObservedLogs
	attrs  []slog.Attr
	groups []string
}

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

func (c contextObserver) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if c.opts.Level != nil {
		minLevel = c.opts.Level.Level()
	}
	return level >= minLevel
}

func (c contextObserver) Handle(_ context.Context, record slog.Record) error {
	rc := record.Clone()
	rc.AddAttrs(c.attrs...)

	c.logs.add(rc)
	return nil
}

func (c contextObserver) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextObserver{
		opts:   c.opts,
		logs:   c.logs,
		attrs:  append(c.attrs[:len(c.attrs):len(c.attrs)], attrs...),
		groups: c.groups,
	}
}

func (c contextObserver) WithGroup(name string) slog.Handler {
	return &contextObserver{
		opts:   c.opts,
		logs:   c.logs,
		attrs:  c.attrs,
		groups: append(c.groups[:len(c.groups):len(c.groups)], name),
	}
}
