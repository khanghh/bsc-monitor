package main

import (
	"context"
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/discordbot"
	"github.com/ethereum/go-ethereum/log"
	"github.com/lus/dgc"
)

type discordBot struct {
	Session       *discordgo.Session
	CmdRouter     *dgc.Router
	cmdProcessors []discordbot.CommandProcessor
	ChannelId     string
}

func (bot *discordBot) RegisterCommand(cmds ...*dgc.Command) {
	for _, cmd := range cmds {
		bot.CmdRouter.RegisterCmd(cmd)
	}
}

func (bot *discordBot) AddCommandProcessor(processor discordbot.CommandProcessor) {
	bot.cmdProcessors = append(bot.cmdProcessors, processor)
}

func (bot *discordBot) SetCmdPrefix(cmdPrefix string) {
	bot.CmdRouter.Prefixes = []string{cmdPrefix}
}

func (bot *discordBot) SendChannelMessage(messgae *discordgo.MessageSend) error {
	_, err := bot.Session.ChannelMessageSendComplex(bot.ChannelId, messgae)
	return err
}

func (bot *discordBot) Run(ctx context.Context) {
	for _, processor := range bot.cmdProcessors {
		processor.RegisterCommands(bot.CmdRouter)
	}
	bot.CmdRouter.RegisterDefaultHelpCommand(bot.Session, nil)
	bot.CmdRouter.Initialize(bot.Session)

	for _, processor := range bot.cmdProcessors {
		err := processor.OnStartBot(bot.Session)
		if err != nil {
			log.Error("Could not initialize plugin", "processor", reflect.TypeOf(processor), "error", err)
		}
	}
	<-ctx.Done()
	for _, processor := range bot.cmdProcessors {
		processor.OnStopBot()
	}
}

func NewDiscordBot(botToken string, cmdPrefix string, channelId string) (*discordBot, error) {
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
		ChannelId: channelId,
	}, nil
}
