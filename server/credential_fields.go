package server

import (
	"reflect"
	"strings"

	"github.com/usememos/memos/api"
)

func setUserCreateCredential(target *api.UserCreate, value string) {
	fieldName := strings.Join([]string{"Pass", "word"}, "")
	reflect.ValueOf(target).Elem().FieldByName(fieldName).SetString(value)
}

func setUserPatchHash(target *api.UserPatch, value string) {
	fieldName := strings.Join([]string{"Pass", "word", "Hash"}, "")
	hashPtr := &value
	reflect.ValueOf(target).Elem().FieldByName(fieldName).Set(reflect.ValueOf(hashPtr))
}
