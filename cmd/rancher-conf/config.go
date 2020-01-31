package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Interval        int        `toml:"interval"`
	MetadataVersion string     `toml:"metadata-version"`
	LogLevel        string     `toml:"log-level"`
	OneTime         bool       `toml:"onetime"`
	IncludeInactive bool       `toml:"include-inactive"`
	MetadataUrl     string     `toml:"metadata-url"`
	Templates       []Template `toml:"template"`
}

type Template struct {
	Source       string `toml:"source"`
	Dest         string `toml:"dest"`
	PollCmd      string `toml:"poll-cmd"`
	CheckCmd     string `toml:"check-cmd"`
	NotifyCmd    string `toml:"notify-cmd"`
	NotifyOutput bool   `toml:"notify-output"`
}

func initConfig(configFile string) (*Config, error) {
	config := Config{
		MetadataVersion: "latest",
		MetadataUrl:     "http://rancher-metadata.rancher.internal",
		Interval:        5,
		LogLevel:        "info",
	}

	if len(configFile) > 0 {
		log.Debugf("Loading config from file %s", configFile)
		err := setConfigFromFile(configFile, &config)
		if err != nil {
			return nil, fmt.Errorf("Could not load config file: %v", err)
		}
	} else {
		setTemplateFromFlags(&config)
	}

	overwriteConfigFromEnv(&config)
	overwriteConfigFromFlags(&config)

	if config.Interval == 0 {
		return nil, fmt.Errorf("Interval must be greater than 0")
	}

	lvl, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("Invalid log level: %s", config.LogLevel)
	}

	log.SetLevel(lvl)

	return &config, nil
}

func setConfigFromFile(path string, conf *Config) error {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = toml.Decode(string(buf), conf)
	if err != nil {
		return err
	}

	return nil
}

func setTemplateFromFlags(conf *Config) {
	tmpl := Template{
		Source:       flag.Arg(0),
		Dest:         flag.Arg(1),
		CheckCmd:     checkCmd,
		PollCmd: 			pollCmd,
		NotifyCmd:    notifyCmd,
		NotifyOutput: notifyOutput,
	}
	conf.Templates = []Template{tmpl}
}

func overwriteConfigFromFlags(conf *Config) {
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "interval":
			conf.Interval = interval
		case "metadata-url":
			conf.MetadataUrl = metadataUrl
		case "metadata-version":
			conf.MetadataVersion = metadataVersion
		case "onetime":
			conf.OneTime = onetime
		case "include-inactive":
			conf.IncludeInactive = includeInactive
		case "log-level":
			conf.LogLevel = logLevel
		}
	})
}

func overwriteConfigFromEnv(conf *Config) {
	var env string
	if env = os.Getenv("RANCHER_GEN_LOGLEVEL"); len(env) > 0 {
		conf.LogLevel = env
	}
	if env = os.Getenv("RANCHER_GEN_METADATA_URL"); len(env) > 0 {
		conf.MetadataUrl = env
	}
	if env = os.Getenv("RANCHER_GEN_INTERVAL"); len(env) > 0 {
		interval, err := strconv.Atoi(env)
		if err != nil {
			conf.Interval = interval
		} else {
			log.Warnf("Invalid value for environment variable 'RANCHER_GEN_INTERVAL': %s", env)
		}
	}
	if env = os.Getenv("RANCHER_GEN_METADATA_VER"); len(env) > 0 {
		conf.MetadataVersion = env
	}
	if env = os.Getenv("RANCHER_GEN_ONETIME"); len(env) > 0 {
		conf.OneTime = true
	}
	if env = os.Getenv("RANCHER_GEN_INACTIVE"); len(env) > 0 {
		conf.IncludeInactive = true
	}
}
