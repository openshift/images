package router

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed testdata/router/ipfailover.yaml
var ipfailoverTemplate []byte

func init() {
	dir := filepath.Join("e2e", "testdata", "router")
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("testdata_embed: could not create directory %s: %v", dir, err))
	}
	dest := filepath.Join(dir, "ipfailover.yaml")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := os.WriteFile(dest, ipfailoverTemplate, 0644); err != nil {
			panic(fmt.Sprintf("testdata_embed: could not write %s: %v", dest, err))
		}
	}
}
