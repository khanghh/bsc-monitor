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
	config Config
	bot    *WhaleBot
}

func (p *WhaleMonitorPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	if err := loadConfig(path.Join(ctx.DataDir, defaultConfigFile), &p.config); err != nil {
		return err
	}

	client, err := ctx.Node.Attach()
	if err != nil {
		return err
	}
	p.handler = &handler{
		PluginCtx: ctx,
		client:    client,
	}

	p.bot, err = NewWhaleBot(&p.config, p.handler)
	if err != nil {
		return err
	}

	ctx.Monitor.AddProcessor(NewTokenTransferMonitor(p.handler, p.config.Thresholds))
	return nil
}

func (p *WhaleMonitorPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	p.bot.Stop()
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &WhaleMonitorPlugin{
		config: defaultConfig,
	}
}
