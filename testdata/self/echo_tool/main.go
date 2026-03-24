// Echo tool for process adapter integration tests.
// Subcommands:
//
//	echo_tool json '{"key":"value"}'  — prints the JSON to stdout, exit 0
//	echo_tool exit <code>             — exits with the given code
//	echo_tool stderr <message>        — prints message to stderr, exit 1
//	echo_tool greet <name>            — prints {"greeting":"hello <name>"} to stdout
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: echo_tool <subcommand> [args...]")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "json":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo_tool json <json-string>")
			os.Exit(2)
		}
		// Validate it's JSON, then print it
		var v any
		if err := json.Unmarshal([]byte(os.Args[2]), &v); err != nil {
			fmt.Fprintf(os.Stderr, "invalid json: %v\n", err)
			os.Exit(2)
		}
		fmt.Println(os.Args[2])

	case "exit":
		code := 0
		if len(os.Args) >= 3 {
			c, err := strconv.Atoi(os.Args[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid exit code: %v\n", err)
				os.Exit(2)
			}
			code = c
		}
		os.Exit(code)

	case "stderr":
		msg := "error"
		if len(os.Args) >= 3 {
			msg = os.Args[2]
		}
		fmt.Fprint(os.Stderr, msg)
		os.Exit(1)

	case "greet":
		name := "world"
		if len(os.Args) >= 3 {
			name = os.Args[2]
		}
		out := map[string]any{
			"greeting": fmt.Sprintf("hello %s", name),
			"name":     name,
		}
		json.NewEncoder(os.Stdout).Encode(out)

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
}
