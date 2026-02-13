package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ab/design-reviewer/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
		fs := flag.NewFlagSet("login", flag.ExitOnError)
		server := fs.String("server", "", "server URL")
		fs.Parse(os.Args[2:])
		if err := cli.Login(*server); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "logout":
		if err := cli.Logout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "push":
		fs := flag.NewFlagSet("push", flag.ExitOnError)
		name := fs.String("name", "", "project name")
		server := fs.String("server", "", "server URL")
		fs.Parse(os.Args[2:])
		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Usage: design-reviewer push <directory> [--name <project-name>] [--server URL]")
			os.Exit(1)
		}
		if err := cli.Push(fs.Arg(0), *name, *server); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: design-reviewer <command> [options]

Commands:
  login   [--server URL]                          Log in via Google OAuth
  logout                                          Remove stored token
  push    <directory> [--name <name>] [--server URL]  Upload a design project`)
}
