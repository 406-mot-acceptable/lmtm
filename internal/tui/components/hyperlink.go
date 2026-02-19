package components

import "fmt"

// Hyperlink renders an OSC8 clickable hyperlink for terminals that support it.
// Terminals that do not support OSC8 will display just the text.
// Format: \033]8;;URL\033\\TEXT\033]8;;\033\\
func Hyperlink(url, text string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// HTTPSLink generates a clickable https://localhost:PORT hyperlink.
func HTTPSLink(port int) string {
	url := fmt.Sprintf("https://localhost:%d", port)
	return Hyperlink(url, url)
}

// HTTPLink generates a clickable http://localhost:PORT hyperlink.
func HTTPLink(port int) string {
	url := fmt.Sprintf("http://localhost:%d", port)
	return Hyperlink(url, url)
}
