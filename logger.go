package logger

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	requestedLevel               = InfoLevel
	remoteServer                 = ""
	c              chan string   = nil
	stop           chan struct{} = nil
)

type LogLevel uint32

const (
	FatalLevel LogLevel = iota
	ErrorLevel
	InfoLevel
	DebugLevel
)

func (level LogLevel) String() string {
	switch level {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// EnableDebug increases logging, more verbose (debug)
func EnableDebug() {
	requestedLevel = DebugLevel
	formatMessage(InfoLevel, "Debug mode enabled")
}

// SetupRemoteServer enables pushing Error/Fatal logs to the remote server
// by sending POST http request
func SetupRemoteServer(url string) {
	if url == "" {
		return
	}

	Info("Enable pushing error/fatal logs to remote server %q", url)

	remoteServer = url
	c = make(chan string, 20)
	stop = make(chan struct{})

	go func() {
		tick := time.Tick(5 * time.Second)
		messages := make(map[string]struct{})
		send := func() {
			msgPack := ""
			for msg := range messages {
				msgPack += msg + "\n"
				delete(messages, msg)
			}
			if msgPack != "" {
				r, err := http.Post(remoteServer, "text/plain", strings.NewReader(msgPack[:len(msgPack)-1]))
				if err != nil {
					Error("Cannot push logs to remote server: %v", err)
				}
				r.Body.Close()
			}
		}

		for {
			select {
			case <-tick:
				send()
			case msg := <-c:
				messages[msg] = struct{}{}
			case <-stop:
				send()
				return
			}
		}
	}()
}

// Debug sends a debug log message.
func Debug(format string, v ...interface{}) {
	if requestedLevel >= DebugLevel {
		fmt.Fprintf(os.Stdout, formatMessage(DebugLevel, format, v...))
	}
}

// Info sends an info log message.
func Info(format string, v ...interface{}) {
	if requestedLevel >= InfoLevel {
		fmt.Fprintf(os.Stdout, formatMessage(InfoLevel, format, v...))
	}
}

// Error sends an error log message.
func Error(format string, v ...interface{}) {
	if requestedLevel >= ErrorLevel {
		msg := formatMessage(ErrorLevel, format, v...)
		Push(msg)
		fmt.Fprintf(os.Stderr, msg)
	}
}

// Fatal sends a fatal log message and stop the execution of the program.
func Fatal(format string, v ...interface{}) {
	if requestedLevel >= FatalLevel {
		msg := formatMessage(FatalLevel, format, v...)
		fmt.Fprintf(os.Stderr, msg)
		if remoteServer != "" {
			c <- msg
			stop <- struct{}{}
			time.Sleep(time.Second)
		}
		os.Exit(1)
	}
}

// Push pushs message to remote server(if have)
func Push(message string) {
	if remoteServer != "" {
		c <- message
	}
}

func formatMessage(level LogLevel, format string, v ...interface{}) string {
	prefix := fmt.Sprintf("[%s] [%s] ", time.Now().Format("2006-01-02T15:04:05"), level)
	return fmt.Sprintf(prefix+format+"\n", v...)
}
