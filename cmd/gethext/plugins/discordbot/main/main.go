package main

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/log"
)

const (
	pluginName = "DiscordBot"
)

type DiscordConfig struct {
	BotToken     string   `json:"botToken"`
	CmdPrefix    string   `json:"cmdPrefix"`
	AllowedRoles []string `json:"allowedRoles"`
}

func (cfg *DiscordConfig) sanitize() error {
	if cfg.CmdPrefix == "" {
		cfg.CmdPrefix = "!"
	}
	if cfg.BotToken == "" {
		return errors.New("bot token not provided")
	}
	return nil
}

type DiscordPlugin struct {
	bot  *discordBot
	quit context.CancelFunc
}

func (p *DiscordPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	config := new(DiscordConfig)
	if err := ctx.LoadConfig(pluginName, &config); err != nil {
		return err
	}
	if err := config.sanitize(); err != nil {
		return err
	}
	var err error
	p.bot, err = NewDiscordBot(config.BotToken, config.CmdPrefix)
	if err != nil {
		log.Error("Could not initialize discord bot", "error", err)
		return err
	}
	p.bot.RegisterCommand(NewAdminCmdProcessor(config.AllowedRoles).Commands()...)
	botCtx, cancel := context.WithCancel(context.Background())
	p.quit = cancel
	go p.bot.Run(botCtx)
	ctx.Set(pluginName, p.bot)
	return nil
}

func (p *DiscordPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	if p.quit != nil {
		p.quit()
	}
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	pl := &DiscordPlugin{}
	return pl
}
