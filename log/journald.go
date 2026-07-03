package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"

	"go.uber.org/zap/zapcore"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
)

var (
	journaldBufferPool = buffer.NewPool()
)

type JournaldLogger struct {
	parentLogger *zap.Logger
	logger       *zap.Logger
	prefix       string
	labels       []string
}

func NewJournaldLogger(labels []string) Logger {
	parentLogger, err := getJournaldCfg().Build(zap.AddCallerSkip(1))
	if err != nil {
		log.Fatalf("failed to init logger err=%v", err)
	}

	journaldLogger := &JournaldLogger{
		parentLogger: parentLogger,
	}

	return journaldLogger.With(labels...)
}

func (l *JournaldLogger) With(labels ...string) Logger {
	return l.WithCtx(context.Background(), labels...)
}

func (l *JournaldLogger) WithCtx(ctx context.Context, labels ...string) Logger {
	logLabels := l.labels
	logLabels = append(logLabels, labelFieldsFromCtx(ctx)...)
	logLabels = append(logLabels, labels...)

	mustValidateLabels(logLabels...)

	logLabels = sortLogLabels(logLabels)

	logger := l.parentLogger.With(makeJournaldZapFields(ctx, logLabels)...)

	return &JournaldLogger{
		parentLogger: l.parentLogger,
		logger:       logger,
		labels:       logLabels,
		prefix:       makeLogPrefix(logLabels),
	}
}

func sortLogLabels(labels []string) []string {
	if len(labels)%2 != 0 {
		log.Printf("error: log labels[%v] should be of even count", labels)
		return nil
	}
	type keyIndex struct {
		key   string
		index int
	}
	keys := make([]keyIndex, 0, len(labels)/2)
	for i := 0; i < len(labels); i += 2 {
		keys = append(keys, keyIndex{labels[i], i})
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].key < keys[j].key
	})
	var sortedLabels []string
	for _, key := range keys {
		sortedLabels = append(sortedLabels, key.key, labels[key.index+1])
	}
	return sortedLabels
}

func (l *JournaldLogger) T(tenantID string, labels ...string) Logger {
	return l.TWithCtx(context.Background(), tenantID, labels...)
}

func (l *JournaldLogger) TWithCtx(ctx context.Context, tenantID string, labels ...string) Logger {
	labels = append([]string{"t", tenantID}, labels...)
	return l.WithCtx(ctx, labels...)
}

func (l *JournaldLogger) Printf(format string, args ...any) {
	if !IsInfoLogEnabled() {
		return
	}

	l.logger.Info(l.makeMsg(format, args...))
}

func (l *JournaldLogger) Infof(format string, args ...any) {
	if !IsInfoLogEnabled() {
		return
	}

	l.logger.Info(l.makeMsg(format, args...))
}

// turn off debug logging for on-prem
func (l *JournaldLogger) Debugf(format string, args ...any) {
	if !IsDebugLogEnabled() {
		return
	}

	l.logger.Debug(l.makeMsg(format, args...))
}

func (l *JournaldLogger) Errorf(format string, args ...any) {
	if !IsErrorLogEnabled() {
		return
	}

	l.logger.Error(l.makeMsg(format, args...))
}

func (l *JournaldLogger) Warnf(format string, args ...any) {
	if !IsWarnLogEnabled() {
		return
	}

	l.logger.Warn(l.makeMsg(format, args...))
}

func (l *JournaldLogger) Warningf(format string, args ...any) {
	if !IsWarnLogEnabled() {
		return
	}

	l.logger.Warn(l.makeMsg(format, args...))
}

func (l *JournaldLogger) Fatalf(format string, args ...any) {
	l.logger.Fatal(l.makeMsg(format, args...))
}

func (l *JournaldLogger) makeMsg(format string, args ...any) string {
	msg := journaldBufferPool.Get()
	msg.AppendString(fmt.Sprintf(l.prefix+format, args...))

	message := msg.String()
	msg.Free()
	return message
}

func (l *JournaldLogger) InfoWriter() io.Writer {
	return nil
}

func (l *JournaldLogger) ErrWriter() io.Writer {
	return nil
}

func getJournaldCfg() zap.Config {
	logLevel := zap.DebugLevel

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(logLevel)
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.EncoderConfig = getJournaldEncoderCfg()
	return cfg
}

func makeJournaldZapFields(ctx context.Context, labels []string) (fields []zapcore.Field) {
	if field := getJournaldLabelField(ctx, labels); !field.Equals(emptyZapField) {
		fields = append(fields, field)
	}

	return fields
}

func getJournaldLabelField(ctx context.Context, labels []string) zapcore.Field {
	labelFields := LabelsFromCtx(ctx)

	if len(labels) != 0 && len(labels)%2 == 0 {
		for i := 0; i < len(labels); i += 2 {
			labelFields[labels[i]] = labels[i+1]
		}
	}

	if len(labels) == 0 {
		return emptyZapField
	}
	return makeJournaldLabelField(labelFields)
}

func makeJournaldLabelField(labels map[string]string) zapcore.Field {
	return zapcore.Field{
		Key:       "labels",
		Type:      zapcore.ReflectType,
		Interface: labels,
	}
}

func getJournaldEncoderCfg() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.LevelKey = "severity"
	encoderCfg.MessageKey = "message"
	encoderCfg.EncodeLevel = journaldLogLevelEncoding()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder
	encoderCfg.EncodeCaller = zapcore.FullCallerEncoder
	return encoderCfg
}

func journaldLogLevelEncoding() zapcore.LevelEncoder {
	return func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		switch l {
		case zapcore.DebugLevel:
			enc.AppendString("DEBUG")
		case zapcore.InfoLevel:
			enc.AppendString("INFO")
		case zapcore.WarnLevel:
			enc.AppendString("WARNING")
		case zapcore.ErrorLevel:
			enc.AppendString("ERROR")
		case zapcore.DPanicLevel:
			enc.AppendString("CRITICAL")
		case zapcore.PanicLevel:
			enc.AppendString("ALERT")
		case zapcore.FatalLevel:
			enc.AppendString("EMERGENCY")
		}
	}
}
