package main

import (
	"encoding/json"
	"fmt"
	"github.com/thoj/go-ircevent"
	"io"
	"io/ioutil"
	"strings"
)

type IRCConfig struct {
	Nick     string
	Username string
	Server   string
	Channels []string
}

func addCallbacks(c *irc.Connection) {
	// fmt.Println(c)
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

func main() {
	config, err := getConfig()
	if err == nil {
		connection := irc.IRC(config.Nick, config.Username)
		connection.UseTLS = true

		addCallbacks(connection)

		//Connect to the server
		err := connection.Connect(config.Server)
		if err == nil {

			// Join the channels
			for _, channel := range config.Channels {
				fmt.Printf("Joining %s\n", channel)
				connection.Join(channel)
			}

		} else {
			fmt.Println(err)
		}

	} else {
		fmt.Println(err)
	}

}
