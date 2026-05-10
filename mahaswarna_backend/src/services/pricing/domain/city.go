package domain

// City represents an Indian city for which rates are tracked.
type City struct {
	ID       string
	Name     string
	State    string
	IsActive bool
}
