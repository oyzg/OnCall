package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Logger struct {
	base  *log.Logger
	level string
}

func New(level string) *Logger {
	return &Logger{
		base:  log.New(os.Stdout, "", log.LstdFlags),
		level: strings.ToUpper(level),
	}
}

func (l *Logger) Info(message string, keyvals ...any) {
	l.log("INFO", message, keyvals...)
}

func (l *Logger) Error(message string, keyvals ...any) {
	l.log("ERROR", message, keyvals...)
}

func (l *Logger) log(level, message string, keyvals ...any) {
	if level == "INFO" && l.level == "ERROR" {
		return
	}

	payload := ""
	if len(keyvals) > 0 {
		payload = " " + formatKeyvals(keyvals...)
	}

	l.base.Printf("[%s] %s%s", level, message, payload)
}

func formatKeyvals(keyvals ...any) string {
	parts := make([]string, 0, len(keyvals)/2)

	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprintf("field_%d", i)
		if i < len(keyvals) {
			key = fmt.Sprint(keyvals[i])
		}

		var value any = ""
		if i+1 < len(keyvals) {
			value = keyvals[i+1]
		}

		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}

	return strings.Join(parts, " ")
}
