package featureflags

import "os"

func Enabled(name string, defaultVal bool) bool {
	v := os.Getenv("FF_" + name)
	if v == "" {
		return defaultVal
	}
	return v == "1" || v == "true"
}
