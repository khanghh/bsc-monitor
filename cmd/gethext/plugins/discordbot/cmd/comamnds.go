package main

import (
	"github.com/lus/dgc"
)

type AdminCmdProcessor struct {
	AllowedRoles []string
}

func (p *AdminCmdProcessor) handlePing(ctx *dgc.Ctx) {
	ctx.RespondText("Pong!")
}

func (p *AdminCmdProcessor) Commands() []dgc.Command {
	return []dgc.Command{
		{
			Name:        "ping",
			Description: "Response with \"Pong!\"",
			Flags:       p.AllowedRoles,
			Usage:       "ping",
			Handler:     p.handlePing,
		},
	}
}

func NewAdminCmdProcessor(allowedRoles []string) *AdminCmdProcessor {
	return &AdminCmdProcessor{allowedRoles}
}
