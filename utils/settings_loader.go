// settings_loader.go
package utils

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

const DEFAULT_HOST = "localhost"
const DEFAULT_PORT = 3811

type HostData struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func saveDefaultHostSettings() {
	defaultHostData := HostData{DEFAULT_HOST, DEFAULT_PORT}
	jsonByteData, err := json.Marshal(defaultHostData)
	if IsError(err) {
		log.Println(err)
	}
	err = ioutil.WriteFile("settings.json", jsonByteData, 0644)
	if IsError(err) {
		log.Println(err)
	}
}

func GetHostDataFromSettingsFile() (string, int) {
	// returns (host, port)
	f, err := ioutil.ReadFile("settings.json")
	if IsError(err) {
		saveDefaultHostSettings()
		return DEFAULT_HOST, DEFAULT_PORT
	}

	hostData := HostData{}
	err = json.Unmarshal([]byte(f), &hostData)
	if IsError(err) {
		saveDefaultHostSettings()
		return DEFAULT_HOST, DEFAULT_PORT
	}

	return hostData.Host, hostData.Port
}
