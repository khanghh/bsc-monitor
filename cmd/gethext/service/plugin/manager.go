//
// Created on 2022/12/23 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package plugin

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

const (
	pluginConfigFile = "config.toml"
	pluginOnLoadFunc = "OnLoad"
	pluginExtLinux   = ".so"
	pluginExtDarwin  = ".dylib"
)

var (
	errNotPlugin = errors.New("not a plugin")
	errNotFound  = errors.New("plugin not found")
	pluginExt    = pluginExtLinux
)

func init() {
	if runtime.GOOS == "darwin" {
		pluginExt = pluginExtDarwin
	}
}

type loadedPlugin struct {
	ctx      *PluginCtx
	name     string
	instance Plugin
	enabled  bool
}

type PluginManager struct {
	pluginDir   string
	configStore *configStore
	plugins     map[string]loadedPlugin
	ctx         *sharedCtx
	mtx         sync.Mutex
}

func (m *PluginManager) loadPlugin(fullpath string) (*loadedPlugin, error) {
	plib, err := plugin.Open(fullpath)
	if err != nil {
		return nil, err
	}
	ifunc, err := plib.Lookup(pluginOnLoadFunc)
	if err != nil {
		return nil, err
	}
	if plOnload, ok := ifunc.(func(*PluginCtx) Plugin); ok {
		plname := strings.ReplaceAll(filepath.Base(fullpath), pluginExt, "")
		plctx := &PluginCtx{
			sharedCtx: m.ctx,
			Log:       newLogger(plname),
			LoadConfig: func(cfg interface{}) error {
				return m.configStore.loadConfig(plname, cfg)
			},
			SaveConfig: func(cfg interface{}) error {
				return m.configStore.saveConfig(plname, cfg)
			},
		}
		plinstance := plOnload(plctx)
		loaded := loadedPlugin{
			ctx:      plctx,
			name:     plname,
			instance: plinstance,
			enabled:  false,
		}
		m.plugins[plname] = loaded
		return &loaded, m.EnablePlugin(plname)
	}
	return nil, errNotPlugin
}

func (m *PluginManager) LoadPlugins() error {
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		return err
	}
	files, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return err
	}
	loaded := []string{}
	for _, entry := range files {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), pluginExt) {
			fullpath := filepath.Join(m.pluginDir, entry.Name())
			pl, err := m.loadPlugin(fullpath)
			if err != nil {
				log.Error("Error occur when load plugin", "plugin", entry.Name(), "error", err)
				continue
			}
			loaded = append(loaded, pl.name)
		}
	}
	log.Info(fmt.Sprintf("Loaded %d plugin(s).", len(loaded)), "plugins", loaded)
	return nil
}

func (m *PluginManager) EnablePlugin(name string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	pl, isExist := m.plugins[name]
	if !isExist {
		return errNotFound
	}

	if err := pl.instance.OnEnable(pl.ctx); err != nil {
		return err
	}
	pl.enabled = true
	return nil
}

func (m *PluginManager) DisablePlugin(name string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	pl, isExist := m.plugins[name]
	if !isExist {
		return nil
	}

	if err := pl.instance.OnDisable(pl.ctx); err != nil {
		return err
	}
	pl.enabled = false
	return nil
}

func (m *PluginManager) Stop() error {
	for name := range m.plugins {
		if err := m.DisablePlugin(name); err != nil {
			log.Error("Error occur when trying to stop plugin", "plugin", name, "error", err)
		}
	}
	return nil
}

func NewPluginManager(pluginDir string, node *node.Node, ethBackend EthBackend, monitorBackend MonitorBackend, taskMgr TaskManager) (*PluginManager, error) {
	backend := &PluginManager{
		pluginDir:   pluginDir,
		configStore: NewConfigStore(path.Join(pluginDir, pluginConfigFile)),
		plugins:     make(map[string]loadedPlugin),
		ctx: &sharedCtx{
			Node:    node,
			Eth:     ethBackend,
			Monitor: monitorBackend,
			TaskMgr: taskMgr,
		},
	}
	return backend, nil
}