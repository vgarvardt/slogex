package slogex

import "log/slog"

// DefaultErrorKey is the default error key.
const DefaultErrorKey = "error"

// ErrorKey is the error key that is used by the package. User may set it to own value on the package level.
var ErrorKey = DefaultErrorKey

// Error returns slog attribute with error key.
func Error(err error) slog.Attr {
	if err == nil {
		// return empty attr so that logger will filter this field out, like zap does
		return slog.Attr{}
	}

	return slog.String(ErrorKey, err.Error())
}
