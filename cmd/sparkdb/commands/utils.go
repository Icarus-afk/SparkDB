package commands

import "fmt"

func quoteIdent(name string) string {
	return fmt.Sprintf(`"%s"`, name)
}

func quoteIdents(names []string) []string {
	res := make([]string, len(names))
	for i, n := range names {
		res[i] = quoteIdent(n)
	}
	return res
}
