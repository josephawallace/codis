package log

import (
	"context"
	"github.com/rs/zerolog"
	"os"
	"strings"
	"time"
)

type Interface interface {
	Debug(message string, args ...interface{})
	Info(message string, args ...interface{})
	Warn(message string, args ...interface{})
	Error(err error)
	Fatal(err error)
	Panic(err error)
}

type Logger struct {
	logger *zerolog.Logger
}

func NewLogger() *Logger {
	skipFrameCount := 2

	//logRotator := lumberjack.Logger{
	//	Filename:   "./logs/log-" + strconv.Itoa(int(time.Now().Unix())) + ".log",
	//	MaxBackups: 7,
	//	MaxSize:    500,
	//	MaxAge:     10,
	//}

	//logger := zerolog.New(&logRotator).
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount + skipFrameCount).
		Logger()

	return &Logger{
		logger: &logger,
	}
}

func (l *Logger) Debug(message string, args ...interface{}) {
	l.log("debug", message, args...)
}

func (l *Logger) Info(message string, args ...interface{}) {
	l.log("info", message, args...)
}

func (l *Logger) Warn(message string, args ...interface{}) {
	l.log("warn", message, args...)
}

func (l *Logger) Error(err error) {
	l.err("error", err)
}

func (l *Logger) Fatal(err error) {
	l.err("fatal", err)
}

func (l *Logger) Panic(err error) {
	l.err("panic", err)
}

func (l *Logger) WithCtx(ctx context.Context) context.Context {
	return l.logger.WithContext(ctx)
}

func (l *Logger) log(level string, message string, args ...interface{}) {
	switch strings.ToLower(level) {
	case "info":
		l.logger.Info().Msgf(message, args...)
	case "warn":
		l.logger.Warn().Msgf(message, args...)
	case "debug":
		l.logger.Debug().Msgf(message, args...)
	}
}

func (l *Logger) err(level string, err error) {
	switch level {
	case "error":
		l.logger.Error().Err(err).Send()
	case "fatal":
		l.logger.Fatal().Err(err).Send()
	case "panic":
		l.logger.Panic().Err(err).Send()
	default:
		l.logger.Error().Msg("invalid log level!")
	}
}

type ctxKey struct{}

func Ctx(ctx context.Context) *Logger {
	defaultContextLogger := &Logger{logger: zerolog.DefaultContextLogger}

	nop := zerolog.Nop()
	disabledLogger := &Logger{logger: &nop}

	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return l
	} else if l = defaultContextLogger; l != nil {
		return l
	}
	return disabledLogger
}
