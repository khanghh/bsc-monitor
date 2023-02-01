package monitor

var (
	DefaultConfig = Config{}
)

type Config struct {
	ProcessQueue int
	ProcessSlot  int
}

func (cfg *Config) Sanitize() error {
	return nil
}
