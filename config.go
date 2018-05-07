package main

import (
	"gopkg.in/ini.v1"
)

type Config struct {
	WebsocketURL string
	RPCURL       string

	DBHostname string
	DBProtocol string
	DBName     string
	DBUser     string
	DBPass     string
}

func LoadConfiguration(filepath string) (*Config, error) {
	cfg, err := ini.Load(filepath)
	if err != nil {
		return nil, err
	}

	config := new(Config)

	config.RPCURL = cfg.Section("network").Key("rpc_host").String()
	config.WebsocketURL = cfg.Section("network").Key("websocket_host").String()

	config.DBHostname = cfg.Section("db").Key("host").String()
	config.DBProtocol = cfg.Section("db").Key("protocol").String()
	config.DBName = cfg.Section("db").Key("name").String()
	config.DBUser = cfg.Section("db").Key("user").String()
	config.DBPass = cfg.Section("db").Key("pass").String()

	return config, nil
}
