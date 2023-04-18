package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"gopkg.in/urfave/cli.v1"
)

var (
	extdbCommand = cli.Command{
		Name:      "extdb",
		Usage:     "Low level operations for extension database",
		ArgsUsage: "",
		Category:  "EXTDB COMMANDS",
		Flags: []cli.Flag{
			utils.DataDirFlag,
		},
		Subcommands: []cli.Command{
			extdbInspectCmd,
		},
	}
	extdbInspectCmd = cli.Command{
		Action:    utils.MigrateFlags(inspectExtDB),
		Name:      "inspect",
		ArgsUsage: "<prefix> <start>",
		Flags: []cli.Flag{
			utils.DataDirFlag,
		},
		Usage:       "Inspect the storage size for each type of data in the database",
		Description: `This commands iterates the entire database. If the optional 'prefix' and 'start' arguments are provided, then the iteration is limited to the given subset of data.`,
	}
)

func inspectExtDB(ctx *cli.Context) error {
	var (
		prefix []byte
		start  []byte
	)
	if ctx.NArg() > 2 {
		return fmt.Errorf("max 2 arguments: %v", ctx.Command.ArgsUsage)
	}
	if ctx.NArg() >= 1 {
		if d, err := hexutil.Decode(ctx.Args().Get(0)); err != nil {
			return fmt.Errorf("failed to hex-decode 'prefix': %v", err)
		} else {
			prefix = d
		}
	}
	if ctx.NArg() >= 2 {
		if d, err := hexutil.Decode(ctx.Args().Get(1)); err != nil {
			return fmt.Errorf("failed to hex-decode 'start': %v", err)
		} else {
			start = d
		}
	}
	stack := newNode(ctx, loadConfig(ctx))
	defer stack.Close()

	db, err := stack.OpenDatabase(extDatabaseName, extDatabaseCache, extDatabaseHandle, extNamespace, false)
	if err != nil {
		utils.Fatalf("Could not open database: %v", err)
	}
	defer db.Close()
	return extdb.InspectDatabase(db, prefix, start)
}
