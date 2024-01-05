package observer

import "log/slog"

// An LoggedRecord is a log record representation suitable for direct comparison.
// Record attributes are extracted to a list to allow comparing them.
type LoggedRecord struct {
	Record slog.Record
	Attrs  []slog.Attr
}

// AttrsMap returns a map for all attributes in the log record.
// Groups are recursively converted to maps.
func (e LoggedRecord) AttrsMap() map[string]any {
	return e.attrsMap(e.Attrs)
}

func (e LoggedRecord) attrsMap(attrs []slog.Attr) map[string]any {
	res := make(map[string]any, len(attrs))
	for _, a := range attrs {
		if a.Key == "" {
			// TODO: implement the full logic that ensures that the value is empty as well.
			//  The logic is implemented in Attr.isEmpty() method that is private and it checks private field values.
			continue
		}

		if a.Value.Kind() == slog.KindGroup {
			res[a.Key] = e.attrsMap(a.Value.Group())
			continue
		}
		res[a.Key] = a.Value.Any()
	}
	return res
}
