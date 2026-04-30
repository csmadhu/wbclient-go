package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	enableDebugLog = (os.Getenv("ENABLE_DEBUG_LOG") == "true")
)

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](values ...T) Set[T] {
	s := make(Set[T])
	s.Add(values...)
	return s
}

func (s Set[T]) Add(values ...T) {
	for _, value := range values {
		s[value] = struct{}{}
	}
}

func (s Set[T]) Has(value T) bool {
	_, ok := s[value]
	return ok
}

type Logger interface {
	Printf(format string, l ...any)
	Infof(format string, l ...any)
	Debugf(format string, l ...any)
	Errorf(format string, l ...any)
	Warnf(format string, l ...any)
	Warningf(format string, l ...any)
	Fatalf(format string, l ...any)

	With(prefixFields ...string) Logger
	T(tenantID string, labels ...string) Logger

	WithCtx(ctx context.Context, labels ...string) Logger
	TWithCtx(ctx context.Context, tenantID string, labels ...string) Logger

	InfoWriter() io.Writer
	ErrWriter() io.Writer
}

var (
	globalLogger = newLogger()
)

func newLogger() Logger {
	encoding := os.Getenv("LOG_ENCODING")
	if encoding == "" {
		encoding = "text"
	}
	switch strings.ToLower(encoding) {
	case "json":
		return NewJSONLogger(nil)
	default:
		return NewTextLogger()
	}
}

func Printf(format string, l ...any) {
	if false {
		_ = fmt.Sprintf(format, l...) // enable printf checker
	}
	globalLogger.Printf(format, l...)
}

func Debugf(format string, l ...any) {
	if false {
		_ = fmt.Sprintf(format, l...) // enable printf checker
	}
	globalLogger.Debugf(format, l...)
}

func Errorf(format string, l ...any) {
	if false {
		_ = fmt.Sprintf(format, l...) // enable printf checker
	}
	globalLogger.Errorf(format, l...)
}

func Warnf(format string, l ...any) {
	if false {
		_ = fmt.Sprintf(format, l...) // enable printf checker
	}
	globalLogger.Warnf(format, l...)
}

func Fatalf(format string, l ...any) {
	if false {
		_ = fmt.Sprintf(format, l...) // enable printf checker
	}

	globalLogger.Fatalf(format, l...)
}

// T returns a logger which prefixes tenantID and optional fields to log messages.
// Example:
//
//	log.T("123e4567", "r", "R-100").Printf("radius - completed transaction")
//	               prints
//	2022/04/14 10:23:09 INFO [t=123e4567 r=R-100] radius - completed transaction
func T(tenantID string, labels ...string) Logger {
	return globalLogger.T(tenantID, labels...)
}

func TWithCtx(ctx context.Context, tenantID string, labels ...string) Logger {
	return globalLogger.TWithCtx(ctx, tenantID, labels...)
}

// With is used to get a Logger with prefix fields set.
// Example:
//
//		log := log.With("r", "R-100", "go", "6")
//		log.Errorf("Failed to do user lookup")
//	           prints
//	    2022/04/14 10:23:09 ERROR [r=R-100 go=6] Failed to do user lookup
func With(labels ...string) Logger {
	return globalLogger.With(labels...)
}

func WithCtx(ctx context.Context, labels ...string) Logger {
	return globalLogger.WithCtx(ctx, labels...)
}

func NewLogger() Logger {
	return globalLogger.WithCtx(context.Background())
}

// makePrefix combines given fields to prefix string - [fields1=value1 field2=value2...]
func makeLogPrefix(labels []string) string {
	if len(labels) == 0 {
		return ""
	}

	fields := NewSet[string]()

	var parts []string
	for i := 0; i < len(labels); i += 2 {
		key := labels[i]
		value := labels[i+1]

		if fields.Has(key) {
			continue
		}

		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		fields.Add(key)
	}

	return fmt.Sprintf("[%s] ", strings.Join(parts, " "))
}

func UpdateDebugLogSetting(enable bool) {
	enableDebugLog = enable
}

// IsDebugLogEnabled returns whether debug log is enabled
// mainly used for testing purpose.
func IsDebugLogEnabled() bool {
	return enableDebugLog
}
