package main

import "gopkg.in/urfave/cli.v1"

var (
	pluginsDirFlag = cli.StringFlag{
		Name:  "plugins.dir",
		Usage: "Specify the directory where plugin files are located",
	}
	pluginsEnabledFlag = cli.StringSliceFlag{
		Name:  "plugins.enabled",
		Usage: "Comma separated list of plugin names to enable upon application startup",
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
