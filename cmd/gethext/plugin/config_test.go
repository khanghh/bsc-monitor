package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigStore(t *testing.T) {
	configStore := NewConfigStore("Plugins", "config.test.toml")
	type DemoConfig struct {
		Field1 bool
		Field2 string
	}
	cfg := DemoConfig{}
	if err := configStore.LoadConfig("Demo", &cfg); err != nil {
		panic(err)
	}
	assert.Equal(t, true, cfg.Field1)
	assert.Equal(t, "test", cfg.Field2)
}
