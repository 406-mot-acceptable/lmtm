package ssh

import "os"

// newStderrWriter returns os.Stderr as an io.Writer.
// Extracted to a function so it can be replaced in tests.
func newStderrWriter() *os.File {
	return os.Stderr
}
