package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/lus/dgc"
)

type AdminCmdProcessor struct {
	AllowedRoles []string
}

func (p *AdminCmdProcessor) handlePing(ctx *dgc.Ctx) {
	ctx.RespondText("Pong!")
}

func (p *AdminCmdProcessor) RegisterCommands(cmdRouter *dgc.Router) {
	cmdRouter.RegisterCmd(&dgc.Command{
		Name:        "ping",
		Description: "Response with \"Pong!\"",
		Flags:       p.AllowedRoles,
		Usage:       "ping",
		Handler:     p.handlePing,
	})
}

func (p *AdminCmdProcessor) OnStartBot(session *discordgo.Session) error {
	return nil
}

func (p *AdminCmdProcessor) OnStopBot() {

}

func NewAdminCmdProcessor(allowedRoles []string) *AdminCmdProcessor {
	return &AdminCmdProcessor{allowedRoles}
}
