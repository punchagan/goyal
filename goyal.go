package main

import (
	"encoding/json"
	"fmt"
	"github.com/thoj/go-ircevent"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	TIME_MESSAGE_FORMAT = "Jan 2 2006 15:04:05"
	TIME_FILE_FORMAT    = "2006-01-02"
	LOG_FILE_DAYS       = 15
)

type IRCConfig struct {
	Nick     string
	Username string
	Server   string
	Channels []string
	LogDir   string
	LogFiles map[string]*os.File
}

func main() {
	config, err := getConfig()
	if err != nil {
		fmt.Println(err)
		return
	}

	connection := irc.IRC(config.Nick, config.Username)
	connection.UseTLS = true

	defer closeLogFiles(config)
	addCallbacks(connection, config)

	//Connect to the server
	err = connection.Connect(config.Server)
	if err != nil {
		fmt.Println("Failed to connect.")
		return
	}

	// Connect and wait
	connection.Loop()
}

func addCallbacks(connection *irc.Connection, config IRCConfig) {

	// Join the channels
	connection.AddCallback("001", func(e *irc.Event) {
		for _, channel := range config.Channels {
			connection.Join(channel)
		}
	})

	connection.AddCallback("JOIN", func(e *irc.Event) {
		channel := e.Arguments[0]
		var message string

		if e.Nick == config.Nick {
			message = fmt.Sprintf("Hello, I'm yet another logbot written in Go.")
		} else {
			message = fmt.Sprintf("Hello %s, Welcome to %s!", e.Nick, channel)
		}

		connection.Privmsg(channel, message)
		logMessage(&config, channel, "%s entered %s", e.Nick, channel)
	})

	connection.AddCallback("PRIVMSG", func(e *irc.Event) {
		channel := e.Arguments[0]
		switch channel {
		case config.Nick:
			connection.Privmsg(e.Nick, "Sorry, I don't accept direct messages!")
		default:
			logMessage(&config, channel, "%s: %s", e.Nick, e.Message())
		}
	})

	connection.AddCallback("CTCP_ACTION", func(e *irc.Event) {
		channel := e.Arguments[0]
		logMessage(&config, channel, "***%s %s", e.Nick, e.Message())
	})

	connection.AddCallback("PART", func(e *irc.Event) {
		channel := e.Arguments[0]
		logMessage(&config, channel, "%s left %s", e.Nick, channel)
	})

	connection.AddCallback("QUIT", func(e *irc.Event) {
		channel := e.Arguments[0]
		logMessage(&config, channel, "%s quit IRC.", e.Nick)
	})

}

func getConfig() (config IRCConfig, err error) {
	data, err := ioutil.ReadFile("goyal-config.json")
	if err != nil {
		return IRCConfig{}, fmt.Errorf("Could not read config file.\n")
	}

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&config); err == io.EOF {
		// Return config ...
	} else if err != nil {
		return IRCConfig{}, fmt.Errorf("Could not read config file.\n")
	}

	config.LogFiles = make(map[string]*os.File)

	return config, nil
}

func closeLogFiles(config IRCConfig) {
	for _, logFile := range config.LogFiles {
		logFile.Close()
	}
}

func logMessage(config *IRCConfig, channel string, format string, args ...interface{}) {
	now := time.Now().UTC()
	logFile := getLogFile(config, channel, now)
	message := fmt.Sprintf("<%s> %s\n", now.Format(TIME_MESSAGE_FORMAT), format)
	message = fmt.Sprintf(message, args...)

	_, err := logFile.WriteString(message)
	if err != nil {
		panic(fmt.Sprintf("Writing is failing %+v\n", err))
	}

}

func getLogFile(config *IRCConfig, channel string, now time.Time) *os.File {
	today := now.Format(TIME_FILE_FORMAT)
	logFileName := path.Join(config.LogDir, fmt.Sprintf("%s-%s.txt", channel, today))
	symLinkName := path.Join(config.LogDir, fmt.Sprintf("%s-%s.txt", channel, "log"))
	var err error

	// FIXME: maps are not safe to use concurrently
	logFile, ok := config.LogFiles[channel]
	switch {

	case ok && logFileName == logFile.Name():
		// Nothing to do.

	default:
		// If existing file open, close it.
		if ok {
			logFile.Close()
		}

		logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(fmt.Sprintf("Log file opening failed: %+v\n", err))
		}

		// FIXME: maps are not safe to use concurrently, but this
		// probably doesn't matter.
		config.LogFiles[channel] = logFile
		os.Symlink(logFileName, symLinkName)

		// Delete unnecessary files, if any.
		cleanUpLogs(config.LogDir, now, config.Channels)
	}

	return logFile
}

func cleanUpLogs(dir string, now time.Time, channels []string) {
	re_string := fmt.Sprintf("%s-(?P<date>\\d{4}-\\d{2}-\\d{2}).txt", strings.Join(channels, "|"))
	re := regexp.MustCompile(re_string)
	files, _ := ioutil.ReadDir(dir)
	for _, f := range files {
		match := re.FindStringSubmatch(f.Name())
		if match == nil {
			continue
		}

		t, _ := time.Parse(TIME_FILE_FORMAT, match[1])
		if !now.After(t.AddDate(0, 0, LOG_FILE_DAYS)) {
			continue
		}

		fpath := path.Join(dir, f.Name())
		fmt.Printf("Deleting log file %s\n", fpath)
		err := os.Remove(fpath)
		if err != nil {
			fmt.Println(err)
		}

	}
}
