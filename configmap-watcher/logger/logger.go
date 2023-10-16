package logger

import (
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type customEncoder struct {
	zapcore.Encoder
	Source string
}

// Clone clones a zapencoder and satisfies the interface
func (e *customEncoder) Clone() zapcore.Encoder {
	return &customEncoder{
		Encoder: e.Encoder.Clone(),
		Source:  e.Source,
	}
}

// EncodeEntry allows us to add additional fields to every logger entry
func (e *customEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) { //nolint:all
	fields = append(
		fields,
		zap.String("msg", entry.Message),
		zap.String("file", entry.Caller.TrimmedPath()),
		zap.String("level", entry.Level.String()),
	)
	return e.Encoder.EncodeEntry(entry, fields)
}

// SetupLogger allows us to setup a zaplogger
func SetupLogger(w io.Writer, source string) *zap.Logger {
	ec := zapcore.EncoderConfig{
		NameKey:        "configmap-watcher",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		TimeKey:        "ts",
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	// Create the logger with the custom encoder.
	return zap.New(zapcore.NewCore(
		&customEncoder{
			Encoder: zapcore.NewJSONEncoder(ec),
			Source:  source,
		},
		zapcore.AddSync(w),
		zap.DebugLevel,
	), zap.AddCaller())
}
