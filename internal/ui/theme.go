package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Primary     color.Color
	Accent      color.Color
	Border      color.Color
	FocusBorder color.Color
	Background  color.Color
	Foreground  color.Color
	Muted       color.Color
	Error       color.Color
	Success     color.Color
	Palette     Palette
	// StreamColors color-codes the d-streams (d1 = red, d2 = blue, ...)
	// for the pattern tracing cursor. Streams beyond the list cycle.
	StreamColors []color.Color
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
	streamsDark  = []color.Color{lipgloss.Color("196"), lipgloss.Color("39"), lipgloss.Color("46"), lipgloss.Color("226"), lipgloss.Color("201"), lipgloss.Color("51"), lipgloss.Color("208"), lipgloss.Color("141")}
	streamsLight = []color.Color{lipgloss.Color("124"), lipgloss.Color("25"), lipgloss.Color("28"), lipgloss.Color("130"), lipgloss.Color("127"), lipgloss.Color("30"), lipgloss.Color("166"), lipgloss.Color("61")}
)

func DarkTheme() Theme {
	return Theme{
		Primary:      lipgloss.Color(colorPrimary),
		Accent:       lipgloss.Color(colorAccent),
		Border:       lipgloss.Color(colorBorder),
		FocusBorder:  lipgloss.Color(colorAccent),
		Background:   lipgloss.Color(colorBgDark),
		Foreground:   lipgloss.Color(colorFgDark),
		Muted:        lipgloss.Color(colorMutedDark),
		Error:        lipgloss.Color(colorErrorDark),
		Success:      lipgloss.Color(colorSuccessDark),
		Palette:      DarkPalette(),
		StreamColors: streamsDark,
	}
}

func LightTheme() Theme {
	return Theme{
		Primary:      lipgloss.Color(colorPrimaryLight),
		Accent:       lipgloss.Color(colorAccentLight),
		Border:       lipgloss.Color(colorBorderLight),
		FocusBorder:  lipgloss.Color(colorAccentLight),
		Background:   lipgloss.Color(colorBgLight),
		Foreground:   lipgloss.Color(colorFgLight),
		Muted:        lipgloss.Color(colorMutedLight),
		Error:        lipgloss.Color(colorErrorLight),
		Success:      lipgloss.Color(colorSuccessLight),
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
func (t Theme) StreamColor(streamNum int) color.Color {
	if streamNum < 1 || len(t.StreamColors) == 0 {
		return t.Accent
	}
	return t.StreamColors[(streamNum-1)%len(t.StreamColors)]
}

type Palette struct {
	Active  color.Color
	Muted   color.Color
	Soloed  color.Color
	Idle    color.Color
	Label   color.Color
	Edited  color.Color
	GhciOK  color.Color
	GhciErr color.Color
}

func DarkPalette() Palette {
	return Palette{
		Active:  lipgloss.Color(colorActiveDark),
		Muted:   lipgloss.Color(colorMutedDark),
		Soloed:  lipgloss.Color(colorSoloedDark),
		Idle:    lipgloss.Color(colorIdleDark),
		Label:   lipgloss.Color(colorLabelDark),
		Edited:  lipgloss.Color(colorEditedDark),
		GhciOK:  lipgloss.Color(colorGhciOKDark),
		GhciErr: lipgloss.Color(colorGhciErrDark),
	}
}

func LightPalette() Palette {
	return Palette{
		Active:  lipgloss.Color(colorActiveLight),
		Muted:   lipgloss.Color(colorStreamMutedLight),
		Soloed:  lipgloss.Color(colorSoloedLight),
		Idle:    lipgloss.Color(colorIdleLight),
		Label:   lipgloss.Color(colorLabelLight),
		Edited:  lipgloss.Color(colorEditedLight),
		GhciOK:  lipgloss.Color(colorGhciOKLight),
		GhciErr: lipgloss.Color(colorGhciErrLight),
	}
}
