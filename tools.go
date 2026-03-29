//go:build tools

//go:generate go get github.com/inovacc/genversioninfo
//go:generate go run ./scripts/genversion/genversion.go

package tools

import (
	_ "github.com/inovacc/genversioninfo"
)
