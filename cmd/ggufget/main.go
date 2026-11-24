// Command ggufget downloads GGUF models from mirrors into the LM Studio models
// directory.
package main

import (
	"os"

	"ggufget/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
