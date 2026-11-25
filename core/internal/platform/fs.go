package platform

import "os"

func dirExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
