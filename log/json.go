package log

import (
	"context"
	"fmt"
	"io"
	"log"

	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var (
	bufferPool = buffer.NewPool()
)

type JSONLogger struct {
	logger *zap.Logger
	fields []zap.Field
	prefix string
	labels []string
}

func NewJSONLogger(labels []string) Logger {
	logger, err := getProdCfg().Build(zap.AddCallerSkip(1))
	if err != nil {
		log.Fatalf("failed to init logger err=%v", err)
	}

	jsonLogger := &JSONLogger{
		logger: logger,
	}

	return jsonLogger.With(labels...)
}

func (l *JSONLogger) With(labels ...string) Logger {
	return l.WithCtx(context.Background(), labels...)
}

func (l *JSONLogger) WithCtx(ctx context.Context, labels ...string) Logger {
	logLabels := l.labels
	logLabels = append(logLabels, labelFieldsFromCtx(ctx)...)
	logLabels = append(logLabels, labels...)

	mustValidateLabels(logLabels...)

	return &JSONLogger{
		logger: l.logger,
		labels: logLabels,
		fields: makeZapFields(ctx, logLabels),
		prefix: makeLogPrefix(logLabels),
	}
}

func (l *JSONLogger) T(tenantID string, labels ...string) Logger {
	return l.TWithCtx(context.Background(), tenantID, labels...)
}

func (l *JSONLogger) TWithCtx(ctx context.Context, tenantID string, labels ...string) Logger {
	labels = append([]string{"t", tenantID}, labels...)
	return l.WithCtx(ctx, labels...)
}

func (l *JSONLogger) Printf(format string, args ...any) {
	l.logger.Info(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) Infof(format string, args ...any) {
	l.logger.Info(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) Debugf(format string, args ...any) {
	l.logger.Debug(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) Errorf(format string, args ...any) {
	l.logger.Error(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) Warnf(format string, args ...any) {
	l.logger.Warn(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) Warningf(format string, args ...any) {
	l.logger.Warn(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) Fatalf(format string, args ...any) {
	l.logger.Fatal(l.makeMsg(format, args...), l.fields...)
}

func (l *JSONLogger) makeMsg(format string, args ...any) string {
	msg := bufferPool.Get()
	msg.AppendString(fmt.Sprintf(l.prefix+format, args...))

	message := msg.String()
	msg.Free()
	return message
}

func (l *JSONLogger) InfoWriter() io.Writer {
	return nil
}

func (l *JSONLogger) ErrWriter() io.Writer {
	return nil
}

func getProdCfg() zap.Config {
	logLevel := zap.InfoLevel
	if enableDebugLog {
		logLevel = zap.DebugLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(logLevel)
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.EncoderConfig = getGCPEncoderCfg()
	return cfg
}

var emptyZapField zapcore.Field

func makeZapFields(ctx context.Context, labels []string) (fields []zapcore.Field) {
	fields = append(fields, getTraceFields(ctx)...)

	if field := getLabelField(ctx, labels); !field.Equals(emptyZapField) {
		fields = append(fields, field)
	}

	return fields
}

func getTraceFields(ctx context.Context) (fields []zapcore.Field) {
	otelSpan := oteltrace.SpanFromContext(ctx)
	if !otelSpan.IsRecording() {
		return
	}

	spanCtx := otelSpan.SpanContext()
	if spanCtx.HasTraceID() {
		fields = append(fields, makeGCPTraceField(spanCtx.TraceID().String()))
	}

	if spanCtx.HasSpanID() {
		fields = append(fields, makeGCPSpanField(spanCtx.SpanID().String()))
	}

	fields = append(fields, makeGCPSampledField(spanCtx.IsSampled()))

	return fields
}

func getLabelField(ctx context.Context, labels []string) zapcore.Field {
	labelFields := LabelsFromCtx(ctx)

	if len(labels) != 0 && len(labels)%2 == 0 {
		for i := 0; i < len(labels); i += 2 {
			labelFields[labels[i]] = labels[i+1]
		}
	}

	if len(labels) == 0 {
		return emptyZapField
	}
	return makeGCPLabelField(labelFields)
}
