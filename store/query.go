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

func blobCol(driver string) string {
	if driver == "mysql" {
		return "`blob`"
	}
	return "blob"
}

func blobColRef(driver string) string {
	if driver == "mysql" {
		return "resource.`blob`"
	}
	return "resource.blob"
}

func boolLiteral(driver string, value bool) string {
	if driver == "postgres" {
		if value {
			return "TRUE"
		}
		return "FALSE"
	}
	if value {
		return "1"
	}
	return "0"
}

func quotedKeyCol(driver string) string {
	if driver == "mysql" {
		return "`key`"
	}
	return "key"
}
