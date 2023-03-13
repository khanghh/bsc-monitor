package main

import (
	"context"
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/ethereum/go-ethereum/log"
	"github.com/lus/dgc"
)

type CommandProcessor interface {
	RegisterCommands(cmdRouter *dgc.Router)
	OnStartBot(session *discordgo.Session) error
	OnStopBot()
}

type DiscordBot struct {
	Session       *discordgo.Session
	CmdRouter     *dgc.Router
	cmdProcessors []CommandProcessor
}

func (bot *DiscordBot) RegisterCommand(cmds ...*dgc.Command) {
	for _, cmd := range cmds {
		bot.CmdRouter.RegisterCmd(cmd)
	}
}

func (bot *DiscordBot) AddCommandProcessor(processor CommandProcessor) {
	bot.cmdProcessors = append(bot.cmdProcessors, processor)
}

func (bot *DiscordBot) SetCmdPrefix(cmdPrefix string) {
	bot.CmdRouter.Prefixes = []string{cmdPrefix}
}

func (bot *DiscordBot) Run(ctx context.Context) {
	// bot.CmdRouter.RegisterMiddleware(restrictRolesMiddleware)
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

func NewDiscordBot(botToken string, cmdPrefix string) (*DiscordBot, error) {
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
	return &DiscordBot{
		Session:   botSession,
		CmdRouter: cmdRouter,
	}, nil
}
