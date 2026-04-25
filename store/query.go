package store

import "fmt"

func formatQuery(driver, query string) string {
	if driver != "postgres" {
		return query
	}
	var result []byte
	n := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			result = append(result, []byte(fmt.Sprintf("$%d", n))...)
			n++
		} else {
			result = append(result, query[i])
		}
	}
	return string(result)
}

func userTableName(driver string) string {
	switch driver {
	case "postgres":
		return `"user"`
	case "mysql":
		return "`user`"
	default:
		return "user"
	}
}

func quotedKeyCol(driver string) string {
	if driver == "mysql" {
		return "`key`"
	}
	return "key"
}
