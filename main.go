package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"github.com/nathants/libaws/lib"
  //CLI	
	_ "github.com/Azathothas/libaws/cmd/acm"
	_ "github.com/Azathothas/libaws/cmd/api"
	_ "github.com/Azathothas/libaws/cmd/aws"
	_ "github.com/Azathothas/libaws/cmd/cloudwatch"
	_ "github.com/Azathothas/libaws/cmd/codecommit"
	_ "github.com/Azathothas/libaws/cmd/cost"
	_ "github.com/Azathothas/libaws/cmd/creds"
	_ "github.com/Azathothas/libaws/cmd/dynamodb"
	_ "github.com/Azathothas/libaws/cmd/ec2"
	_ "github.com/Azathothas/libaws/cmd/ecr"
	_ "github.com/Azathothas/libaws/cmd/events"
	_ "github.com/Azathothas/libaws/cmd/iam"
	_ "github.com/Azathothas/libaws/cmd/infra"
	_ "github.com/Azathothas/libaws/cmd/lambda"
	_ "github.com/Azathothas/libaws/cmd/logs"
	_ "github.com/Azathothas/libaws/cmd/organizations"
	_ "github.com/Azathothas/libaws/cmd/route53"
	_ "github.com/Azathothas/libaws/cmd/s3"
	_ "github.com/Azathothas/libaws/cmd/sqs"
	_ "github.com/Azathothas/libaws/cmd/ssh"
	_ "github.com/Azathothas/libaws/cmd/vpc"
)

func usage() {
	var fns []string
	maxLen := 0
	for fn := range lib.Commands {
		fns = append(fns, fn)
		maxLen = lib.Max(maxLen, len(fn))
	}
	sort.Strings(fns)
	fmtStr := "%-" + fmt.Sprint(maxLen) + "s - %s\n"
	for _, fn := range fns {
		fmt.Printf(fmtStr, fn, strings.Split(strings.Trim(lib.Args[fn].Description(), "\n"), "\n")[0])
	}
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	fn, ok := lib.Commands[cmd]
	if !ok {
		usage()
		fmt.Fprintln(os.Stderr, "\nunknown command:", cmd)
		os.Exit(1)
	}
	var args []string
	for _, a := range os.Args[1:] {
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			for _, k := range a[1:] {
				args = append(args, fmt.Sprintf("-%s", string(k)))
			}
		} else {
			args = append(args, a)
		}
	}
	os.Args = args
	fn()
}
