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
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

const (
	pluginConfigPrefix = "Plugins"
	pluginOnLoadFunc   = "OnLoad"
	pluginExtLinux     = ".so"
	pluginExtDarwin    = ".dylib"
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
	config      *PluginsConfig
	configStore *configStore
	plugins     map[string]*loadedPlugin
	ctx         *sharedCtx
	mtx         sync.Mutex
}

func (m *PluginManager) loadPlugin(filename string) (*loadedPlugin, error) {
	if filepath.Base(filename) != filename {
		return nil, errNotFound
	}
	if !strings.HasSuffix(filename, pluginExt) {
		filename = filename + pluginExt
	}
	fullpath := filepath.Join(m.config.PluginsDir, filename)
	if _, err := os.Stat(fullpath); err != nil {
		return nil, errNotFound
	}
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
			LoadConfig: func(cfg interface{}) error {
				return m.configStore.loadConfig(plname, cfg)
			},
		}
		plinstance := plOnload(plctx)
		m.plugins[plname] = &loadedPlugin{
			ctx:      plctx,
			name:     plname,
			instance: plinstance,
			enabled:  false,
		}
		return m.plugins[plname], nil
	}
	return nil, errNotPlugin
}

func (m *PluginManager) loadPlugins() error {
	if _, err := os.Stat(m.config.PluginsDir); os.IsNotExist(err) {
		return nil
	}
	files, err := os.ReadDir(m.config.PluginsDir)
	if err != nil {
		log.Error("Failed to read plugins directory", "error", err)
		return err
	}
	loaded := []string{}
	for _, entry := range files {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), pluginExt) {
			pl, err := m.loadPlugin(entry.Name())
			if err != nil {
				log.Error("Could not load plugin", "plugin", entry.Name(), "error", err)
				continue
			}
			loaded = append(loaded, pl.name)
		}
	}
	log.Info(fmt.Sprintf("Loaded %d plugin(s).", len(loaded)), "plugins", loaded)
	return nil
}

func (m *PluginManager) recoverPanic(plName string) {
	if err := recover(); err != nil {
		log.Error(fmt.Sprintf("Plugin %s crashed: %#v", plName, err))
	}
}

func (m *PluginManager) Status() ([]string, []string) {
	enabled := []string{}
	disabled := []string{}
	for name, pl := range m.plugins {
		if pl.enabled {
			enabled = append(enabled, name)
		} else {
			disabled = append(disabled, name)
		}
	}
	return enabled, disabled
}

func (m *PluginManager) EnablePlugin(name string) error {
	m.mtx.Lock()
	defer m.recoverPanic(name)
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
	defer m.recoverPanic(name)
	defer m.mtx.Unlock()
	pl, isExist := m.plugins[name]
	if !isExist {
		return nil
	}

	pl.ctx.EventScope.Close()
	if err := pl.instance.OnDisable(pl.ctx); err != nil {
		return err
	}
	pl.enabled = false
	return nil
}

func (m *PluginManager) Start() error {
	for _, name := range m.config.Enabled {
		if err := m.EnablePlugin(name); err != nil {
			log.Error(fmt.Sprintf("Could not enable plugin %s", name), "error", err)
		}
	}
	enabled, disabled := m.Status()
	if len(enabled) > 0 {
		log.Info(fmt.Sprintf("Enabled %d/%d plugin(s).", len(enabled), len(m.plugins)), "enabled", enabled, "disabled", disabled)
	}
	return nil
}

func (m *PluginManager) Stop() error {
	for name := range m.plugins {
		if err := m.DisablePlugin(name); err != nil {
			log.Error("Error occur when trying to stop plugin", "plugin", name, "error", err)
		}
	}
	log.Info("PluginManager stopped")
	return nil
}

func NewPluginManager(config *PluginsConfig, node *node.Node, ethBackend EthBackend, monitorBackend MonitorBackend, taskMgr TaskManager) (*PluginManager, error) {
	pm := &PluginManager{
		config:      config,
		configStore: NewConfigStore(pluginConfigPrefix, config.ConfigFile),
		plugins:     make(map[string]*loadedPlugin),
		ctx: &sharedCtx{
			Node:    node,
			Eth:     ethBackend,
			Monitor: monitorBackend,
			TaskMgr: taskMgr,
		},
	}
	if err := pm.loadPlugins(); err != nil {
		return nil, err
	}
	return pm, nil
}
