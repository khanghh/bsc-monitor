package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

var nilAddress = common.Address{}

var defaultConfig = Config{
	ExplorerUrl: "https://bscscan.com",
	NativeToken: "BNB",
	Thresholds: map[common.Address]float64{
		common.Address{}: 500, // ETH
		common.HexToAddress("0xA3183498b579bd228aa2B62101C40CC1da978F24"): 50000,  // test token
		common.HexToAddress("0x55d398326f99059fF775485246999027B3197955"): 200000, // USDT
		common.HexToAddress("0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"): 200000, // BUSD
		common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"): 500,    // WBNB
		common.HexToAddress("0x7130d2A12B9BCbFAe4f2634d864A1Ee1Ce3Ead9c"): 7,      // BTCB
		common.HexToAddress("0x2170Ed0880ac9A755fd29B2688956BD959F933F8"): 100,    // WETH
		common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"): 200000, // USDC
		common.HexToAddress("0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82"): 10000,  // CAKE
	},
}

type Config struct {
	ExplorerUrl string
	ChannelId   string
	NativeToken string
	Thresholds  map[common.Address]float64
}

func (c *Config) UnmarshalJSON(data []byte) error {
	var cfg struct {
		ExplorerUrl string
		ChannelId   string
		NativeToken string
		Thresholds  map[string]float64
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.ExplorerUrl != "" {
		c.ExplorerUrl = cfg.ExplorerUrl
	}
	if cfg.ChannelId != "" {
		c.ChannelId = cfg.ChannelId
	}
	if cfg.NativeToken != "" {
		c.NativeToken = cfg.NativeToken
	}
	c.Thresholds = make(map[common.Address]float64)
	for tokenAddr, val := range cfg.Thresholds {
		if common.IsHexAddress(tokenAddr) {
			c.Thresholds[common.HexToAddress(tokenAddr)] = val
		} else {
			c.Thresholds[nilAddress] = val
		}
	}
	return nil
}

func loadConfig(filename string, cfg interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}
	return nil
}
