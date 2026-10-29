package core

// publishStartup is called with mu held. Readers use an immutable snapshot so
// configuration downloads or a macOS authorization prompt cannot block status.
// Lifecycle writes remain serialized by opMu and the existing cancellation path.
func (s *Service) publishStartup(stage string, port int) {
	s.startupStatus.Store(&Status{
		Starting: true, StartupStage: stage, MixedPort: port,
		CorePath: corePath(), SystemProxy: s.useSystemProxy, Tun: s.useTun,
	})
}
