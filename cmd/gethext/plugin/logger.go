package plugin

import (
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

type logger struct {
	tag string
}

func (l *logger) New(ctx ...interface{}) log.Logger {
	return log.Root().New(ctx)
}

func (l *logger) GetHandler() log.Handler {
	return log.Root().GetHandler()
}

func (l *logger) SetHandler(h log.Handler) {
	log.Root().SetHandler(h)
}

func (l *logger) Trace(msg string, ctx ...interface{}) {
	log.Root().Trace(fmt.Sprintf("[%s] %s", l.tag, msg), ctx...)
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	log.Root().Debug(fmt.Sprintf("[%s] %s", l.tag, msg), ctx...)
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	log.Root().Info(fmt.Sprintf("[%s] %s", l.tag, msg), ctx...)
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	log.Root().Warn(fmt.Sprintf("[%s] %s", l.tag, msg), ctx...)
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	log.Root().Error(fmt.Sprintf("[%s] %s", l.tag, msg), ctx...)
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	log.Root().Crit(fmt.Sprintf("[%s] %s", l.tag, msg), ctx...)
}

func newLogger(tag string) log.Logger {
	return &logger{tag}
}
