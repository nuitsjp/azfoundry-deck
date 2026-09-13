package mock

// MockService exposes mock-only controls to the F1 screen. The deployment
// read itself remains on service.DeploymentService so mock and Azure paths use
// the same screen-facing contract.
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
