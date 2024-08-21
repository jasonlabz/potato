package utils

import (
	"fmt"
	"testing"
)

func TestListDir(t *testing.T) {
	files, err := WalkDir("F:\\baidu\\aiib-go\\gen_\\custom", "")
	fmt.Print(files)
	fmt.Print(err)
}

func TestIsExist(t *testing.T) {
	ok := IsExist("./utils.go")
	fmt.Print(ok)
}
