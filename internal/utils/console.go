package utils

import (
	"fmt"
	"io"
	"os"
	"runtime"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

var useColor = runtime.GOOS != "windows" || os.Getenv("TERM") != ""

func Blue(v string) string {
	return paint(colorBlue, v)
}

func Green(v string) string {
	return paint(colorGreen, v)
}

func Red(v string) string {
	return paint(colorRed, v)
}

func Yellow(v string) string {
	return paint(colorYellow, v)
}

func paint(color, v string) string {
	if !useColor {
		return v
	}
	return fmt.Sprintf("%s%s%s", color, v, colorReset)
}

func Println(w io.Writer, msg string) {
	_, _ = fmt.Fprintln(w, msg)
}
