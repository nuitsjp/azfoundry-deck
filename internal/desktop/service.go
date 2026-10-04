// Package desktop owns application interaction, not product use cases.
package desktop

import "log/slog"

type Info struct {
	DiagnosticsAvailable bool   `json:"diagnosticsAvailable"`
	Name                 string `json:"name"`
	Version              string `json:"version"`
	AppID                string `json:"appID"`
	Server               bool   `json:"server"`
}
type Service struct {
	info   Info
	logger *slog.Logger
}

func New(info Info, logger *slog.Logger) *Service {
	return &Service{info: info, logger: logger}
}
func (s *Service) GetInfo() Info { return s.info }
func (s *Service) ReportFrontendError(message string) {
	if len(message) > 2000 {
		message = message[:2000]
	}
	// Callers send exception descriptions only, never input data or API payloads.
	s.logger.Error("frontend_unhandled", "message", message)
}
