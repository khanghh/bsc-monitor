package main

import (
	"github.com/lus/dgc"
)

func isRoleAllowed(requireRoles []string, userRoles []string) bool {
	if len(requireRoles) == 0 {
		return true
	}
	for _, roleId := range userRoles {
		for _, requiredRoleId := range requireRoles {
			if roleId == requiredRoleId {
				return true
			}
		}
	}
	return false
}

func restrictRolesMiddleware(next dgc.ExecutionHandler) dgc.ExecutionHandler {
	return func(ctx *dgc.Ctx) {
		if isRoleAllowed(ctx.Command.Flags, ctx.Event.Member.Roles) {
			next(ctx)
			return
		}
		ctx.RespondText("You do not have permission to perform this action.")
	}
}
