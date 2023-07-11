//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

var (
	DefaultConfig = Config{
		Enabled: false,
	}
)

type Config struct {
	Enabled bool
}

func (cfg *Config) Sanitize() error {
	return nil
}
