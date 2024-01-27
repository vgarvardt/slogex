package observer

import (
	"context"
	"log/slog"
)

// ObservedLogs is a collection of observed logs.
type ObservedLogs interface {
	Add(record slog.Record, attrs []slog.Attr)
	// Len returns the number of items in the collection.
	Len() int
	// All returns a copy of all the observed logs.
	All() []LoggedRecord
	// TakeAll returns a copy of all the observed logs, and truncates the observed slice.
	TakeAll() []LoggedRecord
	// AllUntimed returns a copy of all the observed logs, but overwrites the
	// observed timestamps with time.Time's zero value. This is useful when making
	// assertions in tests.
	AllUntimed() []LoggedRecord
	// Filter returns a copy of this ObservedLogsDefault containing only those entries
	// for which the provided function returns true.
	Filter(keep func(LoggedRecord) bool) ObservedLogs
	// FilterLevelExact filters entries to those logged at exactly the given level.
	FilterLevelExact(level slog.Level) ObservedLogs
	// FilterMessage filters entries to those that have the specified message.
	FilterMessage(msg string) ObservedLogs
	// FilterMessageSnippet filters entries to those that have a message containing the specified snippet.
	FilterMessageSnippet(snippet string) ObservedLogs
	// FilterAttr filters entries to those that have the specified attribute.
	FilterAttr(attr slog.Attr) ObservedLogs
	// FilterFieldKey filters entries to those that have the specified key.
	FilterFieldKey(key string) ObservedLogs
}

// HandlerOptions are options for an observer Handler.
type HandlerOptions struct {
	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes slog.LevelInfo.
	// The handler calls Level.Level for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler

	// MaxLogs is the maximum number of logs to store. If this is zero, the
	// default, then the number of logs stored is unlimited.
	// If ObservedLogs is set, then MaxLogs is ignored.
	MaxLogs uint

	// ObservedLogs collection implementation. If not set then ObservedLogsDefault is used.
	// When set - MaxLogs is ignored.
	ObservedLogs ObservedLogs
}

var _ slog.Handler = (*contextObserver)(nil)

type contextObserver struct {
	opts   HandlerOptions
	logs   ObservedLogs
	attrs  []slog.Attr
	groups []slog.Attr
}

// New creates new slog.Handler that buffers logs in memory.
// It's particularly useful in tests.
func New(opts *HandlerOptions) (slog.Handler, ObservedLogs) {
	if opts == nil {
		opts = &HandlerOptions{}
	}

	ol := opts.ObservedLogs
	if ol == nil {
		ol = NewObservedLogsDefault(opts.MaxLogs)
	}

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

	c.logs.Add(rc, attrs)
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
