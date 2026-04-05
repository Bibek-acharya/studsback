package logger

import (
	"os"

	"studsphere/backend/internal/shared/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log     *zap.SugaredLogger
	BaseLog *zap.Logger
)

func Init() error {
	var level zapcore.Level
	switch config.AppConfig.GinMode {
	case "debug":
		level = zapcore.DebugLevel
	case "release":
		level = zapcore.InfoLevel
	default:
		level = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	BaseLog = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	Log = BaseLog.Sugar()

	return nil
}

func Sync() {
	if BaseLog != nil {
		_ = BaseLog.Sync()
	}
}

func WithFields(fields map[string]interface{}) *zap.SugaredLogger {
	if Log == nil {
		_ = Init()
	}
	args := []interface{}{}
	for k, v := range fields {
		args = append(args, k, v)
	}
	return Log.With(args...)
}

func Debug(msg string, args ...interface{}) {
	if Log == nil {
		_ = Init()
	}
	Log.Debugw(msg, args...)
}

func Info(msg string, args ...interface{}) {
	if Log == nil {
		_ = Init()
	}
	Log.Infow(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	if Log == nil {
		_ = Init()
	}
	Log.Warnw(msg, args...)
}

func Error(msg string, args ...interface{}) {
	if Log == nil {
		_ = Init()
	}
	Log.Errorw(msg, args...)
}

func Fatal(msg string, args ...interface{}) {
	if Log == nil {
		_ = Init()
	}
	Log.Fatalw(msg, args...)
}
