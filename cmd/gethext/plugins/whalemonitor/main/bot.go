package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/discordbot"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/event"
)

const (
	discordInstance = "DiscordBot"
)

type WhaleBot struct {
	discordbot.DiscordBot
	config  *Config
	handler *handler
	whaleCh chan whalemonitor.WhaleEvent
	sub     event.Subscription
}

func (bot *WhaleBot) Stop() {
	bot.sub.Unsubscribe()
	close(bot.whaleCh)
}

func (bot *WhaleBot) renderWhaleMessage(event *whalemonitor.WhaleEvent) *discordgo.MessageSend {
	title := ""
	tokenText := ""
	if event.Token != nil {
		title = "Big ERC20 transfer"
		amount := AmountString(event.Value, event.Token.Decimals)
		tokenUrl := fmt.Sprintf("%s/token/%s", bot.config.ExplorerUrl, event.Token.Address)
		tokenText = fmt.Sprintf("%s [%s](%s)", amount, event.Token.Name, tokenUrl)
	} else {
		title = "Big BNB transfer"
		amount := AmountString(event.Value, 18)
		tokenText = fmt.Sprintf("%s BNB", amount)
	}
	fields := []*discordgo.MessageEmbedField{
		{
			Name:  "Tx",
			Value: event.TxHash.String(),
		},
		{
			Name:  "From",
			Value: event.From.String(),
		},
		{
			Name:  "To",
			Value: event.To.String(),
		},
		{
			Name:  "Amount",
			Value: tokenText,
		},
	}
	return &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title:     title,
			Type:      "rich",
			URL:       fmt.Sprintf("%s/tx/%s", bot.config.ExplorerUrl, event.TxHash),
			Color:     0x3498db,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
}

func (bot *WhaleBot) notifyLoop() {
	for {
		select {
		case <-bot.sub.Err():
			return
		case event := <-bot.whaleCh:
			msg := bot.renderWhaleMessage(&event)
			if err := bot.SendChannelMessage(bot.config.ChannelId, msg); err != nil {
				log.Error("Could not send discord messaqge", "error", err)
			}
		}
	}
}

func NewWhaleBot(config *Config, handler *handler) (*WhaleBot, error) {
	bot := &WhaleBot{
		config:  config,
		handler: handler,
		whaleCh: make(chan whalemonitor.WhaleEvent),
	}
	if instance, exist := handler.ctx.Get(discordInstance); exist {
		bot.DiscordBot = instance.(discordbot.DiscordBot)
	} else {
		return nil, errors.New("discord bot plugin not enabled")
	}
	bot.addDiscordCommands()
	bot.sub = handler.SubscribeWhaleEvent(bot.whaleCh)
	go bot.notifyLoop()
	return bot, nil
}
