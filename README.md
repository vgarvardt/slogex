# slogex

[![GoDev](https://img.shields.io/static/v1?label=godev&message=reference&color=00add8)](https://pkg.go.dev/github.com/vgarvardt/slogex)
[![Coverage Status](https://codecov.io/gh/vgarvardt/slogex/branch/master/graph/badge.svg)](https://codecov.io/gh/vgarvardt/slogex)
[![ReportCard](https://goreportcard.com/badge/github.com/vgarvardt/slogex)](https://goreportcard.com/report/github.com/vgarvardt/slogex)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/license/apache-2-0/)

Collection of Golang `log/slog` logger extensions and related wrappers.

## Extensions

- [`github.com/vgarvardt/slogex/observer`](#githubcomvgarvardtslogexobserver) - `slog.Handler` implementation that keeps
  log records in memory. Useful for applications that want to test log output. Heavily inspired
  by `go.uber.org/zap/zaptest/observer`.
- [`github.com/vgarvardt/slogex/fxlogger`](#githubcomvgarvardtslogexfxlogger) - `go.uber.org/fx/fxevent.Logger`
  implementation.

## Examples

### `github.com/vgarvardt/slogex/observer`

```go
package something_test

import (
    "log/slog"
    "testing"

    "github.com/vgarvardt/slogex/observer"
)

func TestSomeLogs(t *testing.T) {
    handler, logs := observer.New(nil)

    logger := slog.New(handler).With(slog.Int("i", 1))
    logger.Info("foo")

    loggerRecords := logs.All()
    for _, r := range loggerRecords {
        t.Log(r.Record.Level, r.Record.Message, r.Attrs)
    }
}

```

## `github.com/vgarvardt/slogex/fxlogger`

```go
package main

import (
    "log/slog"

    "go.uber.org/fx"
    "go.uber.org/fx/fxevent"

    "github.com/vgarvardt/slogex/fxlogger"
)

func FxOptions() []fx.Option {
    return []fx.Option{
        fx.WithLogger(func(logger *slog.Logger) fxevent.Logger {
            return &fxlogger.Logger{
                Logger: logger.With(slog.String("source", "fx")),
            }
        }),
    }
}

```
