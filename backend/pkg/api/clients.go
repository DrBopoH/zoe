package api

import (
	"os"
	"strings"
)

// EnvList — значения из первой непустой переменной окружения, разбитые по запятой (пустые и пробелы отброшены).
// Нужен для токенов банков: MONO_TOKENS (список) или MONO_TOKEN (один).
func EnvList(names ...string) []string {
	for _, n := range names {
		v := os.Getenv(n)
		if v == "" {
			continue
		}
		var out []string
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// ClientsFromEnv создаёт клиентов банка по токенам из окружения.
func ClientsFromEnv[T any](mk func(token string) T, names ...string) []T {
	var out []T
	for _, tok := range EnvList(names...) {
		out = append(out, mk(tok))
	}
	return out
}

// EnvOr — значение переменной окружения или запасное.
func EnvOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
