package logger

import (
	"context"
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
	WarnLevel
	InfoLevel
	DebugLevel
)

func (level LogLevel) String() string {
	switch level {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func init() {
	remoteServer = os.Getenv("LOGGER_REMOTE_SERVER")
	if remoteServer == "" {
		return
	}

	Info("Enable pushing error/fatal logs to remote server %q", remoteServer)

	c = make(chan string, 20)
	stop = make(chan struct{})

	go func() {
		tick := time.Tick(5 * time.Second)
		messages := make(map[string]struct{})
		send := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			msgPack := ""
			for msg := range messages {
				msgPack += msg + "\n"
				delete(messages, msg)
			}
			if msgPack != "" {
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, remoteServer, strings.NewReader(msgPack[:len(msgPack)-1]))
				r, err := http.DefaultClient.Do(req)
				if err != nil {
					Warn("Cannot push logs to remote server: %v", err)
					return
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
				for msg := range c {
					messages[msg] = struct{}{}
				}
				send()
				os.Exit(1)
			}
		}
	}()
}

// EnableDebug increases logging, more verbose (debug)
func EnableDebug() {
	requestedLevel = DebugLevel
	fmt.Fprintln(os.Stdout, formatMessage(InfoLevel, "Debug mode enabled"))
}

// Debug sends a debug log message.
func Debug(format string, v ...interface{}) {
	if requestedLevel >= DebugLevel {
		fmt.Fprintln(os.Stdout, formatMessage(DebugLevel, format, v...))
	}
}

// Info sends an info log message.
func Info(format string, v ...interface{}) {
	if requestedLevel >= InfoLevel {
		fmt.Fprintln(os.Stdout, formatMessage(InfoLevel, format, v...))
	}
}

// Warn sends a warn log message.
func Warn(format string, v ...interface{}) {
	if requestedLevel >= WarnLevel {
		fmt.Fprintln(os.Stderr, formatMessage(WarnLevel, format, v...))
	}
}

// Error sends an error log message.
func Error(format string, v ...interface{}) {
	if requestedLevel >= ErrorLevel {
		msg := formatMessage(ErrorLevel, format, v...)
		Push(msg)
		fmt.Fprintln(os.Stderr, msg)
	}
}

// Fatal sends a fatal log message and stop the execution of the program.
func Fatal(format string, v ...interface{}) {
	if requestedLevel >= FatalLevel {
		msg := formatMessage(FatalLevel, format, v...)
		fmt.Fprintln(os.Stderr, msg)
		if remoteServer != "" {
			c <- msg
			close(c)
			stop <- struct{}{}
		} else {
			os.Exit(1)
		}
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
	return fmt.Sprintf(prefix+format, v...)
}
