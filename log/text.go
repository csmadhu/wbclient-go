package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

var (
	infoLogger = log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	errLogger  = log.New(os.Stderr, "", log.LstdFlags|log.LUTC)
)

type TextLogger struct {
	prefix string
	labels []string
	info   *log.Logger
	err    *log.Logger
}

type TextLoggerOption func(*TextLogger)

func WithTextLoggerLabels(labels ...string) TextLoggerOption {
	return func(l *TextLogger) {
		l.labels = labels
	}
}

func WithTextLoggerInfoWriter(w io.Writer) TextLoggerOption {
	return func(l *TextLogger) {
		l.info = log.New(w, "", log.LstdFlags|log.LUTC)
	}
}

func WithTextLoggerErrorWriter(w io.Writer) TextLoggerOption {
	return func(l *TextLogger) {
		l.err = log.New(w, "", log.LstdFlags|log.LUTC)
	}
}

func WithTextLoggerMakeDefault() TextLoggerOption {
	return func(l *TextLogger) {
		globalLogger = l
	}
}

func NewTextLogger(opts ...TextLoggerOption) Logger {
	logger := &TextLogger{
		info: infoLogger,
		err:  errLogger,
	}

	for _, opt := range opts {
		opt(logger)
	}

	logger.prefix = makeLogPrefix(logger.labels)
	return logger
}

func (l *TextLogger) With(labels ...string) Logger {
	return l.WithCtx(context.Background(), labels...)
}

func (l *TextLogger) WithCtx(ctx context.Context, labels ...string) Logger {
	logLabels := l.labels
	logLabels = append(logLabels, labelFieldsFromCtx(ctx)...)
	logLabels = append(logLabels, labels...)

	mustValidateLabels(logLabels...)

	return NewTextLogger(WithTextLoggerLabels(logLabels...), WithTextLoggerInfoWriter(l.info.Writer()), WithTextLoggerErrorWriter(l.err.Writer()))
}

func (l *TextLogger) T(tenantID string, labels ...string) Logger {
	return l.TWithCtx(context.Background(), tenantID, labels...)
}

func (l *TextLogger) TWithCtx(ctx context.Context, tenantID string, labels ...string) Logger {
	labels = append(labels, "t", tenantID)
	return l.WithCtx(ctx, labels...)
}

func (l *TextLogger) Printf(format string, args ...any) {
	l.info.Printf("INFO %s", fmt.Sprintf(l.prefix+format, args...))
}

func (l *TextLogger) Infof(format string, args ...any) {
	l.info.Printf("INFO %s", fmt.Sprintf(l.prefix+format, args...))
}

func (l *TextLogger) Debugf(format string, args ...any) {
	if enableDebugLog {
		l.info.Printf("DEBUG %s", fmt.Sprintf(l.prefix+format, args...))
	}
}

func (l *TextLogger) Errorf(format string, args ...any) {
	l.err.Printf("ERROR %s", fmt.Sprintf(l.prefix+format, args...))
}

func (l *TextLogger) Warnf(format string, args ...any) {
	l.err.Printf("WARN %s", fmt.Sprintf(l.prefix+format, args...))
}

func (l *TextLogger) Warningf(format string, args ...any) {
	l.err.Printf("WARN %s", fmt.Sprintf(l.prefix+format, args...))
}

func (l *TextLogger) Fatalf(format string, args ...any) {
	l.err.Fatalf("FATAL %s", fmt.Sprintf(l.prefix+format, args...))
}

func (l *TextLogger) InfoWriter() io.Writer {
	return newWriter(l, true)
}

func (l *TextLogger) ErrWriter() io.Writer {
	return newWriter(l, false)
}

type writer struct {
	l      *TextLogger
	isInfo bool
}

func newWriter(l *TextLogger, isInfo bool) *writer {
	return &writer{
		l:      l,
		isInfo: isInfo,
	}
}

func (w *writer) Write(p []byte) (n int, err error) {
	if w.isInfo {
		w.l.info.Printf("INFO %s", fmt.Sprintf(w.l.prefix+string(p)))
		return len(p), nil
	}

	w.l.err.Printf("ERROR %s", fmt.Sprintf(w.l.prefix+string(p)))
	return len(p), nil
}
