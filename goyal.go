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
	TIME_FORMAT   = "Jan 2 2006 15:04:05"
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

	setupLogFiles(&config)
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
		logMessage(config.LogFiles[channel], "%s entered %s", e.Nick, channel)
	})

	connection.AddCallback("PRIVMSG", func(e *irc.Event) {
		channel := e.Arguments[0]
		switch channel {
		case config.Nick:
			connection.Privmsg(e.Nick, "Sorry, I don't accept direct messages!")
		default:
			logMessage(config.LogFiles[channel], "%s: %s", e.Nick, e.Message())
		}
	})

	connection.AddCallback("CTCP_ACTION", func(e *irc.Event) {
		channel := e.Arguments[0]
		logMessage(config.LogFiles[channel], "***%s %s", e.Nick, e.Message())
	})

	connection.AddCallback("PART", func(e *irc.Event) {
		channel := e.Arguments[0]
		logMessage(config.LogFiles[channel], "%s left %s", e.Nick, channel)
	})

	connection.AddCallback("QUIT", func(e *irc.Event) {
		channel := e.Arguments[0]
		logMessage(config.LogFiles[channel], "%s closed IRC", e.Nick)
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

	return config, nil
}

func setupLogFiles(config *IRCConfig) {
	config.LogFiles = make(map[string]*os.File)
	for _, channel := range config.Channels {
		/// FIXME: fix path manipulation
		logFileName := config.LogFileDir + "/" + channel
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Println(err)
			fmt.Println("Log file opening failed.")
		}
		config.LogFiles[channel] = logFile
	}
}

func closeLogFiles(config IRCConfig) {
	for _, logFile := range config.LogFiles {
		logFile.Close()
	}
}

func logMessage(logFile *os.File, format string, args ...interface{}) {
	if logFile == nil {
		panic(fmt.Sprintf("No such log file provided."))
	}
	now := time.Now().UTC().Format(TIME_FORMAT)
	message := fmt.Sprintf(fmt.Sprintf("<%s> %s\n", now, format), args...)
	_, err := logFile.WriteString(message)
	if err != nil {
		panic(fmt.Sprintf("Writing is failing %+v\n", err))
	}
}
