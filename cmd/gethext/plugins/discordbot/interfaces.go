package discordbot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/lus/dgc"
)

type DiscordBot interface {
	RegisterCommand(cmd ...dgc.Command)
	UnregisterCommand(name string)
	SendChannelMessage(channelID string, message *discordgo.MessageSend) error
}
