package service

import (
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/service/monitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/reexec"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

type MonitorServiceOptions struct {
	MonitorConfig *monitor.Config
	ReExecConfig  *reexec.Config
	PluginDir     string
	Node          *node.Node
	Ethereum      *eth.Ethereum
}

type MonitorService struct {
	chainMonitor  *monitor.ChainMonitor
	pluginManager *plugin.PluginManager
	taskManager   *reexec.TaskManager

	quitCh   chan struct{}
	quitLock sync.Mutex
}

func (s *MonitorService) Start() error {
	log.Info("Starting chain monitor service.")
	if err := s.chainMonitor.Start(); err != nil {
		log.Error("Could not start chain monitor", "error", err)
		return err
	}
	if err := s.pluginManager.LoadPlugins(); err != nil {
		log.Error("Could not load plugins", "error", err)
		return err
	}
	return nil
}

func (s *MonitorService) Stop() {
	s.quitLock.Lock()
	select {
	case <-s.quitCh:
	default:
		s.taskManager.Stop()
		s.pluginManager.Stop()
		s.chainMonitor.Stop()
		close(s.quitCh)
	}
	s.quitLock.Unlock()
	log.Info("Chain monitor service stopped.")
}

func NewMonitorService(opts *MonitorServiceOptions) (*MonitorService, error) {
	chainMonitor, err := monitor.NewChainMonitor(opts.MonitorConfig, opts.Ethereum)
	if err != nil {
		return nil, err
	}
	taskManager, err := reexec.NewTaskManager(opts.ReExecConfig)
	if err != nil {
		return nil, err
	}
	pluginManager, err := plugin.NewPluginManager(opts.PluginDir, opts.Node, opts.Ethereum.APIBackend, chainMonitor, taskManager)
	if err != nil {
		return nil, err
	}
	instance := &MonitorService{
		chainMonitor:  chainMonitor,
		pluginManager: pluginManager,
		taskManager:   taskManager,
		quitCh:        make(chan struct{}),
	}
	return instance, nil
}
