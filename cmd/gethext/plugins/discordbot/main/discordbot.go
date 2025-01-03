package main

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/lus/dgc"
)

type discordBot struct {
	Session   *discordgo.Session
	CmdRouter *dgc.Router
	Commands  map[string]dgc.Command
}

func (bot *discordBot) rebuildRouter() {
	commands := make([]*dgc.Command, 0, len(bot.Commands))
	for _, cmd := range bot.Commands {
		copyCmd := cmd
		commands = append(commands, &copyCmd)
	}
	bot.CmdRouter.Commands = commands
}

func (bot *discordBot) UnregisterCommand(name string) {
	if _, ok := bot.Commands[name]; ok {
		delete(bot.Commands, name)
		bot.rebuildRouter()
	}
}

func (bot *discordBot) RegisterCommand(cmds ...dgc.Command) {
	if len(cmds) > 0 {
		for _, cmd := range cmds {
			bot.Commands[cmd.Name] = cmd
		}
		bot.rebuildRouter()
	}
}

func (bot *discordBot) SetCmdPrefix(cmdPrefix string) {
	bot.CmdRouter.Prefixes = []string{cmdPrefix}
}

func (bot *discordBot) SendChannelMessage(channelID string, messgae *discordgo.MessageSend) error {
	_, err := bot.Session.ChannelMessageSendComplex(channelID, messgae)
	return err
}

func (bot *discordBot) Run(ctx context.Context) {
	bot.CmdRouter.RegisterDefaultHelpCommand(bot.Session, nil)
	bot.CmdRouter.Initialize(bot.Session)
	<-ctx.Done()
}

func NewDiscordBot(botToken string, cmdPrefix string) (*discordBot, error) {
	botSession, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, err
	}

	if err = botSession.Open(); err != nil {
		return nil, err
	}

	cmdRouter := &dgc.Router{
		Prefixes: []string{cmdPrefix},
		Storage:  make(map[string]*dgc.ObjectsMap),
	}
	return &discordBot{
		Session:   botSession,
		CmdRouter: cmdRouter,
		Commands:  make(map[string]dgc.Command),
	}, nil
}
