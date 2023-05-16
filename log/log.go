package log

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	ctxKey = struct{ name string }{"logger"}
	cfg    = zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			MessageKey:     "msg",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}
)

func FromContext(ctx context.Context) (context.Context, *zap.Logger) {
	if logger, ok := ctx.Value(ctxKey).(*zap.Logger); ok {
		return ctx, logger
	}
	logger, err := cfg.Build(zap.AddCaller())
	if err != nil {
		panic(err)
	}
	return WithLogger(ctx, logger), logger
}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey, logger)
}

const (
	appScope = "github.com/aereal/enjoy-gqlgen"
	gqlScope = "github.com/99designs/gqlgen/graphql"
)

func AppKey(name string, names ...string) string {
	return Key(appScope, name, names...)
}
func GraphQLKey(name string, names ...string) string {
	return Key(gqlScope, name, names...)
}

func Key(scope string, name string, names ...string) string {
	b := new(strings.Builder)
	b.WriteString(scope)
	switch numNames := len(names); numNames {
	case 0:
		b.WriteByte('.')
		b.WriteString(name)
	case 1:
		b.WriteByte('/')
		b.WriteString(name)
		b.WriteByte('.')
		b.WriteString(names[0])
	default:
		b.WriteByte('/')
		b.WriteString(name)
		for i, n := range names {
			var delimtier byte = '/'
			if i == numNames-1 {
				delimtier = '.'
			}
			b.WriteByte(delimtier)
			b.WriteString(n)
		}
	}
	return b.String()
}
