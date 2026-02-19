// Package branding centralizes all Autopus brand-related constants
// for the Local Bridge CLI, including application identity, brand colors,
// and ASCII art assets.
package branding

// Application identity constants.
const (
	AppName    = "Autopus"
	CLIName    = "Autopus Local Bridge"
	BinaryName = "autopus-bridge"
)

// Brand colors in hex format for Lipgloss true color support.
const (
	// ColorPrimary is the main brand color (Primary Purple).
	ColorPrimary = "#8B5CF6"
	// ColorDeepViolet is a darker purple for backgrounds and accents.
	ColorDeepViolet = "#4C1D95"
	// ColorRichPurple is a medium intensity purple.
	ColorRichPurple = "#6D28D9"
	// ColorTeal is the secondary brand color (Ocean Teal).
	ColorTeal = "#14B8A6"
	// ColorDarkTeal is a darker teal for suction cup accents.
	ColorDarkTeal = "#0D9488"
	// ColorCoral is the error/danger color (Deep Coral).
	ColorCoral = "#E11D48"
	// ColorRose is a lighter coral variant.
	ColorRose = "#F43F5E"
	// ColorBgDark is the dark background color.
	ColorBgDark = "#1E1030"
	// ColorWhite is pure white.
	ColorWhite = "#FFFFFF"
	// ColorLightGray is a light gray for labels.
	ColorLightGray = "#A1A1AA"
	// ColorMutedGray is a muted gray for help text.
	ColorMutedGray = "#71717A"
	// ColorBorderGray is a panel border gray for inactive elements.
	ColorBorderGray = "#52525B"
)

// Banner is a compact ASCII art octopus for CLI startup display.
// The design features a dome-shaped head, two eyes, a mouth,
// and tentacles spreading outward below the body.
const Banner = `
     ___
    /o o\
   ( === )
    \   /
  __|_|_|__
 / / | | \ \`

// StartupBanner returns the full branded startup banner
// with the application name appended below the ASCII art.
func StartupBanner() string {
	return Banner + "\n" +
		"  " + CLIName + "\n"
}
