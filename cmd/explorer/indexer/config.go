package indexer

var DefaultConfig = Config{
	DatabaseHandles: 512,
	DatabaseCache:   1024,
}

type Config struct {
	DatabaseHandles int `toml:"-"`
	DatabaseCache   int `toml:",omitempty"`
}
