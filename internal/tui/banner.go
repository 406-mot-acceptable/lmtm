package tui

import (
	"math/rand"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// taglines is a rotating set of quips shown under the LMTM banner.
var taglines = []string{
	"Because VPNs are for quitters.",
	"All ports lead to localhost.",
	"Tunnel vision, but on purpose.",
	"No packets were harmed. Probably.",
	"SSH into things. Professionally.",
	"Port forwarding with unnecessary flair.",
	"Your packets. Your rules.",
	"Because netcat deserves a day off.",
	"Now with 100% more localhost.",
}

// sessionTagline is selected once at startup and stays for the session.
var sessionTagline string

func init() {
	src := rand.NewSource(time.Now().UnixNano())
	r := rand.New(src)
	sessionTagline = taglines[r.Intn(len(taglines))]
}

// Banner returns the large ASCII art LMTM banner with a random tagline.
// LMTM = Lean Mean Tunneling Machine.
func Banner() string {
	art := `
 ██╗     ███╗   ███╗████████╗███╗   ███╗
 ██║     ████╗ ████║╚══██╔══╝████╗ ████║
 ██║     ██╔████╔██║   ██║   ██╔████╔██║
 ██║     ██║╚██╔╝██║   ██║   ██║╚██╔╝██║
 ███████╗██║ ╚═╝ ██║   ██║   ██║ ╚═╝ ██║
 ╚══════╝╚═╝     ╚═╝   ╚═╝   ╚═╝     ╚═╝`

	banner := BannerStyle.Render(art) + "\n" +
		SubtitleStyle.Render(" "+sessionTagline)

	return banner
}

// BannerCompact returns a single-line compact version for smaller screens.
func BannerCompact() string {
	return AccentStyle.Render("[ LMTM ]") +
		DimStyle.Render(" "+sessionTagline)
}

// bannerBorder is the border style used around the banner area.
var bannerBorder = lipgloss.Border{
	Top:         "━",
	Bottom:      "━",
	Left:        "┃",
	Right:       "┃",
	TopLeft:     "┏",
	TopRight:    "┓",
	BottomLeft:  "┗",
	BottomRight: "┛",
}

// BannerFrameStyle wraps the banner in a sleek border.
var BannerFrameStyle = lipgloss.NewStyle().
	BorderStyle(bannerBorder).
	BorderForeground(colorBorder).
	Padding(0, 1).
	Align(lipgloss.Center)
