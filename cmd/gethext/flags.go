package main

import "gopkg.in/urfave/cli.v1"

var (
	pluginDirFlag = cli.StringFlag{
		Name:  "plugindir",
		Usage: "Plugin directory",
	}
	noIndexFlag = cli.BoolFlag{
		Name:  "noindex",
		Usage: "Run geth without node indexing block chain",
	}
)
