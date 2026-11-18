package app

type fontInventory interface {
	List() ([]string, error)
}

type FontService struct{ inventory fontInventory }

func NewFontService(inventory fontInventory) *FontService {
	return &FontService{inventory: inventory}
}

func (s *FontService) ListSystemFonts() ([]string, error) {
	return s.inventory.List()
}
