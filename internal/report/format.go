package report

import (
	"fmt"
	"math"
)

// FormatBytes renders human-readable size (base-2, like timedog default without -H).
func FormatBytes(n int64) string {
	if n < 0 {
		return "..."
	}
	b := float64(n)
	base := 1024.0
	suffixes := []string{"", "K", "M", "G", "T", "P"}
	suf := 0
	for b >= base && suf < len(suffixes)-1 {
		b /= base
		suf++
	}
	s := suffixes[suf]
	if suf > 0 {
		s += "iB"
	}
	if suf == 0 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1f%s", math.Round(b*10)/10, s)
}

// FormatBytesDecimal is timedog-style -H (base 10, KB/MB/… + B).
func FormatBytesDecimal(n int64) string {
	if n < 0 {
		return "..."
	}
	b := float64(n)
	base := 1000.0
	suffixes := []string{"", "K", "M", "G", "T", "P"}
	suf := 0
	for b >= base && suf < len(suffixes)-1 {
		b /= base
		suf++
	}
	s := suffixes[suf]
	if suf > 0 {
		s += "B"
	}
	if suf == 0 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1f%s", math.Round(b*10)/10, s)
}

// FormatDisplay picks new-side size string for timedog-like flags.
func FormatDisplay(n int64, useBase10, simpleFormat bool) string {
	if simpleFormat {
		return FormatBytesSimple(n, useBase10)
	}
	if useBase10 {
		return FormatBytesDecimal(n)
	}
	return FormatBytes(n)
}

// FormatBytesSimple is closer to timedog -n (Ki/Mi without trailing B on multipliers).
func FormatBytesSimple(n int64, base10 bool) string {
	if n < 0 {
		return "..."
	}
	b := float64(n)
	base := 1024.0
	if base10 {
		base = 1000.0
	}
	suffixes := []string{"", "K", "M", "G", "T", "P"}
	suf := 0
	for b >= base && suf < len(suffixes)-1 {
		b /= base
		suf++
	}
	s := suffixes[suf]
	if suf > 0 && !base10 {
		s += "i"
	}
	if suf == 0 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1f%s", math.Round(b*10)/10, s)
}
