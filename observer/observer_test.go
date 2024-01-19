package observer

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertEmpty(t testing.TB, logs *ObservedLogs) {
	assert.Equal(t, 0, logs.Len(), "Expected empty ObservedLogs to have zero length.")
	assert.Equal(t, []LoggedRecord{}, logs.All(), "Unexpected LoggedRecord in empty ObservedLogs.")
}

func TestObserver(t *testing.T) {
	handler, logs := New(nil)
	assertEmpty(t, logs)

	t.Run("Enabled", func(t *testing.T) {
		ctx := context.Background()

		assert.False(t, handler.Enabled(ctx, slog.LevelDebug), "Observer should be disabled for Debug level.")
		assert.True(t, handler.Enabled(ctx, slog.LevelInfo), "Observer should be enabled for Info level.")
		assert.True(t, handler.Enabled(ctx, slog.LevelWarn), "Observer should be enabled for Warn level.")
		assert.True(t, handler.Enabled(ctx, slog.LevelError), "Observer should be enabled for Error level.")
	})

	logger := slog.New(handler).With(slog.Int("i", 1))
	logger.Info("foo")
	logger.Debug("bar")
	want := []LoggedRecord{{Record: slog.Record{Message: "foo", Level: slog.LevelInfo}, Attrs: []slog.Attr{slog.Int("i", 1)}}}

	assert.Equal(t, 1, logs.Len(), "Unexpected observed logs Len.")
	assert.Equal(t, want, logs.AllUntimed(), "Unexpected contents from AllUntimed.")

	all := logs.All()
	require.Equal(t, 1, len(all), "Unexpected number of LoggedRecord returned from All.")
	assert.NotEqual(t, time.Time{}, all[0].Record.Time, "Expected non-zero time on LoggedEntry.")

	// copy & zero time for stable assertions
	untimed := append([]LoggedRecord{}, all...)
	untimed[0].Record.Time = time.Time{}
	assert.Equal(t, want, untimed, "Unexpected LoggedRecord from All.")

	assert.Equal(t, all, logs.TakeAll(), "Expected All and TakeAll to return identical results.")
	assertEmpty(t, logs)
}

func TestObserverWith(t *testing.T) {
	handler1, logs := New(nil)

	// need to pad out enough initial fields so that the underlying slice cap()
	// gets ahead of its len() so that the handler3/4 With append's could choose
	// not to copy (if the implementation doesn't force them)
	handler1 = handler1.WithAttrs([]slog.Attr{slog.Int("a", 1), slog.Int("b", 2)})

	handler2 := handler1.WithAttrs([]slog.Attr{slog.Int("c", 3)})
	handler3 := handler2.WithAttrs([]slog.Attr{slog.Int("d", 4)})
	handler4 := handler2.WithAttrs([]slog.Attr{slog.Int("e", 5)})
	record := slog.Record{Level: slog.LevelInfo, Message: "hello"}

	ctx := context.Background()
	for i, handler := range []slog.Handler{handler2, handler3, handler4} {
		if handler.Enabled(ctx, record.Level) {
			slog.New(handler).LogAttrs(ctx, record.Level, record.Message, slog.Int("i", i))
		}
	}

	assert.Equal(t, []LoggedRecord{
		{
			Record: record,
			Attrs: []slog.Attr{
				slog.Int("i", 0),
				slog.Int("a", 1),
				slog.Int("b", 2),
				slog.Int("c", 3),
			},
		},
		{
			Record: record,
			Attrs: []slog.Attr{
				slog.Int("i", 1),
				slog.Int("a", 1),
				slog.Int("b", 2),
				slog.Int("c", 3),
				slog.Int("d", 4),
			},
		},
		{
			Record: record,
			Attrs: []slog.Attr{
				slog.Int("i", 2),
				slog.Int("a", 1),
				slog.Int("b", 2),
				slog.Int("c", 3),
				slog.Int("e", 5),
			},
		},
	}, logs.AllUntimed(), "expected no field sharing between WithAttrs siblings")
}

func TestObserverWithGroup(t *testing.T) {
	handler, logs := New(nil)
	logger := slog.New(handler).With(slog.Int("i", 1))

	t.Run("single WithGroup", func(t *testing.T) {
		logger.WithGroup("foo").With(slog.Int("i", 2), slog.Group("bar", slog.Int("i", 3))).Info("foo")

		records := logs.TakeAll()
		require.Len(t, records, 1)

		assert.Equal(t, map[string]any{
			"i": int64(1),
			"foo": map[string]any{
				"i": int64(2),
				"bar": map[string]any{
					"i": int64(3),
				},
			},
		}, records[0].AttrsMap())
	})

	t.Run("nested WithGroup", func(t *testing.T) {
		logger.WithGroup("foo").With(slog.Int("i", 2)).WithGroup("bar").With(slog.Int("i", 3)).Info("foo", slog.Int("j", 4))

		records := logs.TakeAll()
		require.Len(t, records, 1)

		// checked with the slog.NewTextHandler() - should match "msg=foo i=1 foo.i=2 foo.bar.i=3 foo.bar.j=4"
		assert.Equal(t, map[string]any{
			"i": int64(1),
			"foo": map[string]any{
				"i": int64(2),
				"bar": map[string]any{
					"i": int64(3),
					"j": int64(4),
				},
			},
		}, records[0].AttrsMap())
	})
}

func TestFilters(t *testing.T) {
	records := []LoggedRecord{
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "log a"},
			Attrs:  []slog.Attr{slog.String("fStr", "1"), slog.Int("a", 1)},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "log a"},
			Attrs:  []slog.Attr{slog.String("fStr", "2"), slog.Int("b", 2)},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "log b"},
			Attrs:  []slog.Attr{slog.Int("a", 1), slog.Int("b", 2)},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "log c"},
			Attrs:  []slog.Attr{slog.Int("a", 1), slog.Group("ns", slog.Int("a", 2))},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "msg 1"},
			Attrs:  []slog.Attr{slog.Int("a", 1), slog.Group("ns", slog.String("group-must-not", "be-empty"))},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "any map"},
			Attrs:  []slog.Attr{slog.Any("map", map[string]string{"a": "b"})},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "any slice"},
			Attrs:  []slog.Attr{slog.Any("slice", []string{"a"})},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "msg 2"},
			Attrs:  []slog.Attr{slog.Int("b", 2), slog.Group("filterMe", slog.String("group-must-not", "be-empty"))},
		},
		{
			Record: slog.Record{Level: slog.LevelInfo, Message: "any slice"},
			Attrs:  []slog.Attr{slog.Any("filterMe", []string{"b"})},
		},
		{
			Record: slog.Record{Level: slog.LevelWarn, Message: "danger will robinson"},
			Attrs:  []slog.Attr{slog.Int("b", 42)},
		},
		{
			Record: slog.Record{Level: slog.LevelError, Message: "warp core breach"},
			Attrs:  []slog.Attr{slog.Int("b", 42)},
		},
	}

	handler, logs := New(nil)
	logger := slog.New(handler)
	ctx := context.Background()

	for _, log := range records {
		logger.LogAttrs(ctx, log.Record.Level, log.Record.Message, log.Attrs...)
	}

	tests := []struct {
		msg      string
		filtered *ObservedLogs
		want     []LoggedRecord
	}{
		{
			msg:      "filter by message",
			filtered: logs.FilterMessage("log a"),
			want:     records[0:2],
		},
		{
			msg:      "filter by field",
			filtered: logs.FilterAttr(slog.String("fStr", "1")),
			want:     records[0:1],
		},
		{
			msg:      "filter by message and field",
			filtered: logs.FilterMessage("log a").FilterAttr(slog.Int("b", 2)),
			want:     records[1:2],
		},
		{
			msg:      "filter by field with duplicate fields",
			filtered: logs.FilterAttr(slog.Int("a", 2)),
			want:     records[3:4],
		},
		{
			msg:      "filter doesn't match any messages",
			filtered: logs.FilterMessage("no match"),
			want:     []LoggedRecord{},
		},
		{
			msg:      "filter by snippet",
			filtered: logs.FilterMessageSnippet("log"),
			want:     records[0:4],
		},
		{
			msg:      "filter by snippet and field",
			filtered: logs.FilterMessageSnippet("a").FilterAttr(slog.Int("b", 2)),
			want:     records[1:2],
		},
		{
			msg:      "filter for map",
			filtered: logs.FilterAttr(slog.Any("map", map[string]string{"a": "b"})),
			want:     records[5:6],
		},
		{
			msg:      "filter for slice",
			filtered: logs.FilterAttr(slog.Any("slice", []string{"a"})),
			want:     records[6:7],
		},
		{
			msg:      "filter field key",
			filtered: logs.FilterFieldKey("filterMe"),
			want:     records[7:9],
		},
		{
			msg: "filter by arbitrary function",
			filtered: logs.Filter(func(r LoggedRecord) bool {
				return len(r.Attrs) > 1
			}),
			want: func() []LoggedRecord {
				// Do not modify records slice.
				w := make([]LoggedRecord, 0, len(records))
				w = append(w, records[0:5]...)
				w = append(w, records[7])
				return w
			}(),
		},
		{
			msg:      "filter level",
			filtered: logs.FilterLevelExact(slog.LevelWarn),
			want:     records[9:10],
		},
	}

	for _, tt := range tests {
		got := tt.filtered.AllUntimed()
		assert.Equal(t, tt.want, got, tt.msg)
	}
}

func TestMaxLogs(t *testing.T) {
	handler, logs := New(&HandlerOptions{MaxLogs: 3})
	logger := slog.New(handler)

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "log", 0)
	for i := 0; i < 10; i++ {
		logger.Info("log", slog.Int("i", i))
	}

	require.Equal(t, 3, logs.Len(), "Expected only max log to be recorded.")
	assert.Equal(t, []LoggedRecord{
		{
			Record: record,
			Attrs:  []slog.Attr{slog.Int("i", 7)},
		},
		{
			Record: record,
			Attrs:  []slog.Attr{slog.Int("i", 8)},
		},
		{
			Record: record,
			Attrs:  []slog.Attr{slog.Int("i", 9)},
		},
	}, logs.AllUntimed())
}

func BenchmarkMaxLogs(b *testing.B) {
	handler, _ := New(&HandlerOptions{MaxLogs: 3})
	logger := slog.New(handler)

	// bench with the array shift via copy(o.logs[0:], o.logs[1:])
	// BenchmarkMaxLogs-8   	  186794	      5473 ns/op	     960 B/op	      20 allocs/op
	// bench with simple ring buffer implementation
	// BenchmarkMaxLogs-8   	  210639	      5466 ns/op	     960 B/op	      20 allocs/op
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			logger.Info("log", slog.Int("i", j))
		}
	}
}
