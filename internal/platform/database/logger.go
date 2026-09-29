package database

import (
	"context"
	"log"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

type safeSQLLogger struct {
	level gormlogger.LogLevel
	slow  time.Duration
}

func (l safeSQLLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.level = level
	return l
}
func (l safeSQLLogger) Info(context.Context, string, ...interface{}) {}
func (l safeSQLLogger) Warn(context.Context, string, ...interface{}) {}
func (l safeSQLLogger) Error(context.Context, string, ...interface{}) {
	if l.level >= gormlogger.Error {
		log.Print("database operation failed")
	}
}
func (l safeSQLLogger) Trace(_ context.Context, begin time.Time, _ func() (string, int64), err error) {
	if err != nil && l.level >= gormlogger.Error {
		log.Print("database operation failed")
		return
	}
	if time.Since(begin) > l.slow && l.level >= gormlogger.Warn {
		log.Print("database slow operation")
	}
}
