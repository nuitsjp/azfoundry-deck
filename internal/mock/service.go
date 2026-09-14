package mock

// MockService exposes mock-only controls to the F1 and F2 screens. Reads
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

// GetModelScenario keeps the F2 control in sync when the browser reloads.
func (s *MockService) GetModelScenario() string {
	s.provider.mu.Lock()
	defer s.provider.mu.Unlock()
	return s.provider.modelScenario
}

// SetModelScenario changes the response used by the next F2 read.
func (s *MockService) SetModelScenario(name string) error {
	return s.provider.SetModelScenario(name)
}
