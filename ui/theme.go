package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Primary     lipgloss.Color
	Accent      lipgloss.Color
	Border      lipgloss.Color
	FocusBorder lipgloss.Color
	Background  lipgloss.Color
	Foreground  lipgloss.Color
	Muted       lipgloss.Color
	Error       lipgloss.Color
	Success     lipgloss.Color
	Palette     Palette
	// StreamColors color-codes the d-streams (d1 = red, d2 = blue, ...)
	// for the pattern tracing cursor. Streams beyond the list cycle.
	StreamColors []lipgloss.Color
}

const (
	colorPrimary     = "62"
	colorAccent      = "207"
	colorBorder      = "62"
	colorBgDark      = "235"
	colorFgDark      = "252"
	colorMutedDark   = "240"
	colorErrorDark   = "196"
	colorSuccessDark = "46"

	colorBgLight      = "254"
	colorFgLight      = "232"
	colorMutedLight   = "248"
	colorErrorLight   = "124"
	colorSuccessLight = "22"
	colorPrimaryLight = "25"
	colorAccentLight  = "99"
	colorBorderLight  = "25"

	colorActiveDark  = "46"
	colorSoloedDark  = "226"
	colorIdleDark    = "238"
	colorLabelDark   = "245"
	colorGhciOKDark  = "46"
	colorGhciErrDark = "196"
	colorEditedDark  = "214"

	colorActiveLight  = "28"
	colorSoloedLight  = "130"
	colorIdleLight    = "248"
	colorLabelLight   = "243"
	colorGhciOKLight  = "28"
	colorGhciErrLight = "124"
	colorEditedLight  = "166"

	colorStreamMutedLight = "245"
)

var (
	// Stream colors for the pattern tracing cursor, d1 onwards.
	streamsDark  = []lipgloss.Color{"196", "39", "46", "226", "201", "51", "208", "141"}
	streamsLight = []lipgloss.Color{"124", "25", "28", "130", "127", "30", "166", "61"}
)

func DarkTheme() Theme {
	return Theme{
		Primary:      colorPrimary,
		Accent:       colorAccent,
		Border:       colorBorder,
		FocusBorder:  colorAccent,
		Background:   colorBgDark,
		Foreground:   colorFgDark,
		Muted:        colorMutedDark,
		Error:        colorErrorDark,
		Success:      colorSuccessDark,
		Palette:      DarkPalette(),
		StreamColors: streamsDark,
	}
}

func LightTheme() Theme {
	return Theme{
		Primary:      colorPrimaryLight,
		Accent:       colorAccentLight,
		Border:       colorBorderLight,
		FocusBorder:  colorAccentLight,
		Background:   colorBgLight,
		Foreground:   colorFgLight,
		Muted:        colorMutedLight,
		Error:        colorErrorLight,
		Success:      colorSuccessLight,
		Palette:      LightPalette(),
		StreamColors: streamsLight,
	}
}

func ThemeFor(name string) Theme {
	if name == "light" {
		return LightTheme()
	}
	return DarkTheme()
}

// StreamColor returns the palette color for a 1-indexed d-stream,
// cycling through the theme's stream colors for streams beyond it.
func (t Theme) StreamColor(streamNum int) lipgloss.Color {
	if streamNum < 1 || len(t.StreamColors) == 0 {
		return t.Accent
	}
	return t.StreamColors[(streamNum-1)%len(t.StreamColors)]
}

type Palette struct {
	Active  lipgloss.Color
	Muted   lipgloss.Color
	Soloed  lipgloss.Color
	Idle    lipgloss.Color
	Label   lipgloss.Color
	Edited  lipgloss.Color
	GhciOK  lipgloss.Color
	GhciErr lipgloss.Color
}

func DarkPalette() Palette {
	return Palette{
		Active:  colorActiveDark,
		Muted:   colorMutedDark,
		Soloed:  colorSoloedDark,
		Idle:    colorIdleDark,
		Label:   colorLabelDark,
		Edited:  colorEditedDark,
		GhciOK:  colorGhciOKDark,
		GhciErr: colorGhciErrDark,
	}
}

func LightPalette() Palette {
	return Palette{
		Active:  colorActiveLight,
		Muted:   colorStreamMutedLight,
		Soloed:  colorSoloedLight,
		Idle:    colorIdleLight,
		Label:   colorLabelLight,
		Edited:  colorEditedLight,
		GhciOK:  colorGhciOKLight,
		GhciErr: colorGhciErrLight,
	}
}
