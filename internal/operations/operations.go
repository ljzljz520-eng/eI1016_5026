package operations

type AssetStatus struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Detail      string `json:"detail"`
	UpdatedAt   string `json:"updatedAt"`
	OpenTickets int    `json:"openTickets"`
}

type Repository interface {
	List() []AssetStatus
}

type MemoryRepository struct {
	items []AssetStatus
}

func NewMemoryRepository(items []AssetStatus) *MemoryRepository {
	copied := make([]AssetStatus, len(items))
	copy(copied, items)
	return &MemoryRepository{items: copied}
}

func (r *MemoryRepository) List() []AssetStatus {
	items := make([]AssetStatus, len(r.items))
	copy(items, r.items)
	return items
}
