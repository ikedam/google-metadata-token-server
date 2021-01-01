// Package log is the package wrapping logrus
package log

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var (
	// Logger is the logger used in gtokenserver. You can replace or configure it as you like.
	Logger *logrus.Logger
)

func init() {
	Logger = logrus.New()
}

// Debug outputs debug logs.
func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

// Debugf outputs debug logs.
func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

// Error outputs debug logs.
func Error(args ...interface{}) {
	Logger.Error(args...)
}

// Errorf outputs error logs.
func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

// Exit runs exit hook handlers of logrus and exit the program.
func Exit(code int) {
	Logger.Exit(code)
}

// Info output information logs.
func Info(args ...interface{}) {
	Logger.Info(args...)
}

// Infof outputs information logs.
func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

// Trace output trace logs.
func Trace(args ...interface{}) {
	Logger.Trace(args...)
}

// Tracef outputs trace logs.
func Tracef(format string, args ...interface{}) {
	Logger.Tracef(format, args...)
}

// Warning outputs warning logs.
func Warning(args ...interface{}) {
	Logger.Warning(args...)
}

// Warningf outputs warning logs.
func Warningf(format string, args ...interface{}) {
	Logger.Warningf(format, args...)
}

// WithError parepares log outputs with the specified error.
func WithError(err error) *logrus.Entry {
	return Logger.WithError(err)
}

// WithField prepares log outputs with the specified field.
func WithField(key string, value interface{}) *logrus.Entry {
	return Logger.WithField(key, value)

}

// SetLevelByName configures the log level with specified name.
func SetLevelByName(level string) error {
	l, err := logrus.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("Invalid level name %v: %w", level, err)
	}
	Logger.SetLevel(l)
	return nil
}
