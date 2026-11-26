package app

type autostartManager interface {
	Enabled() (bool, error)
	Set(bool) error
}

type AutostartService struct{ manager autostartManager }

func NewAutostartService(manager autostartManager) *AutostartService {
	return &AutostartService{manager: manager}
}

func (s *AutostartService) GetAutostartEnabled() (bool, error) {
	return s.manager.Enabled()
}

func (s *AutostartService) SetAutostartEnabled(enabled bool) (bool, error) {
	if err := s.manager.Set(enabled); err != nil {
		return false, err
	}
	return s.manager.Enabled()
}
