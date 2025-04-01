package utils

import (
	"fmt"
	"log"

	"github.com/fatih/color"
)

func LogWithTimestamp(logColor func(format string, a ...interface{}) string, skipTimestamp bool, format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	log.Print(logColor(message))
}

func LogInfo(format string, a ...interface{}) {
	LogWithTimestamp(color.WhiteString, false, format, a...)
}

func LogSuccess(format string, a ...interface{}) {
	LogWithTimestamp(color.GreenString, false, format, a...)
}

func LogWarning(format string, a ...interface{}) {
	LogWithTimestamp(color.YellowString, false, format, a...)
}

func LogError(format string, a ...interface{}) {
	LogWithTimestamp(color.RedString, false, format, a...)
}

func LogFatal(format string, a ...interface{}) {
	LogWithTimestamp(color.RedString, false, format, a...)
	log.Fatal()
}
