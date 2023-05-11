package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/discordbot"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/event"
)

const (
	explorerURL = "https://bscscan.com"
)

type discordSender struct {
	bot discordbot.DiscordBot
}

func renderMessageEmbed(event *whalemonitor.WhaleEvent) *discordgo.MessageSend {
	title := ""
	amount := ""
	tokenUrl := ""
	tokenName := ""
	if event.Token != nil {
		title = "Big ERC20 transfer"
		amount = AmountString(event.Value, event.Token.Decimals)
		tokenUrl = fmt.Sprintf("%s/token/%s", explorerURL, event.Token.Address)
		tokenName = event.Token.Name
	} else {
		title = "Big ETH transfer"
		amount = AmountString(event.Value, 18)
		tokenName = "ETH"
	}
	fields := []*discordgo.MessageEmbedField{
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
			Value: fmt.Sprintf("%s [**%s**](${%s})", amount, tokenName, tokenUrl),
		},
	}
	return &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title:     title,
			Type:      "rich",
			URL:       fmt.Sprintf("%s/tx/%s", explorerURL, event.TxHash),
			Color:     0x3498db,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
}

func (s *discordSender) Send(event *whalemonitor.WhaleEvent) {
	msg := renderMessageEmbed(event)
	if err := s.bot.SendChannelMessage(msg); err != nil {
		logger.Error("Could not send discord message", "error", err)
	}
}

func newDiscordSender(bot discordbot.DiscordBot) *discordSender {
	return &discordSender{bot}
}

func initNotificationSenders(ctx *plugin.PluginCtx) []notificationSender {
	senders := []notificationSender{}
	if instance, exist := ctx.Get(discordbot.PluginNamespace); exist {
		bot := instance.(discordbot.DiscordBot)
		senders = append(senders, newDiscordSender(bot))
	}
	return senders
}

type notificationSender interface {
	Send(event *whalemonitor.WhaleEvent)
}

func notifyEventLoop(senders []notificationSender, eventCh chan whalemonitor.WhaleEvent, subs event.Subscription) {
	sendEvent := func(event *whalemonitor.WhaleEvent) {
		for _, sender := range senders {
			sender.Send(event)
		}
	}
	for {
		select {
		case <-subs.Err():
		case event := <-eventCh:
			sendEvent(&event)
		}
	}
}
