package mock

// MockService exposes mock-only controls to the screens. Reads
// remain on the common screen-facing services so mock and Azure paths use the
// same data contract.
type MockService struct {
	provider *Provider
}

func NewMockService(provider *Provider) *MockService {
	return &MockService{provider: provider}
}

// GetScenario keeps the controls in sync when the browser reloads.
func (s *MockService) GetScenario() string {
	s.provider.mu.Lock()
	defer s.provider.mu.Unlock()
	return s.provider.scenario
}

// SetScenario changes the response used by the next read.
func (s *MockService) SetScenario(name string) error {
	return s.provider.SetScenario(name)
}

// GetAddModelScenario keeps the model addition control in sync on reload.
func (s *MockService) GetAddModelScenario() string {
	s.provider.mu.Lock()
	defer s.provider.mu.Unlock()
	return s.provider.addScenario
}

// SetAddModelScenario changes the response used by the next addition read.
func (s *MockService) SetAddModelScenario(name string) error {
	return s.provider.SetAddModelScenario(name)
}

// GetCreateScenario keeps the creation-result control in sync on reload.
func (s *MockService) GetCreateScenario() string {
	s.provider.mu.Lock()
	defer s.provider.mu.Unlock()
	return s.provider.createScenario
}

// SetCreateScenario changes the creation result reproduced by the next add.
func (s *MockService) SetCreateScenario(name string) error {
	return s.provider.SetCreateScenario(name)
}
