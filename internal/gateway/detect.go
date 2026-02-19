package gateway

import (
	"context"
	"strings"
)

// Detect determines the gateway type and returns the appropriate Gateway
// implementation. It takes the SSH server banner and a command runner.
//
// Detection strategy:
//  1. Check SSH banner for "ROSSSH" or "MikroTik" -> MikroTik
//  2. Try `/system identity print` -- if it succeeds -> MikroTik
//  3. Try `cat /etc/version` or `uname -a` -- if contains "EdgeOS" or "ubnt" -> Ubiquiti
//  4. Default to Ubiquiti (Linux-based commands are more portable)
func Detect(ctx context.Context, banner string, run CommandRunner) (Gateway, error) {
	// Step 1: banner-based detection.
	upper := strings.ToUpper(banner)
	if strings.Contains(upper, "ROSSSH") || strings.Contains(upper, "MIKROTIK") {
		return newMikroTik(run), nil
	}

	// Step 2: command probe -- MikroTik identity.
	if out, err := run(ctx, "/system identity print"); err == nil {
		out = strings.TrimSpace(out)
		if out != "" && !strings.Contains(out, "not found") && !strings.Contains(out, "No such file") {
			return newMikroTik(run), nil
		}
	}

	// Step 3: command probe -- Ubiquiti / EdgeOS.
	if out, err := run(ctx, "cat /etc/version"); err == nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "edgeos") || strings.Contains(lower, "ubnt") || strings.Contains(lower, "ubiquiti") {
			return newUbiquiti(run), nil
		}
	}

	if out, err := run(ctx, "uname -a"); err == nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "edgeos") || strings.Contains(lower, "ubnt") || strings.Contains(lower, "ubiquiti") {
			return newUbiquiti(run), nil
		}
	}

	// Step 4: default to Ubiquiti -- Linux-based commands are more portable.
	return newUbiquiti(run), nil
}
