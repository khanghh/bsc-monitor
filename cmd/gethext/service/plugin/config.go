//
// Created on 2022/12/23 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package plugin

import (
	"fmt"
	"os"
	"reflect"
	"sync"

	"github.com/naoina/toml"
)

var (
	configFile    string
	config        map[string]interface{}
	startupLoader sync.Once
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		return fmt.Errorf("field '%s' is not defined in %s", field, rt.String())
	},
}

func ConfigFile() string {
	return configFile
}

func LoadConfigFile(filename string) error {
	var err error
	startupLoader.Do(func() {
		err = LoadTOMLConfig(filename, &config)
		if err == nil {
			configFile = filename
		}
	})
	return err
}

func LoadTOMLConfig(filename string, conf interface{}) error {
	var err error
	var buf []byte
	if buf, err = os.ReadFile(filename); err == nil {
		err = tomlSettings.Unmarshal(buf, conf)
	}
	return err
}

func GetConfig(tag string, conf interface{}) error {
	var (
		rawConf interface{}
		exists  bool
	)
	if tag == "" {
		rawConf = config
	} else if rawConf, exists = config[tag]; !exists {
		return nil
	}
	buf, err := tomlSettings.Marshal(rawConf)
	if err != nil {
		return err
	}
	return tomlSettings.Unmarshal(buf, conf)
}
