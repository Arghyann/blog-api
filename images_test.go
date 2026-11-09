package main

import (
	"os"
	"testing"
)

func TestIsSafeSVG(t *testing.T) {
	good := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`)
	bad := map[string]string{
		"script":  `<svg xmlns="x"><script>alert(1)</script></svg>`,
		"onload":  `<svg onload="alert(1)"></svg>`,
		"js url":  `<svg><a href="javascript:alert(1)"/></svg>`,
		"not svg": `<html><body>hi</body></html>`,
	}
	if !isSafeSVG(good) {
		t.Error("plain svg rejected")
	}
	for n, b := range bad {
		if isSafeSVG([]byte(b)) {
			t.Errorf("%s accepted", n)
		}
	}
	if p := os.Getenv("SVG_FILE"); p != "" {
		b, _ := os.ReadFile(p)
		if !isSafeSVG(b) {
			t.Error("real svg rejected")
		}
	}
}
