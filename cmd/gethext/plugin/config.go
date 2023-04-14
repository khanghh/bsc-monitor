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

func saveTOMLConfig(filename string, conf interface{}) error {
	buf, err := tomlSettings.Marshal(conf)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, buf, 0644)
}

type configStore struct {
	fileName string
	fileInfo os.FileInfo
	payload  map[string]interface{}
}

func (c *configStore) getConfig(name string, cfg interface{}) error {
	var (
		rawConf interface{}
		exists  bool
	)
	if rawConf, exists = c.payload[name]; !exists {
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
		if err := loadTOMLConfig(c.fileName, &c.payload); err != nil {
			return err
		}
	}
	c.fileInfo = fileInfo
	return c.getConfig(name, cfg)
}

func (c *configStore) saveConfig(name string, cfg interface{}) error {
	if cfg == nil {
		delete(c.payload, name)
	} else {
		c.payload[name] = cfg
	}
	return saveTOMLConfig(c.fileName, c.payload)
}

func NewConfigStore(fileName string) *configStore {
	cfg := &configStore{
		fileName: fileName,
		payload:  make(map[string]interface{}),
	}
	return cfg
}
