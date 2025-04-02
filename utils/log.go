package utils

import (
	"log"
	"time"

	"github.com/fatih/color"
)

func LogWithTimestamp(logColor func(format string, a ...interface{}) string, format string, a ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	log.Printf("[%s] %s", color.BlackString(timestamp), logColor(format, a...))
}

func LogInfo(format string, a ...interface{}) {
	LogWithTimestamp(color.WhiteString, format, a...)
}

func LogSuccess(format string, a ...interface{}) {
	LogWithTimestamp(color.GreenString, format, a...)
}

func LogWarning(format string, a ...interface{}) {
	LogWithTimestamp(color.YellowString, format, a...)
}

func LogError(format string, a ...interface{}) {
	LogWithTimestamp(color.RedString, format, a...)
}

func LogFatal(format string, a ...interface{}) {
	LogWithTimestamp(color.RedString, format, a...)
	log.Fatal()
}
