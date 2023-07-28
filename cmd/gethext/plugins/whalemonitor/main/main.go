package main

import (
	"path"

	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
)

const (
	pluginNamespace   = "WhaleMonitor"
	defaultConfigFile = "config.json"
)

var (
	log = plugin.NewLogger(pluginNamespace)
)

type WhaleMonitorPlugin struct {
	*handler
	bot *WhaleBot
}

func (p *WhaleMonitorPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	config := defaultConfig
	if err := loadConfig(path.Join(ctx.DataDir, defaultConfigFile), &config); err != nil {
		return err
	}

	client, err := ctx.Node.Attach()
	if err != nil {
		return err
	}
	p.handler = &handler{
		PluginCtx: ctx,
		client:    client,
		config:    &config,
	}

	p.bot, err = NewWhaleBot(p.handler)
	if err != nil {
		return err
	}

	ctx.Monitor.AddProcessor(NewTokenTransferMonitor(p.handler))
	return nil
}

func (p *WhaleMonitorPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	p.bot.Stop()
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &WhaleMonitorPlugin{}
}
