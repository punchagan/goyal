package main

import (
	"encoding/json"
	"fmt"
	"github.com/thoj/go-ircevent"
	"io"
	"io/ioutil"
	"os"
	"strings"
 	"time"
)

const (
	TIME_MESSAGE_FORMAT   = "Jan 2 2006 15:04:05"
	TIME_FILE_FORMAT   = "2006-01-02"
)

type IRCConfig struct {
	Nick        string
	Username    string
	Server      string
	Channels    []string
	LogFileDir  string
	LogFiles    map[string]*os.File
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
	now := time.Now().UTC().Format(TIME_MESSAGE_FORMAT)
	today := time.Now().UTC().Format(TIME_FILE_FORMAT)
	// FIXME: Fix path manipulation
	logFileName := fmt.Sprintf("%s/%s-%s.txt", config.LogFileDir, channel, today)
	logFile, ok := config.LogFiles[channel]
	var err error

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
	}

	message := fmt.Sprintf(fmt.Sprintf("<%s> %s\n", now, format), args...)
	_, err = logFile.WriteString(message)
	if err != nil {
		panic(fmt.Sprintf("Writing is failing %+v\n", err))
	}

}
