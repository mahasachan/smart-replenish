package config

import "os"

const DefaultDatabaseURL = "postgres://smart:smart@localhost:55432/smartreplenish?sslmode=disable"

func Env(name, fallback string) string {
	if s := os.Getenv(name); s != "" {
		return s
	}
	return fallback
}
