package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

// Validate option prefixes without inspecting positional arguments or flag values.
func parseFlags(flags *flag.FlagSet, args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, isOption := strings.CutPrefix(arg, "-")
		if !isOption || arg == "-" || arg == "--" {
			break
		}
		name, long := strings.CutPrefix(name, "-")
		name, _, hasValue := strings.Cut(name, "=")
		option := flags.Lookup(name)
		if len(name) > 1 && !long {
			message := fmt.Sprintf("invalid option %q", arg)
			if option != nil || name == "help" {
				message += fmt.Sprintf(". Use %q instead", "-"+arg)
			}
			fmt.Fprintln(flags.Output(), message)
			return errors.New(message)
		}
		if option == nil {
			break // Let flag handle help and unknown options.
		}
		boolean, ok := option.Value.(interface{ IsBoolFlag() bool })
		if !hasValue && !(ok && boolean.IsBoolFlag()) {
			i++
		}
	}
	return flags.Parse(args)
}
