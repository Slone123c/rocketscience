//go:build darwin

package network

// AppFor returns the app around executable, or ok=false for command-line tools.
// NSWorkspace icon lookups are safe off the main thread.
func AppFor(executable string) (App, bool) {
	if executable == "" || load() != nil {
		return App{}, false
	}
	pointer := appIconFn(executable)
	if pointer == nil {
		return App{}, false
	}
	var app App
	if err := decode(pointer, 1<<20, &app); err != nil {
		return App{}, false
	}
	return app, true
}
