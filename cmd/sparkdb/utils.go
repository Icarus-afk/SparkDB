package main

import "strings"

func quoteIdent(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
}

func quoteIdents(s []string) []string {
	res := make([]string, len(s))
	for i, v := range s {
		res[i] = quoteIdent(v)
	}
	return res
}
