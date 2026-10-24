package main

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

// markdownStyle starts from glamour's dark/light theme and replaces the literal
// "## " heading prefixes with visual hierarchy, since terminals can't change font size.
func markdownStyle(theme string) ansi.StyleConfig {
	cfg := styles.LightStyleConfig
	h1fg, h1bg, h2, h3, h4 := "255", "62", "26", "127", "240"
	if theme == "dark" {
		cfg = styles.DarkStyleConfig
		h1fg, h1bg, h2, h3, h4 = "230", "62", "75", "213", "250"
	}

	cfg.H1 = heading(ansi.StylePrimitive{Prefix: " ", Suffix: " ", Color: &h1fg, BackgroundColor: &h1bg, Bold: ptr(true)})
	cfg.H2 = heading(ansi.StylePrimitive{Prefix: "▌ ", Color: &h2, Bold: ptr(true), Underline: ptr(true)})
	cfg.H3 = heading(ansi.StylePrimitive{Prefix: "▸ ", Color: &h3, Bold: ptr(true)})
	cfg.H4 = heading(ansi.StylePrimitive{Color: &h4, Bold: ptr(true)})
	cfg.H5 = heading(ansi.StylePrimitive{Color: &h4, Bold: ptr(true), Italic: ptr(true)})
	cfg.H6 = heading(ansi.StylePrimitive{Color: &h4, Italic: ptr(true)})
	return cfg
}

func heading(p ansi.StylePrimitive) ansi.StyleBlock { return ansi.StyleBlock{StylePrimitive: p} }

func ptr[T any](v T) *T { return &v }
