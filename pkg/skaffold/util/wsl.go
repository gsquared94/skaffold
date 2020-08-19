package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

func DetectWSL() (bool, error) {
	if _, err := os.Stat("/proc/version"); err == nil {
		b, err := ioutil.ReadFile("/proc/version")
		if err != nil {
			return false, fmt.Errorf("read /proc/version: %w", err)
		}

		if bytes.Contains(b, []byte("Microsoft")) {
			return true, nil
		}
	}
	return false, nil
}
