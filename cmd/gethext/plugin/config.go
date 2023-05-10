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

	"github.com/naoina/toml"
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

func loadTOMLConfig(filename string, conf interface{}) error {
	var err error
	var buf []byte
	if buf, err = os.ReadFile(filename); err == nil {
		err = tomlSettings.Unmarshal(buf, conf)
	}
	return err
}

type PluginsConfig struct {
	ConfigFile string
	PluginsDir string
	Enabled    []string
}

type configStore struct {
	prefix     string
	fileName   string
	fileInfo   os.FileInfo
	configData map[string]interface{}
}

func (c *configStore) getConfig(name string, cfg interface{}) error {
	var (
		rawConf interface{}
		exists  bool
	)
	if rawConf, exists = c.configData[name]; !exists {
		return nil
	}
	buf, err := tomlSettings.Marshal(rawConf)
	if err != nil {
		return err
	}
	return tomlSettings.Unmarshal(buf, cfg)
}

func (c *configStore) loadConfig(name string, cfg interface{}) error {
	fileInfo, err := os.Stat(c.fileName)
	if err != nil {
		return err
	}
	if c.fileInfo == nil || fileInfo.ModTime().After(c.fileInfo.ModTime()) {
		tomlConfig := make(map[string]interface{})
		if err := loadTOMLConfig(c.fileName, &tomlConfig); err != nil {
			return err
		}
		c.configData = tomlConfig[c.prefix].(map[string]interface{})
		c.fileInfo = fileInfo
	}
	return c.getConfig(name, cfg)
}

func NewConfigStore(prefix, fileName string) *configStore {
	cfg := &configStore{
		prefix:     prefix,
		fileName:   fileName,
		configData: make(map[string]interface{}),
	}
	return cfg
}
