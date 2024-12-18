package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	logger   *log.Logger
	logLevel LogLevel

	// Color definitions
	debugColor = color.New(color.FgCyan)
	infoColor  = color.New(color.FgGreen)
	warnColor  = color.New(color.FgYellow)
	errorColor = color.New(color.FgRed)
	timeColor  = color.New(color.FgWhite)
)

// formatLog creates a pretty formatted log message
func formatLog(level string, format string, v ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, v...)

	// Format the timestamp
	timeStr := timeColor.Sprintf("[%s]", timestamp)

	// Format the level indicator with appropriate color
	var levelStr string
	switch level {
	case "DEBUG":
		levelStr = debugColor.Sprintf("%-7s", "[DEBUG]")
	case "INFO":
		levelStr = infoColor.Sprintf("%-7s", "[INFO]")
	case "WARN":
		levelStr = warnColor.Sprintf("%-7s", "[WARN]")
	case "ERROR":
		levelStr = errorColor.Sprintf("%-7s", "[ERROR]")
	}

	return fmt.Sprintf("%s %s %s", timeStr, levelStr, message)
}

// Init initializes the logger with the specified log level
func Init(level string) {
	logger = log.New(os.Stdout, "", 0) // Remove default prefix as we'll handle formatting
	setLogLevel(level)
}

// setLogLevel converts string level to LogLevel
func setLogLevel(level string) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		logLevel = DEBUG
	case "INFO":
		logLevel = INFO
	case "WARN":
		logLevel = WARN
	case "ERROR":
		logLevel = ERROR
	default:
		logLevel = INFO
	}
}

// Debugf logs debug messages
func Debugf(format string, v ...interface{}) {
	if logLevel <= DEBUG {
		logger.Println(formatLog("DEBUG", format, v...))
	}
}

// Infof logs info messages
func Infof(format string, v ...interface{}) {
	if logLevel <= INFO {
		logger.Println(formatLog("INFO", format, v...))
	}
}

// Warnf logs warning messages
func Warnf(format string, v ...interface{}) {
	if logLevel <= WARN {
		logger.Println(formatLog("WARN", format, v...))
	}
}

// Errorf logs error messages
func Errorf(format string, v ...interface{}) {
	if logLevel <= ERROR {
		logger.Println(formatLog("ERROR", format, v...))
	}
}

// GetLevel returns the current log level as a string
func GetLevel() string {
	switch logLevel {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
