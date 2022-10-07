/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"codis/cmd"
	"codis/pkg/log"
)

func main() {
	logger := log.NewLogger()
	logger.Info("Starting Codis...")

	cmd.Execute()
}
