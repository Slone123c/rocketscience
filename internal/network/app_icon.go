package network

// App is what the Dock shows for the app an executable belongs to.
type App struct {
	Name string `json:"name"`
	// Icon is a PNG data URI.
	Icon string `json:"icon"`
}
