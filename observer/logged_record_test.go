package observer

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggedEntryContextMap(t *testing.T) {
	tests := []struct {
		msg   string
		attrs []slog.Attr
		want  map[string]any
	}{
		{
			msg:   "no fields",
			attrs: nil,
			want:  map[string]any{},
		},
		{
			msg: "simple",
			attrs: []slog.Attr{
				slog.String("k1", "v"),
				slog.Int64("k2", 10),
			},
			want: map[string]any{
				"k1": "v",
				"k2": int64(10),
			},
		},
		{
			msg: "overwrite",
			attrs: []slog.Attr{
				slog.String("k1", "v1"),
				slog.String("k1", "v2"),
			},
			want: map[string]any{
				"k1": "v2",
			},
		},
		{
			msg: "nested",
			attrs: []slog.Attr{
				slog.String("k1", "v1"),
				slog.Group("nested", slog.String("k2", "v2")),
				slog.Group("level1", slog.Group("level2", slog.String("k3", "v3"))),
			},
			want: map[string]any{
				"k1": "v1",
				"nested": map[string]any{
					"k2": "v2",
				},
				"level1": map[string]any{
					"level2": map[string]any{
						"k3": "v3",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			entry := LoggedRecord{Attrs: tt.attrs}
			assert.Equal(t, tt.want, entry.AttrsMap())
		})
	}
}
