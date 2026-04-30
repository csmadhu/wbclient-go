package log

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getGCPEncoderCfg() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.LevelKey = "severity"
	encoderCfg.MessageKey = "message"
	encoderCfg.EncodeLevel = gcpLogLevelEncoding()
	encoderCfg.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder
	encoderCfg.EncodeCaller = zapcore.FullCallerEncoder
	return encoderCfg
}

func gcpLogLevelEncoding() zapcore.LevelEncoder {
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

func makeGCPLabelField(labels map[string]string) zapcore.Field {
	return zapcore.Field{
		Key:       "logging.googleapis.com/labels",
		Type:      zapcore.ReflectType,
		Interface: labels,
	}
}

func makeGCPTraceField(traceID string) zapcore.Field {
	return zapcore.Field{
		Key:    "logging.googleapis.com/trace",
		Type:   zapcore.StringType,
		String: fmt.Sprintf("projects/%s/traces/%s", os.Getenv("PROJECT_ID"), traceID),
	}
}

func makeGCPSpanField(spanID string) zapcore.Field {
	return zapcore.Field{
		Key:    "logging.googleapis.com/spanId",
		Type:   zapcore.StringType,
		String: spanID,
	}
}

func makeGCPSampledField(isSampled bool) zapcore.Field {
	return zapcore.Field{
		Key:       "logging.googleapis.com/trace_sampled",
		Type:      zapcore.BoolType,
		Interface: isSampled,
	}
}
