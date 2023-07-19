package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/discordbot"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/event"
	"github.com/lus/dgc"
)

const (
	discordInstance = "DiscordBot"
)

var (
	msgConfigReloadFail = &discordgo.MessageSend{
		Content: "❌ Failed to reload config file.",
	}
	msgConfigReloadOK = &discordgo.MessageSend{
		Content: "✅ Config file reloaded successfully.",
	}
)

type WhaleBot struct {
	discordbot.DiscordBot
	*handler
	whaleCh chan whalemonitor.WhaleEvent
	sub     event.Subscription
}

func (bot *WhaleBot) Stop() {
	bot.sub.Unsubscribe()
	close(bot.whaleCh)
}

func (bot *WhaleBot) renderWhaleTokenTransferMessage(event *whalemonitor.WhaleEvent) *discordgo.MessageSend {
	title := "Whale Transfer Detected!"
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("**TxHash**\n [%s](%s/tx/%s)\n", event.TxHash, bot.config.ExplorerUrl, event.TxHash))
	desc.WriteString("\n**Transfers**\n")
	for idx, transfer := range event.Transfers {
		var tokenAmount string
		if transfer.Token != nil {
			amount := AmountString(transfer.Value, transfer.Token.Decimals)
			tokenAmount = fmt.Sprintf("%s [%s](%s/address/%s)", amount, transfer.Token.Symbol, bot.config.ExplorerUrl, transfer.Token.Address)
		} else {
			amount := AmountString(transfer.Value, 18)
			tokenAmount = fmt.Sprintf("%s %s", amount, bot.config.NativeToken)
		}
		desc.WriteString(fmt.Sprintf(
			"%d. [%s](%s) => [%s](%s): %s\n",
			idx+1,
			transfer.From, fmt.Sprintf("%s/address/%s", bot.config.ExplorerUrl, transfer.From),
			transfer.To, fmt.Sprintf("%s/address/%s", bot.config.ExplorerUrl, transfer.To),
			tokenAmount,
		))
	}
	return &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title:       title,
			Description: desc.String(),
			Type:        "rich",
			Color:       0x3498db,
			Timestamp:   time.Now().Format(time.RFC3339),
		},
	}
}

func (bot *WhaleBot) renderWhaleMessage(event *whalemonitor.WhaleEvent) *discordgo.MessageSend {
	if event.Type == whalemonitor.TypeTokenTransfer {
		return bot.renderWhaleTokenTransferMessage(event)
	}
	return nil
}

func (bot *WhaleBot) notifyLoop() {
	for {
		select {
		case <-bot.sub.Err():
			return
		case event := <-bot.whaleCh:
			msg := bot.renderWhaleMessage(&event)
			if msg != nil {
				bot.sendChannelMessage(msg)
			}
		}
	}
}

func (bot *WhaleBot) registerBotCommands() {
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

func (bot *WhaleBot) sendChannelMessage(msg *discordgo.MessageSend) error {
	if err := bot.SendChannelMessage(bot.config.ChannelId, msg); err != nil {
		msgJson, _ := json.Marshal(msg)
		log.Error("Could not send discord message", "msg", string(msgJson), "error", err)
		return err
	}
	return nil
}

func (bot *WhaleBot) handleReload(ctx *dgc.Ctx) {
	configFile := path.Join(bot.handler.DataDir, defaultConfigFile)
	msg := msgConfigReloadOK
	if err := loadConfig(configFile, bot.config); err != nil {
		log.Error("Failed to reload config file", "error", err)
		msg = msgConfigReloadFail
	}
	buf, _ := json.MarshalIndent(bot.config, "", " ")
	bot.sendChannelMessage(&discordgo.MessageSend{
		Content: fmt.Sprintf("```json\n%s\n```", string(buf)),
	})
	bot.sendChannelMessage(msg)
}

func (bot *WhaleBot) handleShowConfig(ctx *dgc.Ctx) {
	buf, _ := json.MarshalIndent(bot.config, "", " ")
	msg := &discordgo.MessageSend{
		Content: fmt.Sprintf("```json\n%s\n```", string(buf)),
	}
	bot.sendChannelMessage(msg)
}

func NewWhaleBot(handler *handler) (*WhaleBot, error) {
	bot := &WhaleBot{
		handler: handler,
		whaleCh: make(chan whalemonitor.WhaleEvent),
	}
	if instance, exist := handler.Get(discordInstance); exist {
		bot.DiscordBot = instance.(discordbot.DiscordBot)
	} else {
		return nil, errors.New("discord bot plugin not enabled")
	}
	bot.registerBotCommands()
	bot.sub = handler.SubscribeWhaleEvent(bot.whaleCh)
	go bot.notifyLoop()
	return bot, nil
}
