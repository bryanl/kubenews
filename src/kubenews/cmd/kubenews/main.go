package main

import (
	"fmt"
	"kubenews/commands"
	"os"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

}
