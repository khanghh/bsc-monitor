package main

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/bwmarrin/discordgo"
	"github.com/lus/dgc"
)

var (
	msgConfigReloadFail = &discordgo.MessageSend{
		Content: "❌ Failed to reload config file.",
	}
	msgConfigReloadOK = &discordgo.MessageSend{
		Content: "✅ Config file reloaded successfully.",
	}
)

func (bot *WhaleBot) addDiscordCommands() {
	fmt.Println("addDiscordCommands")
	bot.RegisterCommand(
		dgc.Command{
			Name:        "reload",
			Description: "Reload and apply plugin config file",
			Handler:     bot.handleReload,
		},
		dgc.Command{
			Name:        "config",
			Description: "Show curernt plugin config",
			Handler:     bot.handleShowConfig,
		},
	)
}

func (bot *WhaleBot) handleReload(ctx *dgc.Ctx) {
	fmt.Println("handleReload")
	configFile := path.Join(bot.handler.ctx.DataDir, defaultConfigFile)
	msg := msgConfigReloadOK
	if err := loadConfig(configFile, bot.config); err != nil {
		log.Error("Failed to reload config file", "error", err)
		msg = msgConfigReloadFail
	}
	if err := bot.SendChannelMessage(bot.config.ChannelId, msg); err != nil {
		log.Error("Could not send discord message", "error", err)
	}
}

func (bot *WhaleBot) handleShowConfig(ctx *dgc.Ctx) {
	fmt.Println("handleShowConfig")
	buf, _ := json.MarshalIndent(bot.config, "", " ")
	msg := &discordgo.MessageSend{
		Content: fmt.Sprintf("```json\n%s\n```", string(buf)),
	}
	if err := bot.SendChannelMessage(bot.config.ChannelId, msg); err != nil {
		log.Error("Could not send discord message", "error", err)
	}
}
