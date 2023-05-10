package discordbot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/lus/dgc"
)

type CommandProcessor interface {
	RegisterCommands(cmdRouter *dgc.Router)
	OnStartBot(session *discordgo.Session) error
	OnStopBot()
}

type DiscordBot interface {
	RegisterCommand(cmd ...*dgc.Command)
	AddCommandProcessor(processor CommandProcessor)
}
