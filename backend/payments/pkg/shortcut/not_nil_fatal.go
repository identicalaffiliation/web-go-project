package shortcut

import (
	"log"
)

func ErrNotNilFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
