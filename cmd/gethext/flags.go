package main

import "gopkg.in/urfave/cli.v1"

var (
	pluginsDirFlag = cli.StringFlag{
		Name:  "pluginsdir",
		Usage: "Plugins directory",
	}
	monitorEnableFlag = cli.BoolFlag{
		Name:  "monitor.enabled",
		Usage: "Enable chain monitor",
	}
	indexerEnableFlag = cli.BoolFlag{
		Name:  "indexer.enabled",
		Usage: "Enable chain indexer",
	}
)
