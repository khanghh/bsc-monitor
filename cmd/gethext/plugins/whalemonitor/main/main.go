package main

import (
	"path"

	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	pluginNamespace   = "WhaleMonitor"
	defaultConfigFile = "config.json"
)

var (
	log = plugin.NewLogger(pluginNamespace)
)

// handler serves as a base type for monitor processors.
// The monitor processor must inherit this struct and implement reexec.TransactionHook
type handler struct {
	ctx    *plugin.PluginCtx
	client *rpc.Client
	feed   event.Feed
}

func (p *handler) SubscribeWhaleEvent(ch chan whalemonitor.WhaleEvent) event.Subscription {
	return p.ctx.EventScope.Track(p.feed.Subscribe(ch))
}

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
	p.handler = &handler{ctx: ctx, client: client}

	p.bot, err = NewWhaleBot(&p.config, p.handler)
	if err != nil {
		return err
	}

	ctx.Monitor.AddProcessor((*TokenSwapMonitor)(p.handler))
	ctx.Monitor.AddProcessor((*TokenTransferMonitor)(p.handler))
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
